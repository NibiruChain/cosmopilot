package chainnode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/chainutils"
	"github.com/voluzi/cosmopilot/v2/internal/chainutils/sdkcmd"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	"github.com/voluzi/cosmopilot/v2/internal/k8s"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
)

func (r *Reconciler) isChainNodePodRunning(ctx context.Context, chainNode *appsv1.ChainNode) (bool, bool, error) {
	pod, err := r.getChainNodePod(ctx, chainNode)
	if err != nil {
		return false, false, err
	}

	if pod == nil {
		return false, false, nil
	}

	// Check if the pod is terminating or in a failed state
	if isPodTerminating(pod) || nodeUtilsIsInFailedState(pod) || podInFailedState(chainNode, pod) {
		return false, false, nil
	}

	// Check if the pod is running
	isRunning := pod.Status.Phase == corev1.PodRunning

	// Check if the pod is ready
	isReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	return isRunning, isReady, nil
}

func (r *Reconciler) getChainNodePod(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pod, nil
}

func (r *Reconciler) ensurePod(ctx context.Context, _ *chainutils.App, chainNode *appsv1.ChainNode, configHash string) error {
	logger := log.FromContext(ctx)

	// Prepare pod spec
	pod, err := r.getPodSpec(ctx, chainNode, configHash)
	if err != nil {
		return fmt.Errorf("failed to get pod spec for %s: %w", chainNode.GetName(), err)
	}

	// Get current pod. If it does not exist create it and exit.
	currentPod := &corev1.Pod{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), currentPod)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.ensureMissingPod(ctx, chainNode, pod)
		}
		return fmt.Errorf("failed to get pod for %s: %w", chainNode.GetName(), err)
	}

	if isPodTerminating(currentPod) {
		logger.Info("wait for pod to finish terminating")
		if err = r.waitForPodTermination(ctx, currentPod); err != nil {
			return fmt.Errorf("failed waiting for pod %s termination: %w", currentPod.GetName(), err)
		}
		absent, absenceErr := r.podIsAuthoritativelyAbsent(ctx, currentPod)
		if absenceErr != nil {
			return fmt.Errorf("failed to verify pod %s absence: %w", currentPod.GetName(), absenceErr)
		}
		if !absent {
			logger.V(1).Info("pod name is still occupied after observed pod termination", "pod", currentPod.GetName())
			return nil
		}
		return r.ensureMissingPod(ctx, chainNode, pod)
	}

	recoveryAction := terminalPodRecoveryFor(chainNode, currentPod)
	if mustStop, _ := chainNode.MustStop(); mustStop && recoveryAction != terminalPodHold {
		logger.Info("enforcing existing halt-height hold", "halt-height", chainNode.Spec.Config.GetHaltHeight())
		return r.recreatePod(ctx, chainNode, currentPod, pod, false)
	}
	if err = r.reconcileHaltHeightHoldForAction(ctx, chainNode, recoveryAction); err != nil {
		return fmt.Errorf("failed to reconcile halt-height hold for %s: %w", chainNode.GetName(), err)
	}

	var requiresUpgrade bool
	switch recoveryAction {
	case terminalPodHold:
		if err = r.persistTerminalHaltLatestHeight(ctx, chainNode, currentPod); err != nil {
			return fmt.Errorf("failed to persist terminal halt height for %s: %w", chainNode.GetName(), err)
		}
		logger.Info("holding node at configured halt height", "halt-height", chainNode.Spec.Config.GetHaltHeight())
		return r.recreatePod(ctx, chainNode, currentPod, pod, false)
	case terminalPodRestart:
		logger.Info("recreating terminal pod without valid halt or upgrade evidence", "pod", pod.GetName())
		return r.recreatePod(ctx, chainNode, currentPod, pod, false)
	case terminalPodWaitForEvidence:
		logger.V(1).Info("checking running node-utils before waiting for termination evidence", "pod", pod.GetName())
		requiresUpgrade, err = r.requiresUpgradeFresh(ctx, chainNode)
		if err != nil {
			return fmt.Errorf("failed to check if terminal pod %s requires upgrade: %w", chainNode.GetName(), err)
		}
		latestHeight, latestHeightAvailable, heightErr := r.refreshLatestHeight(ctx, chainNode)
		if heightErr != nil {
			return fmt.Errorf("failed to update latest height for terminal pod %s: %w", chainNode.GetName(), heightErr)
		}
		current, currentErr := r.terminalEvidencePodIsCurrent(ctx, currentPod)
		if currentErr != nil {
			return fmt.Errorf("failed to verify terminal pod %s identity: %w", chainNode.GetName(), currentErr)
		}
		if !current {
			return nil
		}
		if !requiresUpgrade {
			recoveryAction = terminalPodRecoveryWithoutNodeUtilsEvidence(
				chainNode, currentPod, latestHeight, latestHeightAvailable,
			)
			if err = r.reconcileHaltHeightHoldForAction(ctx, chainNode, recoveryAction); err != nil {
				return fmt.Errorf("failed to reconcile halt-height hold for %s: %w", chainNode.GetName(), err)
			}
			switch recoveryAction {
			case terminalPodHold:
				logger.Info("holding node at configured halt height", "halt-height", chainNode.Spec.Config.GetHaltHeight())
				return r.recreatePod(ctx, chainNode, currentPod, pod, false)
			case terminalPodRestart:
				logger.Info("recreating terminal pod without valid halt or upgrade evidence", "pod", pod.GetName())
				return r.recreatePod(ctx, chainNode, currentPod, pod, false)
			case terminalPodNotTerminated:
				return nil
			default:
				return fmt.Errorf("unexpected terminal recovery action %d for %s", recoveryAction, currentPod.GetName())
			}
		}
	case terminalPodUpgrade:
		evidence, ok := nodeUtilsTerminationEvidence(currentPod)
		if !ok || evidence.RequiredUpgrade == nil {
			return fmt.Errorf("terminal pod %s lost required upgrade evidence", currentPod.GetName())
		}
		requiresUpgrade = true
		if chainNode.Status.LatestHeight != evidence.RequiredUpgrade.Height {
			chainNode.Status.LatestHeight = evidence.RequiredUpgrade.Height
			if err = r.Status().Update(ctx, chainNode); err != nil {
				return fmt.Errorf("failed to project upgrade height for %s: %w", chainNode.GetName(), err)
			}
		}
	case terminalPodNotTerminated:
		// A terminal action must retain the exact Pod identity used to validate its evidence.
		logger.V(1).Info("checking for labels changes", "current", currentPod.Labels, "new", pod.Labels)
		if !reflect.DeepEqual(currentPod.Labels, pod.Labels) {
			logger.Info("updating pod labels", "pod", pod.GetName())
			modifiedPod := currentPod.DeepCopy()
			modifiedPod.Labels = pod.Labels
			currentPod, err = r.PatchPod(ctx, currentPod, modifiedPod)
			if err != nil {
				return fmt.Errorf("failed to patch pod labels for %s: %w", pod.GetName(), err)
			}
		}

		logger.V(1).Info("updating latest height")
		if err = r.updateLatestHeight(ctx, chainNode); err != nil {
			return fmt.Errorf("failed to update latest height for %s: %w", chainNode.GetName(), err)
		}

		logger.V(1).Info("checking if an upgrade is required")
		requiresUpgrade, err = r.requiresUpgrade(ctx, chainNode)
		if err != nil {
			return fmt.Errorf("failed to check if upgrade is required for %s: %w", chainNode.GetName(), err)
		}
	}

	if requiresUpgrade {
		// Get upgrade from scheduled upgrades list
		upgrade := r.getUpgrade(chainNode, chainNode.Status.LatestHeight)

		logger.V(1).Info("upgrade is required", "upgrade", upgrade)

		// If we don't have upgrade info for this upgrade, or it is incomplete (no image), lets through an error
		if upgrade == nil || upgrade.Status == appsv1.UpgradeImageMissing {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonUpgradeMissingData,
				"Missing upgrade or image for upgrade at height %d",
				chainNode.Status.LatestHeight,
			)
			return fmt.Errorf("missing upgrade or image for height %d", chainNode.Status.LatestHeight)
		}

		logger.Info("upgrading node", "pod", pod.GetName())
		if err := r.setUpgradeStatus(ctx, chainNode, upgrade, appsv1.UpgradeOnGoing); err != nil {
			return fmt.Errorf("failed to set upgrade status for %s: %w", chainNode.GetName(), err)
		}

		if recoveryAction == terminalPodNotTerminated {
			// Only a live Pod needs the transitional label. Terminal recovery must retain
			// the evidence-bound UID until the preconditioned delete is accepted.
			modifiedPod := currentPod.DeepCopy()
			if modifiedPod.Labels == nil {
				modifiedPod.Labels = make(map[string]string)
			}
			modifiedPod.Labels[controllers.LabelUpgrading] = controllers.StringValueTrue
			if _, err = r.PatchPod(ctx, currentPod, modifiedPod); err != nil {
				return fmt.Errorf("failed to patch pod upgrading label for %s: %w", pod.GetName(), err)
			}
		}

		// Force update config files, to prevent restarting again because of config changes
		app, err := chainutils.NewApp(r.ClientSet, r.Scheme, r.RestConfig, chainNode,
			chainNode.Spec.App.GetSdkVersion(),
			[]sdkcmd.Option{sdkcmd.WithGenesisSubcommand(chainNode.Spec.App.UseGenesisSubcommand())},
			chainutils.WithImage(chainNode.GetAppImage()),
			chainutils.WithImagePullPolicy(chainNode.Spec.App.ImagePullPolicy),
			chainutils.WithBinary(chainNode.Spec.App.App),
			chainutils.WithPriorityClass(r.opts.GetDefaultPriorityClassName()),
			chainutils.WithAffinityConfig(chainNode.Spec.Affinity),
			chainutils.WithNodeSelector(chainNode.Spec.NodeSelector),
		)
		if err != nil {
			return fmt.Errorf("failed to create new app for upgrade %s: %w", chainNode.GetName(), err)
		}
		configHash, err = r.ensureConfigs(ctx, app, chainNode, true)
		if err != nil {
			return fmt.Errorf("failed to ensure configs for upgrade %s: %w", chainNode.GetName(), err)
		}

		// Get new pod spec with updated configs
		pod, err = r.getPodSpec(ctx, chainNode, configHash)
		if err != nil {
			return fmt.Errorf("failed to get pod spec after config update for %s: %w", chainNode.GetName(), err)
		}

		if targetCommitted, err := r.upgradePod(ctx, chainNode, currentPod, pod, upgrade.Image); err != nil {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonUpgradeFailed,
				"Failed to restart for upgrade: %v",
				err,
			)
			var upgradeStatus appsv1.UpgradePhase
			if targetCommitted {
				// Once deletion of the old Pod is accepted, preserve the target version even
				// if deletion, creation, or startup later fails. Retrying the old image after
				// an on-disk migration could corrupt the node.
				return r.completeUpgrade(ctx, chainNode, upgrade)
			} else {
				upgradeStatus = appsv1.UpgradeScheduled
			}
			return r.setUpgradeStatus(ctx, chainNode, upgrade, upgradeStatus)
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonUpgradeCompleted,
			"Upgraded node to %s on height %d",
			upgrade.Image, upgrade.Height,
		)
		return r.completeUpgrade(ctx, chainNode, upgrade)
	}

	// Recreate pod if it is in failed state
	if podInFailedState(chainNode, currentPod) {
		logger.Info("pod is in failed state", "pod", pod.GetName())
		ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, currentPod)
		logs, err := ph.GetLogs(ctx, chainNode.Spec.App.App)
		if err != nil {
			logger.Info("could not retrieve logs: " + err.Error())
		} else {
			logLines := strings.Split(logs, "\n")
			if len(logLines) > defaultLogsLineCount {
				logger.Info("app error: " + strings.Join(logLines[len(logLines)-defaultLogsLineCount:], "\n"))
			} else {
				logger.Info("app error: " + strings.Join(logLines, "\n"))
			}
		}
		return r.recreatePod(ctx, chainNode, currentPod, pod, false)
	}

	// Re-create pod if spec changes
	if podSpecChanged(ctx, currentPod, pod) {
		logger.Info("pod spec changed", "pod", pod.GetName())
		return r.recreatePod(ctx, chainNode, currentPod, pod, r.opts.DisruptionCheckEnabled)
	}

	// Re-create pod if config changed
	if currentPod.Annotations[controllers.AnnotationConfigHash] != configHash {
		logger.Info("config changed", "pod", pod.GetName())
		return r.recreatePod(ctx, chainNode, currentPod, pod, r.opts.DisruptionCheckEnabled)
	}

	return r.setNodePhase(ctx, chainNode)
}

