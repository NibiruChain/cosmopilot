package chainnode

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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

	currentPod := &corev1.Pod{}
	updatedPod, err := r.getPodSpec(ctx, chainNode)
	if err != nil {
		return err
	}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), currentPod)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pod")
			return r.Create(ctx, updatedPod)
		}
		return err
	}

	// Re-create pod if spec changes
	if !equality.Semantic.DeepDerivative(updatedPod.Spec, currentPod.Spec) {
		logger.Info("recreating pod")
		if err := r.Delete(ctx, currentPod); err != nil {
			return err
		}
		return r.Create(ctx, updatedPod)
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
					Name: "secret",
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
						},
						{
							Name:          chainutils.RpcPortName,
							ContainerPort: chainutils.Rpcport,
						},
						{
							Name:          chainutils.LcdPortName,
							ContainerPort: chainutils.LcdPort,
						},
						{
							Name:          chainutils.GrpcPortName,
							ContainerPort: chainutils.GrpcPort,
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
							Name:      "secret",
							MountPath: "/secret",
						},
					}, configFilesMounts...),
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(chainNode, pod, r.Scheme); err != nil {
		return nil, err
	}

	return pod, nil
}
