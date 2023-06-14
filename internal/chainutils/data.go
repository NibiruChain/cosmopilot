package chainutils

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

func (a *App) InitPvcData(ctx context.Context, pvc *corev1.PersistentVolumeClaim, initCommands ...*InitCommand) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-init-data", a.owner.GetName()),
			Namespace: a.owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
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
							ClaimName: pvc.GetName(),
						},
					},
				},
				{
					Name: "home",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "temp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "app",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args:            []string{"init", "test", "--home", "/home/app"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "home",
							MountPath: "/home/app",
						},
						{
							Name:      "data",
							MountPath: "/home/app/data",
						},
					},
				},
			},
			Containers: []corev1.Container{
				// no-op container
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"echo"},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(0),
		},
	}

	// Add additional commands
	for i, cmd := range initCommands {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:    fmt.Sprintf("init-command-%d", i),
			Image:   cmd.Image,
			Command: cmd.Command,
			Args:    cmd.Args,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "home",
					MountPath: "/home/app",
				},
				{
					Name:      "data",
					MountPath: "/home/app/data",
				},
				{
					Name:      "temp",
					MountPath: "/temp",
				},
			},
		})
	}

	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return err
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return err
	}
	return ph.WaitForPodSucceeded(ctx, 5*time.Minute)
}