func (r *Reconciler) createPod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	if mustStop, reason := chainNode.MustStop(); mustStop {
		logger.Info("node must be stopped. not creating pod", "pod", pod.GetName(), "reason", reason)
		return r.setNodePhase(ctx, chainNode)
	}

	logger.Info("creating pod", "pod", pod.GetName())
	startPhase := appsv1.PhaseChainNodeStarting
	if chainNode.Status.LatestHeight > 0 {
		startPhase = appsv1.PhaseChainNodeRestarting
	}
	if err := r.updatePhase(ctx, chainNode, startPhase); err != nil {
		return fmt.Errorf("failed to update phase for %s: %w", chainNode.GetName(), err)
	}

	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
	if err := ph.Create(ctx); err != nil {
		return fmt.Errorf("failed to create pod %s: %w", pod.GetName(), err)
	}
	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
		return fmt.Errorf("timeout waiting for container %s to start in pod %s: %w", chainNode.Spec.App.App, pod.GetName(), err)
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeStarted,
		"Node successfully started",
	)
	return r.setNodePhase(ctx, chainNode)
}

func (r *Reconciler) ensureMissingPod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	if shouldMigrateLegacyHaltHeightHold(chainNode) {
		if err := r.reconcileHaltHeightHoldForAction(ctx, chainNode, terminalPodHold); err != nil {
			return fmt.Errorf("failed to migrate legacy halt-height hold for %s: %w", chainNode.GetName(), err)
		}
	}
	if err := r.reconcileHaltHeightHoldForAction(ctx, chainNode, terminalPodNotTerminated); err != nil {
		return fmt.Errorf("failed to reconcile halt-height hold for %s: %w", chainNode.GetName(), err)
	}
	return r.createPod(ctx, chainNode, pod)
}

