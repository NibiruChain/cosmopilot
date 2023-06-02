package chainnode

import (
	"context"
	"fmt"
	"sort"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

func (r *Reconciler) ensurePod(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	// Prepare pod spec
	updatedPod, err := r.getPodSpec(ctx, chainNode)
	if err != nil {
		return err
	}

	// Get current pod. If it does not exist create it and exit.
	currentPod := &corev1.Pod{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), currentPod)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pod")
			return r.Create(ctx, updatedPod)
		}
		return err
	}

	// Re-create pod if spec changes
	if podSpecChanged(currentPod, updatedPod) {
		logger.Info("pod spec changed")
		return r.recreatePod(ctx, updatedPod)
	}

	// Recreate pod if it is in failed state
	if currentPod.Status.Phase == corev1.PodFailed {
		logger.Info("pod is in failed state")
		return r.recreatePod(ctx, updatedPod)
	}

	return nil
}

func (r *Reconciler) getPodSpec(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.Pod, error) {
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
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
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
			},
			Containers: []corev1.Container{
				{
					Name:            "app",
					Image:           chainNode.GetImage(),
					ImagePullPolicy: chainNode.GetImagePullPolicy(),
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
							ContainerPort: chainutils.Rpcport,
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
					},
					VolumeMounts: append([]corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app/data",
						},
						{
							Name:      "genesis",
							MountPath: "/genesis",
						},
						{
							Name:      "node-key",
							MountPath: "/secret/" + nodeKeyFilename,
							SubPath:   nodeKeyFilename,
						},
					}, configFilesMounts...),
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
				ImagePullPolicy: chainNode.GetSidecarImagePullPolicy(c.Name),
				Command:         c.Command,
				Args:            c.Args,
				Env:             c.Env,
			}

			if c.MountDataVolume != nil {
				container.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "data",
						MountPath: *c.MountDataVolume,
					},
				}
			}

			pod.Spec.Containers = append(pod.Spec.Containers, container)
		}
	}

	if chainNode.IsValidator() {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "priv-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: chainNode.GetValidatorPrivKeySecretName(),
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "priv-key",
			MountPath: "/secret/" + privKeyFilename,
			SubPath:   privKeyFilename,
		})
	}

	return pod, controllerutil.SetControllerReference(chainNode, pod, r.Scheme)
}

func (r *Reconciler) recreatePod(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	logger.Info("recreating pod")
	if err := r.Delete(ctx, pod); err != nil {
		return err
	}
	return r.Create(ctx, pod)
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
