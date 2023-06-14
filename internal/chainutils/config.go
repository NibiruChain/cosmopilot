package chainutils

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

func (a *App) GenerateConfigFiles(ctx context.Context) (map[string]string, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config-generator", a.owner.GetName()),
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
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "home",
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
							Name:      "config",
							MountPath: "/home/app/config",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"cat"},
					Stdin:   true,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/home/app/config",
						},
					},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(0),
		},
	}
	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return nil, err
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return nil, err
	}

	// Wait for the pod to be running
	if err := ph.WaitForPodRunning(ctx, time.Minute); err != nil {
		return nil, err
	}

	// Grab list of config files
	out, _, err := ph.Exec(ctx,
		"busybox",
		[]string{"sh", "-c", "find /home/app/config -type f -name '*.toml' -exec basename {} \\;"},
	)
	if err != nil {
		return nil, err
	}
	filenames := strings.Split(strings.TrimSpace(out), "\n")

	// Get each config file content
	configs := make(map[string]string)
	for _, filename := range filenames {
		configs[filename], _, err = ph.Exec(ctx, "busybox", []string{"cat", "/home/app/config/" + filename})
		if err != nil {
			return nil, err
		}
	}

	return configs, nil
}