func (r *Reconciler) podIsAuthoritativelyAbsent(ctx context.Context, observedPod *corev1.Pod) (bool, error) {
	_, err := r.ClientSet.CoreV1().Pods(observedPod.Namespace).Get(ctx, observedPod.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

func (r *Reconciler) terminalEvidencePodIsCurrent(ctx context.Context, observedPod *corev1.Pod) (bool, error) {
	if observedPod.UID == "" {
		return false, nil
	}
	currentPod, err := r.ClientSet.CoreV1().Pods(observedPod.Namespace).Get(ctx, observedPod.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return currentPod.UID != "" && currentPod.UID == observedPod.UID && !isPodTerminating(currentPod), nil
}

// getConfigFilesMounts loads the ConfigMap and returns individual volume mounts for each config file.
// We mount config files individually to allow the config directory to be writable.
func (r *Reconciler) getConfigFilesMounts(ctx context.Context, chainNode *appsv1.ChainNode) ([]corev1.VolumeMount, error) {
	config := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), config)
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap for %s: %w", chainNode.GetName(), err)
	}

	mounts := make([]corev1.VolumeMount, 0, len(config.Data))
	for k := range config.Data {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: "/home/app/config/" + k,
			SubPath:   k,
		})
	}
	return mounts, nil
}

// buildBaseVolumes creates the base set of volumes that are always present in the pod.
func (r *Reconciler) buildBaseVolumes(chainNode *appsv1.ChainNode) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "app-empty-dir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: chainNode.GetName(),
				},
			},
		},
		{
			Name: "config-empty-dir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: chainNode.GetName(),
					},
				},
			},
		},
		{
			Name: "node-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: chainNode.GetName(),
				},
			},
		},
		{
			Name: "upgrades-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-upgrades", chainNode.GetName()),
					},
				},
			},
		},
	}
}

// buildNodeUtilsInitContainer creates the node-utils sidecar init container.
func (r *Reconciler) buildNodeUtilsInitContainer(chainNode *appsv1.ChainNode, runAsUser int64, runAsGroup *int64) corev1.Container {
	var sidecarRestartAlways = corev1.ContainerRestartPolicyAlways
	securityContext := k8s.RestrictedSecurityContext()
	securityContext.RunAsUser = ptr.To(runAsUser)
	securityContext.RunAsGroup = runAsGroup
	securityContext.RunAsNonRoot = ptr.To(runAsUser != 0)

	return corev1.Container{
		Name:            nodeUtilsContainerName,
		Image:           r.opts.NodeUtilsImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		RestartPolicy:   &sidecarRestartAlways,
		SecurityContext: securityContext,
		Ports: []corev1.ContainerPort{
			{
				Name:          nodeUtilsPortName,
				ContainerPort: 8000,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/home/app/data",
				ReadOnly:  true,
			},
			{
				Name:      "upgrades-config",
				MountPath: "/config",
			},
		},
		Env: append([]corev1.EnvVar{
			{
				Name:  "BLOCK_THRESHOLD",
				Value: chainNode.Spec.Config.GetBlockThreshold(),
			},
			{
				Name:  "LOG_LEVEL",
				Value: chainNode.Spec.Config.GetNodeUtilsLogLevel(),
			},
			{
				Name:  "TMKMS_PROXY",
				Value: strconv.FormatBool(chainNode.IsValidator() && chainNode.UsesTmKms()),
			},
			{
				Name:  "NODE_BINARY_NAME",
				Value: chainNode.Spec.App.App,
			},
			{
				Name:  "HALT_HEIGHT",
				Value: strconv.FormatInt(chainNode.Spec.Config.GetHaltHeight(), 10),
			},
		}, chainNode.Spec.Config.GetNodeUtilsEnv()...),
		Resources: chainNode.Spec.Config.GetNodeUtilsResources(),
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/must_upgrade",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: nodeUtilsPort,
					},
					Scheme: "HTTP",
				},
			},
			FailureThreshold: 1,
			PeriodSeconds:    2,
		},
	}
}

func effectiveRunIdentity(app *corev1.SecurityContext, pod *corev1.PodSecurityContext) (int64, *int64, error) {
	var runAsUser *int64
	if app != nil && app.RunAsUser != nil {
		runAsUser = app.RunAsUser
	} else if pod != nil && pod.RunAsUser != nil {
		runAsUser = pod.RunAsUser
	}
	if runAsUser == nil || *runAsUser < 0 {
		return 0, nil, fmt.Errorf("node-utils requires a resolved numeric runAsUser matching the app container")
	}

	var runAsGroup *int64
	if app != nil && app.RunAsGroup != nil {
		runAsGroup = app.RunAsGroup
	} else if pod != nil && pod.RunAsGroup != nil {
		runAsGroup = pod.RunAsGroup
	}
	if runAsGroup != nil && *runAsGroup < 0 {
		return 0, nil, fmt.Errorf("node-utils requires a non-negative runAsGroup matching the app container")
	}
	return *runAsUser, runAsGroup, nil
}

