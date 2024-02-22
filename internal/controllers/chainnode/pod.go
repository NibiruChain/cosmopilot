package chainnode

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/controllers"
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

func (r *Reconciler) ensurePod(ctx context.Context, chainNode *appsv1.ChainNode, configHash string) error {
	logger := log.FromContext(ctx)

	// Prepare pod spec
	pod, err := r.getPodSpec(ctx, chainNode, configHash)
	if err != nil {
		return err
	}

	// Get current pod. If it does not exist create it and exit.
	currentPod := &corev1.Pod{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), currentPod)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pod", "pod", pod.GetName())
			if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeStarting); err != nil {
				return err
			}

			ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
			if err := ph.Create(ctx); err != nil {
				return err
			}
			if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
				return err
			}
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonNodeStarted,
				"Node successfully started",
			)
			return r.setPhaseRunningOrSyncing(ctx, chainNode)
		}
		return err
	}

	if nodeUtilsIsInFailedState(pod) {
		logger.Info("node-utils is in failed state", "pod", pod.GetName())
		ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, currentPod)
		logs, err := ph.GetLogs(ctx, nodeUtilsContainerName)
		if err != nil {
			logger.Info("could not retrieve logs: " + err.Error())
		} else {
			logLines := strings.Split(logs, "\n")
			if len(logLines) > defaultLogsLineCount {
				logger.Info("app error: " + strings.Join(logLines[len(logLines)-defaultLogsLineCount:], "/n"))
			} else {
				logger.Info("app error: " + strings.Join(logLines, "/n"))
			}
		}
		return r.recreatePod(ctx, chainNode, pod)
	}

	if err := r.updateLatestHeight(ctx, chainNode); err != nil {
		return err
	}

	// Check if the node is waiting for an upgrade
	requiresUpgrade, err := r.requiresUpgrade(chainNode)
	if err != nil {
		return err
	}

	if requiresUpgrade {
		// Get upgrade from scheduled upgrades list
		upgrade := r.getUpgrade(chainNode, chainNode.Status.LatestHeight)

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
			return err
		}

		if upgraded, err := r.upgradePod(ctx, chainNode, pod, upgrade.Image); err != nil {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonUpgradeFailed,
				"Failed to restart for upgrade: %v",
				err,
			)
			var upgradeStatus appsv1.UpgradePhase
			if upgraded {
				// If there was an error on pod creation or watching but the image was already switched, we marked the upgrade
				// completed anyway to avoid downgrading and corrupt data.
				chainNode.Status.AppVersion = upgrade.GetVersion()
				upgradeStatus = appsv1.UpgradeCompleted
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
		chainNode.Status.AppVersion = upgrade.GetVersion()
		return r.setUpgradeStatus(ctx, chainNode, upgrade, appsv1.UpgradeCompleted)
	}

	// Recreate pod if it is in failed state
	if podInFailedState(currentPod) {
		logger.Info("pod is in failed state", "pod", pod.GetName())
		ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, currentPod)
		logs, err := ph.GetLogs(ctx, chainNode.Spec.App.App)
		if err != nil {
			logger.Info("could not retrieve logs: " + err.Error())
		} else {
			logLines := strings.Split(logs, "\n")
			if len(logLines) > defaultLogsLineCount {
				logger.Info("app error: " + strings.Join(logLines[len(logLines)-defaultLogsLineCount:], "/n"))
			} else {
				logger.Info("app error: " + strings.Join(logLines, "/n"))
			}
		}
		return r.recreatePod(ctx, chainNode, pod)
	}

	// Re-create pod if spec changes
	if podSpecChanged(currentPod, pod) {
		logger.Info("pod spec changed", "pod", pod.GetName())
		return r.recreatePod(ctx, chainNode, pod)
	}

	// Re-create pod if config changed
	if currentPod.Annotations[annotationConfigHash] != configHash {
		logger.Info("config changed", "pod", pod.GetName())
		return r.recreatePod(ctx, chainNode, pod)
	}

	if volumeSnapshotInProgress(chainNode) {
		return nil
	}
	return r.setPhaseRunningOrSyncing(ctx, chainNode)
}

