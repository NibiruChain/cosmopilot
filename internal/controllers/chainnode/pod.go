package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/internal/k8s"
	"github.com/NibiruChain/cosmopilot/pkg/nodeutils"
)

func (r *Reconciler) ensurePod(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode, configHash string) error {
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
			return r.setNodePhase(ctx, chainNode)
		}
		return err
	}

	if nodeUtilsIsInFailedState(currentPod) {
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

	if err = r.updateLatestHeight(ctx, chainNode); err != nil {
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

		// Force update config files, to prevent restarting again because of config changes
		app, err = chainutils.NewApp(r.ClientSet, r.Scheme, r.RestConfig, chainNode,
			chainNode.Spec.App.GetSdkVersion(),
			chainutils.WithImage(chainNode.GetAppImage()),
			chainutils.WithImagePullPolicy(chainNode.Spec.App.ImagePullPolicy),
			chainutils.WithBinary(chainNode.Spec.App.App),
		)
		if err != nil {
			return err
		}
		configHash, err = r.ensureConfigMap(ctx, app, chainNode)
		if err != nil {
			return err
		}

		// Get new pod spec with updated configs
		pod, err = r.getPodSpec(ctx, chainNode, configHash)
		if err != nil {
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
				// If there was an error on pod creation or watching but the image was already swapped, we mark the upgrade
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
	if podInFailedState(chainNode, currentPod) {
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

	// Patch pod without restart when labels change
	if !reflect.DeepEqual(currentPod.Labels, pod.Labels) {
		logger.Info("updating pod labels", "pod", pod.GetName())
		modifiedPod := currentPod.DeepCopy()
		modifiedPod.Labels = pod.Labels
		_, err = r.PatchPod(ctx, currentPod, modifiedPod)
		return err
	}

	return r.setNodePhase(ctx, chainNode)
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

	var sidecarRestartAlways = corev1.ContainerRestartPolicyAlways

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
			TerminationGracePeriodSeconds: chainNode.Spec.Config.GetTerminationGracePeriodSeconds(),
			Volumes: []corev1.Volume{
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
					Name:            nodeUtilsContainerName,
					Image:           r.opts.NodeUtilsImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					RestartPolicy:   &sidecarRestartAlways,
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
						{
							Name:  "CREATE_FIFO",
							Value: StringValueTrue,
						},
						{
							Name:  "TRACE_STORE",
							Value: "/trace/trace.fifo",
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
			Containers: []corev1.Container{
				{
					Name:            chainNode.Spec.App.App,
					Image:           chainNode.GetAppImage(),
					ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
					Command:         []string{chainNode.Spec.App.App},
					Args: append([]string{"start",
						"--home", "/home/app",
						"--trace-store", "/trace/trace.fifo",
					}, chainNode.GetAdditionalRunFlags()...),
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
			},
		},
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Volumes != nil {
		for _, volume := range chainNode.Spec.Config.Volumes {
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
				Name:    "link-genesis",
				Image:   "busybox",
				Command: []string{"/bin/sh"},
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
			container := corev1.Container{
				Name:            c.Name,
				Image:           c.Image,
				ImagePullPolicy: chainNode.Spec.Config.GetSidecarImagePullPolicy(c.Name),
				RestartPolicy:   &sidecarRestartAlways,
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

			pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
		}
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.SafeToEvict != nil {
		pod.Annotations[annotationSafeEvict] = strconv.FormatBool(*chainNode.Spec.Config.SafeToEvict)
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
			Args:            []string{"-config", filepath.Join("/config/", chainNode.Spec.Config.GetCosmoGuardConfig().Key)},
			Ports: []corev1.ContainerPort{
				{
					Name:          chainutils.RpcPortName,
					ContainerPort: controllers.CosmoGuardRpcPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          chainutils.LcdPortName,
					ContainerPort: controllers.CosmoGuardLcdPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          chainutils.GrpcPortName,
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
				Name:          controllers.EvmRpcPortName,
				ContainerPort: controllers.CosmoGuardEvmRpcPort,
				Protocol:      corev1.ProtocolTCP,
			})
			cosmoGuardContainer.Ports = append(cosmoGuardContainer.Ports, corev1.ContainerPort{
				Name:          controllers.EvmRpcWsPortName,
				ContainerPort: controllers.CosmoGuardEvmRpcWsPort,
				Protocol:      corev1.ProtocolTCP,
			})
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, cosmoGuardContainer)
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

	// Attempt to terminate node-utils container without waiting for grace-period. If there is an error
	// we will just wait for the grace-period
	if err := r.stopNodeUtilsContainer(chainNode); err != nil {
		logger.Info("failed to stop node utils container", "pod", pod.GetName(), "error", err.Error())
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
	return r.setNodePhase(ctx, chainNode)
}

func (r *Reconciler) upgradePod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod, image string) (bool, error) {
	logger := log.FromContext(ctx)

	logger.Info("upgrading pod", "pod", pod.GetName())
	phase := appsv1.PhaseChainNodeUpgrading
	if err := r.updatePhase(ctx, chainNode, phase); err != nil {
		return false, err
	}

	// Attempt to terminate node-utils container without waiting for grace-period. If there is an error
	// we will just wait for the grace-period
	if err := r.stopNodeUtilsContainer(chainNode); err != nil {
		logger.Info("failed to stop node utils container", "pod", pod.GetName(), "error", err.Error())
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
	return true, r.setNodePhase(ctx, chainNode)
}

func (r *Reconciler) PatchPod(ctx context.Context, cur, mod *corev1.Pod) (*corev1.Pod, error) {
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
	if len(pa) == 0 || string(pa) == "{}" {
		return cur, nil
	}
	return r.ClientSet.CoreV1().Pods(cur.GetNamespace()).
		Patch(ctx, cur.GetName(), types.StrategicMergePatchType, pa, metav1.PatchOptions{})
}

func podSpecChanged(existing, new *corev1.Pod) bool {
	// make copies
	existingCopy := existing.DeepCopy()
	newCopy := new.DeepCopy()

	// remove fields populated by kubernetes
	removeFieldsForComparison(existingCopy)

	// ignore labels because we can patch them without restart
	existingCopy.Labels = newCopy.Labels

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

func (r *Reconciler) setNodePhase(ctx context.Context, chainNode *appsv1.ChainNode) error {
	if volumeSnapshotInProgress(chainNode) {
		if chainNode.Status.Phase != appsv1.PhaseChainNodeSnapshotting {
			return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeSnapshotting)
		}
	}

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

func podInFailedState(chainNode *appsv1.ChainNode, pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		return true
	}

	for _, c := range pod.Status.ContainerStatuses {
		if !c.Ready && c.State.Terminated != nil {
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
	}
	return false
}

func nodeUtilsIsInFailedState(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	for _, c := range pod.Status.InitContainerStatuses {
		if c.Name == nodeUtilsContainerName && !c.Ready && c.State.Terminated != nil {
			return true
		}
	}

	return false
}

func (r *Reconciler) stopNodeUtilsContainer(chainNode *appsv1.ChainNode) error {
	return nodeutils.NewClient(chainNode.GetNodeFQDN()).ShutdownNodeUtilsServer()
}