// buildAppContainer creates the main application container with its configuration.
func (r *Reconciler) buildAppContainer(chainNode *appsv1.ChainNode, configFilesMounts []corev1.VolumeMount, readinessPath string, appResources corev1.ResourceRequirements, securityContext *corev1.SecurityContext) corev1.Container {
	return corev1.Container{
		Name:            chainNode.Spec.App.App,
		Image:           chainNode.GetAppImage(),
		ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
		SecurityContext: securityContext,
		Command:         []string{chainNode.Spec.App.App},
		Args:            append([]string{"start", "--home", "/home/app"}, chainNode.GetAdditionalRunFlags()...),
		Env:             chainNode.Spec.Config.GetEnv(),
		Ports: []corev1.ContainerPort{
			{
				Name:          chainutils.P2pPortName,
				ContainerPort: chainutils.P2pPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          chainutils.RpcPortName,
				ContainerPort: chainutils.RpcPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          chainutils.LcdPortName,
				ContainerPort: chainutils.LcdPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          chainutils.GrpcPortName,
				ContainerPort: chainutils.GrpcPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          chainutils.PrivValPortName,
				ContainerPort: chainutils.PrivValPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          chainutils.PrometheusPortName,
				ContainerPort: chainutils.PrometheusPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: append([]corev1.VolumeMount{
			{
				Name:      "app-empty-dir",
				MountPath: "/home/app",
			},
			{
				Name:      "data",
				MountPath: "/home/app/data",
			},
			{
				Name:      "config-empty-dir",
				MountPath: "/home/app/config",
			},
			{
				Name:      "node-key",
				MountPath: "/home/app/config/" + nodeKeyFilename,
				SubPath:   nodeKeyFilename,
			},
		}, configFilesMounts...),
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: nodeUtilsPort,
					},
					Scheme: "HTTP",
				},
			},
			PeriodSeconds:    startupProbePeriodSeconds,
			FailureThreshold: int32(chainNode.Spec.Config.GetStartupTime().Seconds() / startupProbePeriodSeconds),
			TimeoutSeconds:   startupProbeTimeoutSeconds,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: nodeUtilsPort,
					},
					Scheme: "HTTP",
				},
			},
			FailureThreshold: livenessProbeFailureThreshold,
			PeriodSeconds:    livenessProbePeriodSeconds,
			TimeoutSeconds:   livenessProbeTimeoutSeconds,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: readinessPath,
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: nodeUtilsPort,
					},
					Scheme: "HTTP",
				},
			},
			FailureThreshold: readinessProbeFailureThreshold,
			PeriodSeconds:    readinessProbePeriodSeconds,
			TimeoutSeconds:   readinessProbeTimeoutSeconds,
		},
		Resources: appResources,
	}
}