func (r *Reconciler) getPodSpec(ctx context.Context, chainNode *appsv1.ChainNode, configHash string) (*corev1.Pod, error) {
	// Load configmap to have config file names. We will mount them individually to allow the config
	// dir to be writable. When ConfigMap is mounted as whole, the directory is read only.
	config := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), config)
	if err != nil {
		return nil, err
	}
	configFilesMounts := make([]corev1.VolumeMount, len(config.Data))
	i := 0
	for k := range config.Data {
		configFilesMounts[i] = corev1.VolumeMount{
			Name:      "config",
			MountPath: "/home/app/config/" + k,
			SubPath:   k,
		}
		i++
	}

	readinessPath := "/ready"
	if chainNode.Spec.Config.ShouldIgnoreSyncing() {
		readinessPath = "/health"
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.GetName(),
			Namespace: chainNode.GetNamespace(),
			Annotations: map[string]string{
				annotationConfigHash: configHash,
			},
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				LabelNodeID:    chainNode.Status.NodeID,
				LabelChainID:   chainNode.Status.ChainID,
				LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
			}),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Affinity:      chainNode.Spec.Affinity,
			NodeSelector:  chainNode.Spec.NodeSelector,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  pointer.Int64(nonRootId),
				RunAsGroup: pointer.Int64(nonRootId),
				FSGroup:    pointer.Int64(nonRootId),
			},
			TerminationGracePeriodSeconds: pointer.Int64(10),
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: chainNode.GetName(),
						},
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
					Name: "config-empty-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "trace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
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
			},
			InitContainers: []corev1.Container{
				{
					Name:    "init-trace-fifo",
					Image:   "busybox",
					Command: []string{"mkfifo"},
					Args:    []string{"/trace/trace.fifo"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "trace",
							MountPath: "/trace",
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
			},
			Containers: []corev1.Container{
				{
					Name:            chainNode.Spec.App.App,
					Image:           chainNode.GetAppImage(),
					ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
					Command:         []string{chainNode.Spec.App.App},
					Args: []string{"start",
						"--home", "/home/app",
						"--trace-store", "/trace/trace.fifo",
					},
					Env: chainNode.Spec.Config.GetEnv(),
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
							Name:      "data",
							MountPath: "/home/app/data",
						},
						{
							Name:      "node-key",
							MountPath: "/home/app/config/" + nodeKeyFilename,
							SubPath:   nodeKeyFilename,
						},
						{
							Name:      "config-empty-dir",
							MountPath: "/home/app/config",
						},
						{
							Name:      "trace",
							MountPath: "/trace",
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
						PeriodSeconds:    5,
						FailureThreshold: int32(chainNode.Spec.Config.GetStartupTime().Seconds() / 5),
						TimeoutSeconds:   livenessProbeTimeoutSeconds,
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
						FailureThreshold: 2,
						PeriodSeconds:    30,
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
						FailureThreshold: 1,
						PeriodSeconds:    10,
						TimeoutSeconds:   readinessProbeTimeoutSeconds,
					},
					Resources: chainNode.Spec.Resources,
				},
				{
					Name:            nodeUtilsContainerName,
					Image:           r.opts.NodeUtilsImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
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
							Name:      "trace",
							MountPath: "/trace",
						},
						{
							Name:      "upgrades-config",
							MountPath: "/config",
						},
					},
					Env: []corev1.EnvVar{
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
					},
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
				},
			},
		},
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
						Name: fmt.Sprintf("%s-genesis", chainNode.Status.ChainID),
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
		pod.Spec.InitContainers = []corev1.Container{
			{
				Name:    "link-genesis",
				Image:   "busybox",
				Command: []string{"/bin/sh"},
				Args: []string{
					"-c",
					"ln -s /home/app/data/genesis.json /home/app/config/genesis.json",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "data",
						MountPath: "/home/app/data",
					},
					{
						Name:      "config-empty-dir",
						MountPath: "/home/app/config",
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
		}
	}

	if chainNode.Spec.Config != nil {
		pod.Spec.ImagePullSecrets = chainNode.Spec.Config.ImagePullSecrets
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Sidecars != nil {
		for _, c := range chainNode.Spec.Config.Sidecars {
			container := corev1.Container{
				Name:            c.Name,
				Image:           c.Image,
				ImagePullPolicy: chainNode.Spec.Config.GetSidecarImagePullPolicy(c.Name),
				Command:         c.Command,
				Args:            c.Args,
				Env:             c.Env,
				SecurityContext: c.SecurityContext,
				Resources:       c.Resources,
			}

			if c.MountDataVolume != nil {
				container.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "data",
						MountPath: *c.MountDataVolume,
						ReadOnly:  true,
					},
				}
			}

			pod.Spec.Containers = append(pod.Spec.Containers, container)
		}
	}

	if chainNode.IsValidator() {
		if chainNode.UsesTmKms() {
			kms, err := r.getTmkms(ctx, chainNode)
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

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.SafeToEvict != nil {
		pod.Annotations[annotationSafeEvict] = strconv.FormatBool(*chainNode.Spec.Config.SafeToEvict)
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Firewall.Enabled() {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: firewallVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: chainNode.Spec.Config.Firewall.Config.LocalObjectReference,
				},
			},
		})
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:            firewallContainerName,
			Image:           r.opts.CosmosFirewallImage,
			ImagePullPolicy: corev1.PullAlways,
			Args:            []string{"-config", filepath.Join("/config/", chainNode.Spec.Config.Firewall.Config.Key)},
			Ports: []corev1.ContainerPort{
				{
					Name:          chainutils.RpcPortName,
					ContainerPort: controllers.FirewallRpcPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          chainutils.LcdPortName,
					ContainerPort: controllers.FirewallLcdPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          chainutils.GrpcPortName,
					ContainerPort: controllers.FirewallGrpcPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          controllers.FirewallMetricsPortName,
					ContainerPort: controllers.FirewallMetricsPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      firewallVolumeName,
					MountPath: "/config",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    firewallCpuResources,
					corev1.ResourceMemory: firewallMemoryResources,
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/metrics",
						Port: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: controllers.FirewallMetricsPort,
						},
						Scheme: "HTTP",
					},
				},
				FailureThreshold: 1,
				PeriodSeconds:    2,
			},
		})
	}

	return pod, controllerutil.SetControllerReference(chainNode, pod, r.Scheme)
}

