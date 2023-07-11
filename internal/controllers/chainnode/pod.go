package chainnode

import (
	"context"
	"fmt"
	"sort"
	"strconv"

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
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
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
			logger.Info("creating pod")
			if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeStarting); err != nil {
				return err
			}

			ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
			if err := ph.Create(ctx); err != nil {
				return err
			}
			if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, appContainerName); err != nil {
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

	// Re-create pod if config changed
	if currentPod.Annotations[annotationConfigHash] != configHash {
		logger.Info("config changed")
		return r.recreatePod(ctx, chainNode, pod)
	}

	// Re-create pod if spec changes
	if podSpecChanged(currentPod, pod) {
		logger.Info("pod spec changed")
		return r.recreatePod(ctx, chainNode, pod)
	}

	// Recreate pod if it is in failed state
	if podInFailedState(currentPod) {
		logger.Info("pod is in failed state")
		return r.recreatePod(ctx, chainNode, pod)
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

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.GetName(),
			Namespace: chainNode.GetNamespace(),
			Annotations: map[string]string{
				annotationConfigHash: configHash,
			},
			Labels: utils.MergeMaps(
				map[string]string{
					LabelNodeID:    chainNode.Status.NodeID,
					LabelChainID:   chainNode.Status.ChainID,
					LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
				},
				chainNode.Labels),
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
					Name: "genesis",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: fmt.Sprintf("%s-genesis", chainNode.Status.ChainID),
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
			},
			Containers: []corev1.Container{
				{
					Name:            appContainerName,
					Image:           chainNode.Spec.App.GetImage(),
					ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
					Command:         []string{chainNode.Spec.App.App},
					Args:            []string{"start", "--home", "/home/app"},
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
							Name:      "genesis",
							MountPath: "/home/app/config/" + genesisFilename,
							SubPath:   genesisFilename,
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
						FailureThreshold: int32(startupTimeout.Seconds() / 5),
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
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/ready",
								Port: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: nodeUtilsPort,
								},
								Scheme: "HTTP",
							},
						},
						FailureThreshold: 1,
						PeriodSeconds:    10,
					},
					Resources: chainNode.Spec.Resources,
				},
				{
					Name:            nodeUtilsContainerName,
					Image:           r.nodeUtilsImage,
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
					},
					Env: []corev1.EnvVar{
						{
							Name:  "BLOCK_THRESHOLD",
							Value: chainNode.Spec.Config.GetBlockThreshold(),
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    nodeUtilsCpuResources,
							corev1.ResourceMemory: nodeUtilsMemoryResources,
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    nodeUtilsCpuResources,
							corev1.ResourceMemory: nodeUtilsMemoryResources,
						},
					},
				},
			},
		},
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

	if chainNode.IsValidator() && !chainNode.UsesTmKms() {
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
			MountPath: "/home/app/config/" + privKeyFilename,
			SubPath:   privKeyFilename,
		})
	}

	return pod, controllerutil.SetControllerReference(chainNode, pod, r.Scheme)
}

func (r *Reconciler) recreatePod(ctx context.Context, chainNode *appsv1.ChainNode, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	logger.Info("recreating pod")

	if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeRestarting); err != nil {
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

	if err := ph.WaitForContainerStarted(ctx, timeoutPodRunning, appContainerName); err != nil {
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

func podSpecChanged(existing, new *corev1.Pod) bool {
	// make copies
	existingCopy := existing.DeepCopy()
	newCopy := new.DeepCopy()

	// remove fields populated by kubernetes
	removeFieldsForComparison(existingCopy)

	// order volume mounts
	orderVolumeMounts(existingCopy)
	orderVolumeMounts(newCopy)

	patchResult, err := patch.DefaultPatchMaker.Calculate(existingCopy, newCopy,
		patch.IgnoreStatusFields(),
		patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(),
	)
	if err != nil {
		return false
	}
	fmt.Println(string(patchResult.Patch))
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
	c, err := r.getQueryClient(chainNode)
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
			chainNode.Status.AppVersion = chainNode.Spec.App.GetImageVersion()
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
		chainNode.Status.AppVersion = chainNode.Spec.App.GetImageVersion()
		return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeRunning)
	}

	return nil
}

func podInFailedState(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}

	for _, c := range pod.Status.ContainerStatuses {
		if !c.Ready && c.State.Terminated != nil {
			if c.State.Terminated.ExitCode != 0 {
				return true
			}
		}
	}
	return false
}