func (r *Reconciler) getPodSpec(ctx context.Context, chainNode *appsv1.ChainNode, configHash string) (*corev1.Pod, error) {
	logger := log.FromContext(ctx)

	configFilesMounts, err := r.getConfigFilesMounts(ctx, chainNode)
	if err != nil {
		return nil, err
	}

	// Load configmap for sidecar configuration later in the function
	config := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), config); err != nil {
		return nil, fmt.Errorf("failed to get configmap for %s: %w", chainNode.GetName(), err)
	}

	readinessPath := "/ready"
	if chainNode.Spec.Config.ShouldIgnoreSyncing() {
		readinessPath = "/health"
	}

	var sidecarRestartAlways = corev1.ContainerRestartPolicyAlways

	// Get resources from VPA. NOTE: We trust that this method always falls back to ChainNode specified resources
	// if needed. We don't want to fall back ourselves here because if VPA fails it might still fallback to its previous
	// values and avoid unnecessary restarts.
	appResources, err := r.maybeGetVpaResources(ctx, chainNode)
	if err != nil {
		logger.Info("could not get resources from vpa: " + err.Error())
	}

	// Build initial annotations with user-provided annotations
	annotations := map[string]string{}
	if chainNode.Spec.Config != nil {
		for k, v := range chainNode.Spec.Config.PodAnnotations {
			annotations[k] = v
		}
	}
	// System annotations override user annotations
	annotations[controllers.AnnotationConfigHash] = configHash

	// Use custom pod security context if provided, otherwise use restricted
	podSecurityContext := chainNode.Spec.Config.GetPodSecurityContext()
	if podSecurityContext == nil {
		podSecurityContext = k8s.RestrictedPodSecurityContext()
	}

	// Use custom container security context if provided, otherwise use restricted
	appSecurityContext := chainNode.Spec.Config.GetSecurityContext()
	if appSecurityContext == nil {
		appSecurityContext = k8s.RestrictedSecurityContext()
	}
	runAsUser, runAsGroup, err := effectiveRunIdentity(appSecurityContext, podSecurityContext)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        chainNode.GetName(),
			Namespace:   chainNode.GetNamespace(),
			Annotations: annotations,
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelNodeID:    chainNode.Status.NodeID,
				controllers.LabelChainID:   chainNode.Status.ChainID,
				controllers.LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
				controllers.LabelUpgrading: controllers.StringValueFalse,
			}),
		},
		Spec: corev1.PodSpec{
			ShareProcessNamespace:         ptr.To(true),
			RestartPolicy:                 corev1.RestartPolicyNever,
			ServiceAccountName:            chainNode.Spec.Config.GetServiceAccountName(),
			PriorityClassName:             r.opts.GetNodesPriorityClassName(),
			Affinity:                      chainNode.Spec.Affinity,
			NodeSelector:                  chainNode.Spec.NodeSelector,
			SecurityContext:               podSecurityContext,
			TerminationGracePeriodSeconds: chainNode.Spec.Config.GetTerminationGracePeriodSeconds(),
			Volumes:                       r.buildBaseVolumes(chainNode),
			InitContainers:                []corev1.Container{r.buildNodeUtilsInitContainer(chainNode, runAsUser, runAsGroup)},
			Containers:                    []corev1.Container{r.buildAppContainer(chainNode, configFilesMounts, readinessPath, appResources, appSecurityContext)},
		},
	}

	for _, volume := range chainNode.GetPersistenceAdditionalVolumes() {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volume.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-%s", chainNode.GetName(), volume.Name),
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.Path,
		})
	}

	if chainNode.Spec.Config.IsEvmEnabled() {
		pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, corev1.ContainerPort{
			Name:          controllers.EvmRpcPortName,
			ContainerPort: controllers.EvmRpcPort,
			Protocol:      corev1.ProtocolTCP,
		})
		pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, corev1.ContainerPort{
			Name:          controllers.EvmRpcWsPortName,
			ContainerPort: controllers.EvmRpcWsPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Always use latest version we know if we are doing state-sync restore
	if chainNode.StateSyncRestoreEnabled() && chainNode.Status.LatestHeight == 0 {
		pod.Spec.Containers[0].Image = chainNode.GetLatestAppImage()
	}

	if !chainNode.Spec.Genesis.ShouldUseDataVolume() {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "genesis",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: chainNode.Spec.Genesis.GetConfigMapName(chainNode.Status.ChainID),
					},
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "genesis",
			MountPath: "/home/app/config/" + chainutils.GenesisFilename,
			SubPath:   chainutils.GenesisFilename,
		})
	} else {
		//TODO: This is a workaround. Remove this when issue with genesis_file field not being used is fixed
		pod.Spec.InitContainers = append([]corev1.Container{
			{
				Name:            "link-genesis",
				Image:           "busybox",
				Command:         []string{"/bin/sh"},
				SecurityContext: k8s.RestrictedSecurityContext(),
				Args: []string{
					"-c",
					"ln -s /home/app/data/genesis.json /home/app/config/genesis.json",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "config-empty-dir",
						MountPath: "/home/app/config",
					},
					{
						Name:      "data",
						MountPath: "/home/app/data",
					},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    initContainerCpuResources,
						corev1.ResourceMemory: initContainerMemoryResources,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    initContainerCpuResources,
						corev1.ResourceMemory: initContainerMemoryResources,
					},
				},
			},
		}, pod.Spec.InitContainers...)
	}

	if chainNode.Spec.Config != nil {
		pod.Spec.ImagePullSecrets = chainNode.Spec.Config.ImagePullSecrets
	}

	if chainNode.IsValidator() {
		pod.Spec.PriorityClassName = r.opts.GetValidatorsPriorityClassName()
		if chainNode.UsesTmKms() {
			_, kms, err := r.getTmkms(chainNode)
			if err != nil {
				return nil, err
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, kms.GetVolumes()...)
			pod.Spec.Containers = append(pod.Spec.Containers, kms.GetContainersSpec()...)

		} else {
			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "priv-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: chainNode.Spec.Validator.GetPrivKeySecretName(chainNode),
					},
				},
			})
			pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      "priv-key",
				MountPath: "/home/app/config/" + PrivKeyFilename,
				SubPath:   PrivKeyFilename,
			})
		}
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Sidecars != nil {
		for _, c := range chainNode.Spec.Config.Sidecars {
			if r.shouldSkipSidecar(ctx, chainNode, c, pod.Labels) {
				continue
			}
			// Use custom security context if provided, otherwise use restricted
			sidecarSecurityContext := c.SecurityContext
			if sidecarSecurityContext == nil {
				sidecarSecurityContext = k8s.RestrictedSecurityContext()
			}

			sidecar := corev1.Container{
				Name:            c.Name,
				Image:           c.GetImage(chainNode),
				ImagePullPolicy: chainNode.Spec.Config.GetSidecarImagePullPolicy(c.Name),
				Command:         c.Command,
				Args:            c.Args,
				Env:             c.Env,
				SecurityContext: sidecarSecurityContext,
				Resources:       c.Resources,
			}

			if !c.ShouldRunBeforeNode() {
				sidecar.RestartPolicy = &sidecarRestartAlways
			}

			if c.MountDataVolume != nil {
				sidecar.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "data",
						MountPath: *c.MountDataVolume,
					},
				}
			}

			if c.MountConfig != nil {
				configMounts := []corev1.VolumeMount{
					{
						Name:      "config-empty-dir",
						MountPath: *c.MountConfig,
					},
					{
						Name:      "node-key",
						MountPath: path.Join(*c.MountConfig, nodeKeyFilename),
						SubPath:   nodeKeyFilename,
					},
				}
				for k := range config.Data {
					configMounts = append(configMounts, corev1.VolumeMount{
						Name:      "config",
						MountPath: path.Join(*c.MountConfig, k),
						SubPath:   k,
					})
				}
				sidecar.VolumeMounts = append(sidecar.VolumeMounts, configMounts...)
			}

			pod.Spec.InitContainers = append([]corev1.Container{sidecar}, pod.Spec.InitContainers...)
		}
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.SafeToEvict != nil {
		pod.Annotations[controllers.AnnotationSafeEvict] = strconv.FormatBool(*chainNode.Spec.Config.SafeToEvict)
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.CosmoGuardEnabled() {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: cosmoGuardVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: chainNode.Spec.Config.GetCosmoGuardConfig().LocalObjectReference,
				},
			},
		})
		cosmoGuardContainer := corev1.Container{
			Name:            cosmoGuardContainerName,
			Image:           r.opts.CosmoGuardImage,
			ImagePullPolicy: corev1.PullAlways,
			RestartPolicy:   &sidecarRestartAlways,
			SecurityContext: k8s.RestrictedSecurityContext(),
			Args:            []string{"-config", filepath.Join("/config/", chainNode.Spec.Config.GetCosmoGuardConfig().Key)},
			Ports: []corev1.ContainerPort{
				{
					Name:          controllers.CosmoGuardRpcPortName,
					ContainerPort: controllers.CosmoGuardRpcPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          controllers.CosmoGuardLcdPortName,
					ContainerPort: controllers.CosmoGuardLcdPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          controllers.CosmoGuardGrpcPortName,
					ContainerPort: controllers.CosmoGuardGrpcPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          controllers.CosmoGuardMetricsPortName,
					ContainerPort: controllers.CosmoGuardMetricsPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      cosmoGuardVolumeName,
					MountPath: "/config",
				},
			},
			Resources: chainNode.Spec.Config.GetCosmoGuardResources(),
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/metrics",
						Port: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: controllers.CosmoGuardMetricsPort,
						},
						Scheme: "HTTP",
					},
				},
				FailureThreshold: 1,
				PeriodSeconds:    2,
			},
		}
		if chainNode.Spec.Config.IsEvmEnabled() {
			cosmoGuardContainer.Ports = append(cosmoGuardContainer.Ports, corev1.ContainerPort{
				Name:          controllers.CosmoGuardEvmRpcPortName,
				ContainerPort: controllers.CosmoGuardEvmRpcPort,
				Protocol:      corev1.ProtocolTCP,
			})
			cosmoGuardContainer.Ports = append(cosmoGuardContainer.Ports, corev1.ContainerPort{
				Name:          controllers.CosmoGuardEvmRpcWsPortName,
				ContainerPort: controllers.CosmoGuardEvmRpcWsPort,
				Protocol:      corev1.ProtocolTCP,
			})
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, cosmoGuardContainer)
	}

	specHash, err := podSpecHash(pod)
	if err != nil {
		return nil, err
	}
	pod.Annotations[controllers.AnnotationPodSpecHash] = specHash
	return pod, controllerutil.SetControllerReference(chainNode, pod, r.Scheme)
}