func (r *Reconciler) recreatePod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	logger.Info("recreating pod", "pod", pod.GetName())
	phase := appsv1.PhaseChainNodeRestarting
	if err := r.updatePhase(ctx, chainNode, phase); err != nil {
		return err
	}

	deletePod := pod.DeepCopy()
	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, deletePod)
	if err := ph.Delete(ctx); err != nil {
		return err
	}
	if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
		return err
	}

	ph = k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
	if err := ph.Create(ctx); err != nil {
		return err
	}

	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonNodeError,
			"Error: %v",
			err,
		)
		_ = r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeError)
		return err
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeRestarted,
		"Node restarted",
	)
	return r.setPhaseRunningOrSyncing(ctx, chainNode)
}

func (r *Reconciler) upgradePod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod, image string) (bool, error) {
	logger := log.FromContext(ctx)

	logger.Info("upgrading pod", "pod", pod.GetName())
	phase := appsv1.PhaseChainNodeUpgrading
	if err := r.updatePhase(ctx, chainNode, phase); err != nil {
		return false, err
	}

	deletePod := pod.DeepCopy()
	ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, deletePod)
	if err := ph.Delete(ctx); err != nil {
		return false, err
	}
	if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
		return false, err
	}

	ph = k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
	pod.Spec.Containers[0].Image = image
	if err := ph.Create(ctx); err != nil {
		return false, err
	}

	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, chainNode.Spec.App.App); err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonNodeError,
			"Error: %v",
			err,
		)
		_ = r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeError)
		return true, err
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeRestarted,
		"Node upgraded",
	)
	return true, r.setPhaseRunningOrSyncing(ctx, chainNode)
}

func podSpecChanged(existing, new *corev1.Pod) bool {
	// make copies
	existingCopy := existing.DeepCopy()
	newCopy := new.DeepCopy()

	// remove fields populated by kubernetes
	removeFieldsForComparison(existingCopy)

	// order volume mounts
	orderVolumeMounts(existingCopy)
	orderVolumeMounts(newCopy)

	if len(existingCopy.Spec.Containers) != len(new.Spec.Containers) {
		return true
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(existingCopy, newCopy,
		patch.IgnoreStatusFields(),
		patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(),
	)
	if err != nil {
		return false
	}
	return !patchResult.IsEmpty()
}

func orderVolumeMounts(pod *corev1.Pod) {
	for _, c := range pod.Spec.Containers {
		sort.Slice(c.VolumeMounts, func(i, j int) bool {
			return c.VolumeMounts[i].MountPath < c.VolumeMounts[j].MountPath
		})
	}
}

func removeFieldsForComparison(pod *corev1.Pod) {
	// remove service account volume mount
	for i, c := range pod.Spec.Containers {
		volumeMounts := c.VolumeMounts[:0]
		j := 0
		for _, m := range c.VolumeMounts {
			if m.MountPath != "/var/run/secrets/kubernetes.io/serviceaccount" {
				volumeMounts = append(volumeMounts, m)
				j++
			}
		}
		pod.Spec.Containers[i].VolumeMounts = volumeMounts
	}
}

func (r *Reconciler) setPhaseRunningOrSyncing(ctx context.Context, chainNode *appsv1.ChainNode) error {
	c, err := r.getClient(chainNode)
	if err != nil {
		return err
	}

	syncing, err := c.IsNodeSyncing(ctx)
	if err != nil {
		return err
	}

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

func podInFailedState(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		return true
	}

	for _, c := range pod.Status.ContainerStatuses {
		if !c.Ready && c.State.Terminated != nil {
			return true
		}
	}

	return false
}

func nodeUtilsIsInFailedState(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == nodeUtilsContainerName && !c.Ready && c.State.Terminated != nil {
			return true
		}
	}

	return false
}
