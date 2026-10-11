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

	"github.com/voluzi/cosmopilot/v2/internal/k8s"
)

// BuildInitPod constructs the init pod spec without creating it.
// This is useful for inspecting or modifying the pod before creation.
func (a *App) BuildInitPod(pvc *corev1.PersistentVolumeClaim, additionalVolumes []AdditionalVolume, initCommands ...*InitCommand) (*corev1.Pod, error) {
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

	// Build additional volume mounts
	additionalVolumeMounts := make([]corev1.VolumeMount, len(additionalVolumes))
	for i, vol := range additionalVolumes {
		additionalVolumeMounts[i] = corev1.VolumeMount{
			Name:      vol.Name,
			MountPath: vol.Path,
		}
	}

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
			SecurityContext:   k8s.RestrictedPodSecurityContext(),
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
					SecurityContext: k8s.RestrictedSecurityContext(),
				},
			},
			Containers: []corev1.Container{
				// no-op container
				{
					Name:            "busybox",
					Image:           "busybox",
					Command:         []string{"echo"},
					SecurityContext: k8s.RestrictedSecurityContext(),
				},
			},
			TerminationGracePeriodSeconds: ptr.To[int64](0),
		},
	}

	// Add additional volumes
	for _, vol := range additionalVolumes {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: vol.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: vol.PVCName,
				},
			},
		})
	}

	// Build volume mounts for init commands (includes additional volumes)
	initCommandVolumeMounts := append([]corev1.VolumeMount{homeVolumeMount, dataVolumeMount, tempVolumeMount}, additionalVolumeMounts...)

	// Add additional commands
	for i, cmd := range initCommands {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:            fmt.Sprintf("init-command-%d", i),
			Image:           cmd.Image,
			Command:         cmd.Command,
			Args:            cmd.Args,
			VolumeMounts:    initCommandVolumeMounts,
			Resources:       cmd.Resources,
			Env:             cmd.Env,
			SecurityContext: k8s.RestrictedSecurityContext(),
		})
	}

	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return nil, err
	}

	return pod, nil
}

// CreateInitPod creates the init pod without waiting for completion.
// The caller is responsible for monitoring the pod status and cleaning up.
// The timeout is enforced via activeDeadlineSeconds on the pod spec.
func (a *App) CreateInitPod(ctx context.Context, pvc *corev1.PersistentVolumeClaim, timeout time.Duration, additionalVolumes []AdditionalVolume, initCommands ...*InitCommand) error {
	pod, err := a.BuildInitPod(pvc, additionalVolumes, initCommands...)
	if err != nil {
		return err
	}

	// Enforce timeout via Kubernetes activeDeadlineSeconds
	// This causes the pod to transition to Failed if it exceeds the deadline
	if timeout > 0 {
		deadlineSeconds := int64(timeout.Seconds())
		pod.Spec.ActiveDeadlineSeconds = &deadlineSeconds
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)
	return ph.Create(ctx)
}

// InitPvcData creates an init pod and blocks until it completes or times out.
// Deprecated: This method blocks the reconciliation loop. Use CreateInitPod with
// status monitoring in the controller instead. This method is kept for backwards
// compatibility but will delete any existing init pod, losing progress on restart.
func (a *App) InitPvcData(ctx context.Context, pvc *corev1.PersistentVolumeClaim, timeout time.Duration, additionalVolumes []AdditionalVolume, initCommands ...*InitCommand) error {
	pod, err := a.BuildInitPod(pvc, additionalVolumes, initCommands...)
	if err != nil {
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