func (r *Reconciler) recreatePod(ctx context.Context, chainNode *appsv1.ChainNode, currentPod, desiredPod *corev1.Pod, preventDisruption bool) error {
	logger := log.FromContext(ctx)

	if preventDisruption {
		// In case there are other validators for the same chain in this namespace, we will ignore nodeset and groups.
		// This way we make sure validators are taken down one by one even if they belong to different nodesets.
		var disruptionLabels map[string]string
		if chainNode.IsValidator() {
			disruptionLabels = map[string]string{
				controllers.LabelChainID:   chainNode.Status.ChainID,
				controllers.LabelValidator: strconv.FormatBool(true),
			}
		} else {
			disruptionLabels = WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelChainID: chainNode.Status.ChainID,
			})
			if chainNode.ShouldIgnoreGroupOnDisruption() {
				delete(disruptionLabels, controllers.LabelChainNodeSetGroup)
			}
		}

		logger.Info("attempting to acquire lock for recreating pod", "pod", desiredPod.GetName(), "labels", disruptionLabels)
		lock := r.disruptionLocks.getLockForLabels(disruptionLabels)
		lock.Lock()
		defer lock.Unlock()
		logger.Info("acquired lock for recreating pod", "pod", desiredPod.GetName(), "labels", disruptionLabels)

		// If the pod is already unavailable, recreating it won't increase disruption
		if isPodRunningAndReady(currentPod) {
			logger.Info("checking pod disruption", "pod", desiredPod.GetName(), "labels", disruptionLabels)
			err := r.checkDisruptionAllowance(ctx, disruptionLabels)
			if err != nil {
				logger.Info("delaying pod recreation due to disruption limits", "pod", desiredPod.GetName(), "reason", err.Error())
				return nil
			}
		} else {
			logger.Info("pod is already unavailable, skipping disruption check", "pod", desiredPod.GetName())
		}
	}

	logger.Info("recreating pod", "pod", desiredPod.GetName())
	if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeRestarting); err != nil {
		return fmt.Errorf("failed to update phase to Restarting for %s: %w", chainNode.GetName(), err)
	}

	logger.V(1).Info("deleting pod", "pod", currentPod.GetName(), "uid", currentPod.GetUID())
	deletePod := currentPod.DeepCopy()
	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, deletePod)
	if err := ph.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete pod %s for recreation: %w", currentPod.GetName(), err)
	}

	// There is no need to wait for pod to be deleted if we are keeping it stopped
	if mustStop, stopReason := chainNode.MustStop(); mustStop {
		logger.Info("node must be stopped. not recreating pod", "pod", desiredPod.GetName(), "reason", stopReason)

		// Attempt to terminate node-utils container without waiting for grace-period. If there is an error
		// we will just wait for the grace-period
		if err := r.stopNodeUtilsContainer(ctx, chainNode); err != nil {
			logger.Info("failed to stop node utils container", "pod", desiredPod.GetName(), "error", err.Error())
		}
		return r.setNodePhase(ctx, chainNode)
	}

	if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
		return fmt.Errorf("timeout waiting for pod %s to be deleted: %w", currentPod.GetName(), err)
	}
	logger.V(1).Info("pod deleted", "pod", currentPod.GetName(), "uid", currentPod.GetUID())

	ph = k8s.NewPodHelper(r.ClientSet, r.RestConfig, desiredPod)
	if err := ph.Create(ctx); err != nil {
		return fmt.Errorf("failed to recreate pod %s: %w", desiredPod.GetName(), err)
	}

	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonNodeError,
			controllers.FormatErrorEvent("Pod failed to start", err),
		)
		_ = r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeError)
		return fmt.Errorf("timeout waiting for recreated container %s to start in pod %s: %w", chainNode.Spec.App.App, desiredPod.GetName(), err)
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeRestarted,
		"Node restarted",
	)
	return r.setNodePhase(ctx, chainNode)
}

func (r *Reconciler) upgradePod(ctx context.Context, chainNode *appsv1.ChainNode, currentPod, desiredPod *corev1.Pod, image string) (bool, error) {
	logger := log.FromContext(ctx)

	logger.Info("upgrading pod", "pod", desiredPod.GetName())
	phase := appsv1.PhaseChainNodeUpgrading
	if err := r.updatePhase(ctx, chainNode, phase); err != nil {
		return false, err
	}

	deletePod := currentPod.DeepCopy()
	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, deletePod)
	desiredPod.Spec.Containers[0].Image = image
	if err := ph.DeleteWithGracePeriod(ctx, upgradeDeletionGracePeriodSeconds(currentPod)); err != nil {
		return false, err
	}
	if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
		return true, err
	}

	ph = k8s.NewPodHelper(r.ClientSet, r.RestConfig, desiredPod)
	if err := ph.Create(ctx); err != nil {
		return true, err
	}

	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonNodeError,
			controllers.FormatErrorEvent("Pod failed to start after upgrade", err),
		)
		_ = r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeError)
		return true, err
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeRestarted,
		"Node upgraded to %s", image,
	)
	return true, r.setNodePhase(ctx, chainNode)
}

func upgradeDeletionGracePeriodSeconds(pod *corev1.Pod) int64 {
	bounded := int64((timeoutPodDeleted / 2).Seconds())
	if configured := pod.Spec.TerminationGracePeriodSeconds; configured != nil && *configured < bounded {
		if *configured < 0 {
			return 0
		}
		return *configured
	}
	return bounded
}

func (r *Reconciler) waitForPodTermination(ctx context.Context, pod *corev1.Pod) error {
	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
	return ph.WaitForPodDeleted(ctx, timeoutPodDeleted)
}

func (r *Reconciler) PatchPod(ctx context.Context, cur, mod *corev1.Pod) (*corev1.Pod, error) {
	logger := log.FromContext(ctx)

	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, err
	}

	pa, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, corev1.Pod{})
	if err != nil {
		return nil, err
	}

	logger.V(1).Info("created patch for pod", "patch", string(pa), "pod", cur.GetName())

	if len(pa) == 0 || string(pa) == "{}" {
		return cur, nil
	}

	return r.ClientSet.CoreV1().Pods(cur.GetNamespace()).
		Patch(ctx, cur.GetName(), types.StrategicMergePatchType, pa, metav1.PatchOptions{})
}

func podSpecHash(pod *corev1.Pod) (string, error) {
	specCopy := pod.Spec.DeepCopy()

	// Order volume mounts and volumes
	orderVolumes(specCopy)

	specBytes, err := specCopy.Marshal()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(specBytes)), nil
}

func podSpecChanged(ctx context.Context, existing, new *corev1.Pod) bool {
	logger := log.FromContext(ctx)

	oldSpecHash, ok := existing.Annotations[controllers.AnnotationPodSpecHash]
	if !ok {
		// Annotation should be there, so lets assume the spec changed
		return true
	}
	newSpecHash := new.Annotations[controllers.AnnotationPodSpecHash]

	logger.V(1).Info("checked pod spec hash",
		"old-spec", oldSpecHash,
		"new-spec", newSpecHash,
	)
	return newSpecHash != oldSpecHash
}

func orderVolumes(podSpec *corev1.PodSpec) {
	for _, c := range podSpec.Containers {
		sort.Slice(c.VolumeMounts, func(i, j int) bool {
			return c.VolumeMounts[i].MountPath < c.VolumeMounts[j].MountPath
		})
	}
	for _, c := range podSpec.InitContainers {
		sort.Slice(c.VolumeMounts, func(i, j int) bool {
			return c.VolumeMounts[i].MountPath < c.VolumeMounts[j].MountPath
		})
	}
	sort.Slice(podSpec.Volumes, func(i, j int) bool {
		return podSpec.Volumes[i].Name < podSpec.Volumes[j].Name
	})
}

