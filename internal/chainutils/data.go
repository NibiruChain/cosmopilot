package chainutils

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/cosmopilot/internal/k8s"
)

func (a *App) InitPvcData(ctx context.Context, pvc *corev1.PersistentVolumeClaim, timeout time.Duration, initCommands ...*InitCommand) error {

	var (
		homeVolumeMount = corev1.VolumeMount{
			Name:      "home",
			MountPath: defaultHome,
		}
		dataVolumeMount = corev1.VolumeMount{
			Name:      "data",
			MountPath: filepath.Join(defaultHome, defaultData),
		}
		tempVolumeMount = corev1.VolumeMount{
			Name:      "temp",
			MountPath: "/temp",
		}
	)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-init-data", a.owner.GetName()),
			Namespace: a.owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:     corev1.RestartPolicyNever,
			PriorityClassName: a.priorityClassName,
			Affinity:          a.Affinity,
			NodeSelector:      a.NodeSelector,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  ptr.To[int64](nonRootId),
				RunAsGroup: ptr.To[int64](nonRootId),
				FSGroup:    ptr.To[int64](nonRootId),
			},
			Volumes: []corev1.Volume{
				{
					Name: dataVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.GetName(),
						},
					},
				},
				{
					Name: homeVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: tempVolumeMount.Name,
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
					Args:            a.cmd.InitArgs(none, none),
					VolumeMounts:    []corev1.VolumeMount{homeVolumeMount, dataVolumeMount},
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
			TerminationGracePeriodSeconds: ptr.To[int64](0),
		},
	}

	// Add additional commands
	for i, cmd := range initCommands {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:         fmt.Sprintf("init-command-%d", i),
			Image:        cmd.Image,
			Command:      cmd.Command,
			Args:         cmd.Args,
			VolumeMounts: []corev1.VolumeMount{homeVolumeMount, dataVolumeMount, tempVolumeMount},
		})
	}

	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return err
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer func() { _ = ph.Delete(ctx) }()

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return err
	}
	return ph.WaitForPodSucceeded(ctx, timeout)
}
