package k8s

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type PvcHelper struct {
	client     *kubernetes.Clientset
	restConfig *rest.Config
	pvc        *corev1.PersistentVolumeClaim
}

func NewPvcHelper(client *kubernetes.Clientset, cfg *rest.Config, pvc *corev1.PersistentVolumeClaim) *PvcHelper {
	return &PvcHelper{
		client:     client,
		restConfig: cfg,
		pvc:        pvc,
	}
}

func (h *PvcHelper) WriteToFile(ctx context.Context, content, path, pc string, af *corev1.Affinity, ns map[string]string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-write-file", h.pvc.GetName()),
			Namespace: h.pvc.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](0),
			PriorityClassName:             pc,
			Affinity:                      af,
			NodeSelector:                  ns,
			Volumes: []corev1.Volume{
				{
					Name: "pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: h.pvc.GetName(),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						fmt.Sprintf("cp /dev/stdin %s", filepath.Join("/pvc", path)),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "pvc",
							MountPath: "/pvc",
						},
					},
					Stdin:     true,
					StdinOnce: true,
				},
			},
		},
	}

	ph := NewPodHelper(h.client, h.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer func() { _ = ph.Delete(ctx) }()

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return err
	}

	// Wait for container to be running
	if err := ph.WaitForContainerStarted(ctx, time.Minute, "busybox"); err != nil {
		return err
	}

	// Attach to container to push file content
	var input bytes.Buffer
	input.WriteString(content)
	if _, _, err := ph.Attach(ctx, "busybox", &input); err != nil {
		return err
	}

	// Wait for the pod to be running
	if err := ph.WaitForPodSucceeded(ctx, time.Minute); err != nil {
		return err
	}

	return nil
}

func (h *PvcHelper) DownloadGenesis(ctx context.Context, url, path, pc string, af *corev1.Affinity, ns map[string]string) error {
	cmd := fmt.Sprintf("wget -O %s %s", filepath.Join("/pvc", path), url)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-download-genesis", h.pvc.GetName()),
			Namespace: h.pvc.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](0),
			PriorityClassName:             pc,
			Affinity:                      af,
			NodeSelector:                  ns,
			Volumes: []corev1.Volume{
				{
					Name: "pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: h.pvc.GetName(),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "downloader",
					Image:   "apteno/alpine-jq",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", cmd},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "pvc",
							MountPath: "/pvc",
						},
					},
				},
			},
		},
	}

	ph := NewPodHelper(h.client, h.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer func() { _ = ph.Delete(ctx) }()

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return err
	}

	return ph.WaitForPodSucceeded(ctx, time.Hour)
}