func (r *Reconciler) setNodePhase(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	if mustStop, _ := chainNode.MustStop(); mustStop {
		if chainNode.Status.Phase != appsv1.PhaseChainNodeStopped {
			return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeStopped)
		}
		return nil
	}

	if volumeSnapshotInProgress(chainNode) {
		if chainNode.Status.Phase != appsv1.PhaseChainNodeSnapshotting {
			return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeSnapshotting)
		}
	}

	c, err := r.getChainNodeClient(chainNode)
	if err != nil {
		return fmt.Errorf("failed to get chain node client for %s: %w", chainNode.GetName(), err)
	}

	// Check if its state-sync first
	stateSyncing, err := nodeutils.NewClient(chainNode.GetNodeFQDN()).IsStateSyncing(ctx)
	if err != nil {
		return fmt.Errorf("failed to check state-sync status for %s: %w", chainNode.GetName(), err)
	}

	logger.V(1).Info("node state-syncing status", "state-syncing", stateSyncing)
	if stateSyncing {
		if chainNode.Status.Phase != appsv1.PhaseChainNodeStateSyncing {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonNodeStateSyncing,
				"Node is state-syncing",
			)
			chainNode.Status.AppVersion = chainNode.GetAppVersion()
			return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeStateSyncing)
		}
		return nil
	}

	logger.V(1).Info("check if node is syncing")
	syncing, err := c.IsNodeSyncing(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if node %s is syncing: %w", chainNode.GetName(), err)
	}
	logger.V(1).Info("node syncing status", "syncing", syncing)

	if syncing {
		if chainNode.Status.Phase != appsv1.PhaseChainNodeSyncing {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonNodeSyncing,
				"Node is syncing",
			)
			chainNode.Status.AppVersion = chainNode.GetAppVersion()
			return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeSyncing)
		}
		return nil
	}

	if chainNode.Status.Phase != appsv1.PhaseChainNodeRunning {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonNodeRunning,
			"Node is synced and running",
		)
		chainNode.Status.AppVersion = chainNode.GetAppVersion()
		return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeRunning)
	}

	return nil
}

func podInFailedState(chainNode *appsv1.ChainNode, pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		return true
	}

	for _, c := range pod.Status.ContainerStatuses {
		if !c.Ready && c.State.Terminated != nil {
			return true
		}
		if isImagePullFailure(c.State.Waiting) {
			return true
		}
	}

	for _, c := range pod.Status.InitContainerStatuses {
		if !c.Ready && c.State.Terminated != nil && c.State.Terminated.ExitCode != 0 {
			if c.Name == cosmoGuardContainerName {
				if chainNode.Spec.Config.ShouldRestartPodOnCosmoGuardFailure() {
					return true
				}
			}
			if chainNode.Spec.Config != nil {
				for _, s := range chainNode.Spec.Config.Sidecars {
					if s.Name == c.Name && s.ShouldRestartPodOnFailure() {
						return true
					}
				}
			}
		}
		if isImagePullFailure(c.State.Waiting) {
			return true
		}
	}
	return false
}

func isImagePullFailure(state *corev1.ContainerStateWaiting) bool {
	return state != nil && (state.Reason == ReasonImagePullBackOff || state.Reason == ReasonErrImagePull)
}

func nodeUtilsIsInFailedState(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

type terminalPodRecoveryAction uint8

const (
	terminalPodNotTerminated terminalPodRecoveryAction = iota
	terminalPodWaitForEvidence
	terminalPodRestart
	terminalPodUpgrade
	terminalPodHold
)

func terminalPodRecoveryFor(chainNode *appsv1.ChainNode, pod *corev1.Pod) terminalPodRecoveryAction {
	if pod == nil || isPodTerminating(pod) {
		return terminalPodNotTerminated
	}
	terminated, found := appTermination(pod, chainNode.Spec.App.App)
	if !found {
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return terminalPodRestart
		}
		return terminalPodNotTerminated
	}
	evidence, ok := nodeUtilsTerminationEvidence(pod)
	if ok && !evidence.ForcedShutdown && matchingPendingUpgrade(chainNode, evidence.RequiredUpgrade) {
		return terminalPodUpgrade
	}
	if !nodeUtilsHasTerminated(pod) {
		return terminalPodWaitForEvidence
	}
	if !validHaltTermination(terminated) {
		return terminalPodRestart
	}
	if ok && validHaltEvidence(chainNode, pod, evidence) {
		return terminalPodHold
	}
	return terminalPodRestart
}

func terminalPodRecoveryWithoutNodeUtilsEvidence(
	chainNode *appsv1.ChainNode,
	pod *corev1.Pod,
	latestHeight int64,
	latestHeightAvailable bool,
) terminalPodRecoveryAction {
	if pod == nil || isPodTerminating(pod) {
		return terminalPodNotTerminated
	}
	terminated, ok := appTermination(pod, chainNode.Spec.App.App)
	if !ok || !validHaltTermination(terminated) {
		return terminalPodRestart
	}
	haltHeight, ok := configuredPodHaltHeight(chainNode, pod)
	if !ok {
		return terminalPodRestart
	}
	if latestHeightAvailable && atHaltBoundary(latestHeight, haltHeight) {
		return terminalPodHold
	}
	return terminalPodRestart
}

func appTermination(pod *corev1.Pod, name string) (*corev1.ContainerStateTerminated, bool) {
	for i := range pod.Status.ContainerStatuses {
		status := &pod.Status.ContainerStatuses[i]
		if status.Name == name && status.State.Terminated != nil {
			return status.State.Terminated, true
		}
	}
	return nil, false
}

func cleanTermination(terminated *corev1.ContainerStateTerminated) bool {
	return terminated.ExitCode == 0 && terminated.Reason != "OOMKilled" && terminated.Reason != "Error"
}

func validHaltTermination(terminated *corev1.ContainerStateTerminated) bool {
	return cleanTermination(terminated) ||
		terminated.ExitCode != 0 && terminated.ExitCode != 137 && terminated.Signal != 9 && terminated.Reason == "Error"
}

func nodeUtilsHasTerminated(pod *corev1.Pod) bool {
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name == nodeUtilsContainerName {
			return status.State.Terminated != nil
		}
	}
	return pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded
}

