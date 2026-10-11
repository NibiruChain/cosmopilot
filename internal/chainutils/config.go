package chainutils

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/voluzi/cosmopilot/v2/internal/k8s"
)

func (a *App) GenerateConfigFiles(ctx context.Context) (map[string]string, error) {
	var (
		homeVolumeMount = corev1.VolumeMount{
			Name:      "home",
			MountPath: defaultHome,
		}
		configVolumeMount = corev1.VolumeMount{
			Name:      "config",
			MountPath: filepath.Join(defaultHome, defaultConfig),
		}
	)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config-generator", a.owner.GetName()),
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
					Name: configVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: homeVolumeMount.Name,
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
					VolumeMounts:    []corev1.VolumeMount{homeVolumeMount, configVolumeMount},
					SecurityContext: k8s.RestrictedSecurityContext(),
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					Command:         []string{"cat"},
					Stdin:           true,
					VolumeMounts:    []corev1.VolumeMount{configVolumeMount},
					SecurityContext: k8s.RestrictedSecurityContext(),
				},
			},
			TerminationGracePeriodSeconds: ptr.To[int64](0),
		},
	}
	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference: %w", err)
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer func() { _ = ph.Delete(ctx) }()

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return nil, fmt.Errorf("creating config generator pod: %w", err)
	}

	// Wait for the pod to be running
	if err := ph.WaitForPodRunning(ctx, time.Minute); err != nil {
		return nil, fmt.Errorf("waiting for config generator pod: %w", err)
	}

	// Grab list of config files
	out, _, err := ph.Exec(ctx,
		"busybox",
		[]string{"sh", "-c", fmt.Sprintf("find %s -type f -name '*.toml' -exec basename {} \\;", filepath.Join(defaultHome, defaultConfig))},
	)
	if err != nil {
		return nil, fmt.Errorf("listing config files: %w", err)
	}
	filenames := strings.Split(strings.TrimSpace(out), "\n")

	// Get each config file content
	configs := make(map[string]string)
	for _, filename := range filenames {
		configs[filename], _, err = ph.Exec(ctx, "busybox", []string{"cat", filepath.Join(defaultHome, defaultConfig, filename)})
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", filename, err)
		}
	}

	return configs, nil
}