func nodeUtilsTerminationEvidence(pod *corev1.Pod) (nodeutils.TerminationEvidence, bool) {
	if pod == nil {
		return nodeutils.TerminationEvidence{}, false
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name != nodeUtilsContainerName || status.State.Terminated == nil || status.State.Terminated.Message == "" {
			continue
		}
		var evidence nodeutils.TerminationEvidence
		if err := json.Unmarshal([]byte(status.State.Terminated.Message), &evidence); err != nil {
			return nodeutils.TerminationEvidence{}, false
		}
		return evidence, true
	}
	return nodeutils.TerminationEvidence{}, false
}

func validHaltEvidence(chainNode *appsv1.ChainNode, pod *corev1.Pod, evidence nodeutils.TerminationEvidence) bool {
	if evidence.ForcedShutdown {
		return false
	}
	haltHeight, ok := configuredPodHaltHeight(chainNode, pod)
	if !ok || evidence.HaltHeight != haltHeight {
		return false
	}
	if evidence.LatestHeight == nil || !atHaltBoundary(*evidence.LatestHeight, haltHeight) {
		return false
	}
	terminated, ok := appTermination(pod, chainNode.Spec.App.App)
	return ok && validHaltTermination(terminated) && !isPodTerminating(pod)
}

func atHaltBoundary(latestHeight, haltHeight int64) bool {
	return latestHeight == haltHeight || latestHeight == haltHeight-1
}

func (r *Reconciler) persistTerminalHaltLatestHeight(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	evidence, ok := nodeUtilsTerminationEvidence(pod)
	if !ok || evidence.LatestHeight == nil || *evidence.LatestHeight <= chainNode.Status.LatestHeight {
		return nil
	}

	previousHeight := chainNode.Status.LatestHeight
	chainNode.Status.LatestHeight = *evidence.LatestHeight
	if err := r.Status().Update(ctx, chainNode); err != nil {
		chainNode.Status.LatestHeight = previousHeight
		return err
	}
	return nil
}

func configuredPodHaltHeight(chainNode *appsv1.ChainNode, pod *corev1.Pod) (int64, bool) {
	if chainNode == nil || chainNode.Spec.Config == nil || chainNode.Spec.Config.HaltHeight == nil {
		return 0, false
	}
	haltHeight := *chainNode.Spec.Config.HaltHeight
	podHaltHeight, ok := nodeUtilsHaltHeight(pod)
	return haltHeight, ok && haltHeight > 0 && podHaltHeight == haltHeight
}

func nodeUtilsHaltHeight(pod *corev1.Pod) (int64, bool) {
	if pod == nil {
		return 0, false
	}
	for _, container := range pod.Spec.InitContainers {
		if container.Name != nodeUtilsContainerName {
			continue
		}
		var found *int64
		for _, env := range container.Env {
			if env.Name == "HALT_HEIGHT" {
				height, err := strconv.ParseInt(env.Value, 10, 64)
				if err != nil || (found != nil && *found != height) {
					return 0, false
				}
				found = ptr.To(height)
			}
		}
		if found == nil {
			return 0, false
		}
		return *found, true
	}
	return 0, false
}

func matchingPendingUpgrade(chainNode *appsv1.ChainNode, required *nodeutils.RequiredUpgrade) bool {
	if required == nil {
		return false
	}
	for _, upgrade := range chainNode.Status.Upgrades {
		if upgrade.Height == required.Height && upgrade.Source == appsv1.UpgradeSource(required.Source) &&
			(upgrade.Status == appsv1.UpgradeScheduled || upgrade.Status == appsv1.UpgradeOnGoing) {
			return true
		}
	}
	return false
}

func (r *Reconciler) reconcileHaltHeightHold(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	return r.reconcileHaltHeightHoldForAction(ctx, chainNode, terminalPodRecoveryFor(chainNode, pod))
}

func shouldMigrateLegacyHaltHeightHold(chainNode *appsv1.ChainNode) bool {
	if chainNode == nil || chainNode.Status.Phase != appsv1.PhaseChainNodeStopped ||
		chainNode.Spec.Config == nil || chainNode.Spec.Config.HaltHeight == nil ||
		chainNode.GetAnnotations()[appsv1.AnnotationHaltHeightHold] != "" {
		return false
	}
	haltHeight := *chainNode.Spec.Config.HaltHeight
	return haltHeight > 0 && chainNode.Status.LatestHeight == haltHeight
}

func (r *Reconciler) reconcileHaltHeightHoldForAction(ctx context.Context, chainNode *appsv1.ChainNode, action terminalPodRecoveryAction) error {
	annotations := chainNode.GetAnnotations()
	existing := annotations[appsv1.AnnotationHaltHeightHold]
	desired := ""
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.HaltHeight != nil {
		configured := strconv.FormatInt(*chainNode.Spec.Config.HaltHeight, 10)
		if existing == configured || action == terminalPodHold {
			desired = configured
		}
	}
	if desired == existing {
		return nil
	}
	annotations = maps.Clone(annotations)
	if annotations == nil {
		annotations = map[string]string{}
	}
	if desired == "" {
		delete(annotations, appsv1.AnnotationHaltHeightHold)
	} else {
		annotations[appsv1.AnnotationHaltHeightHold] = desired
	}
	chainNode.SetAnnotations(annotations)
	return r.Update(ctx, chainNode)
}

func (r *Reconciler) stopNodeUtilsContainer(ctx context.Context, chainNode *appsv1.ChainNode) error {
	return nodeutils.NewClient(chainNode.GetNodeFQDN()).ShutdownNodeUtilsServer(ctx)
}

func isPodTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func (r *Reconciler) shouldSkipSidecar(ctx context.Context, chainNode *appsv1.ChainNode, sidecar appsv1.SidecarSpec, podLabels map[string]string) bool {
	logger := log.FromContext(ctx)

	if _, belongsToGroup := chainNode.Labels[controllers.LabelChainNodeSetGroup]; belongsToGroup && sidecar.DeferUntilHealthyEnabled() {
		pdbList := &policyv1.PodDisruptionBudgetList{}
		if err := r.List(ctx, pdbList, client.InNamespace(chainNode.Namespace)); err != nil {
			logger.Info("failed to list pdb for sidecar skip check", "sidecar", sidecar.Name, "err", err)
			return false
		}

		// Find PDB that matches this chainNode's group
		for _, pdb := range pdbList.Items {
			// Check if PDB selector matches this chainNode
			if pdb.Spec.Selector != nil {
				selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
				if err != nil {
					logger.Info("failed to create selector from pdb selector", "sidecar", sidecar.Name, "err", err)
					continue
				}

				// Check if this PDB applies to nodes in this group
				if selector.Matches(labels.Set(podLabels)) {
					if pdb.Status.CurrentHealthy < pdb.Status.DesiredHealthy {
						logger.Info("skipping sidecar until group is healthy", "sidecar", sidecar.Name)
						return true
					}
					// PDB found and group is healthy, don't skip
					return false
				}
			}
		}
	}
	return false
}
