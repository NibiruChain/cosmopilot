package framework

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/voluzi/cosmopilot/v2/internal/k8s"
)

const (
	// RandomNamespacePrefix is the prefix for randomly generated namespace names
	RandomNamespacePrefix = "cosmopilot-test-"
)

// BaseFramework contains common functionality for all framework implementations
type BaseFramework struct {
	ctx        context.Context
	cancel     context.CancelFunc
	cfg        *Config
	restCfg    *rest.Config
	client     client.Client
	kubeClient *kubernetes.Clientset
}

// Context returns the framework context
func (b *BaseFramework) Context() context.Context {
	return b.ctx
}

// Client returns the controller-runtime client
func (b *BaseFramework) Client() client.Client {
	return b.client
}

// KubeClient returns the kubernetes clientset
func (b *BaseFramework) KubeClient() *kubernetes.Clientset {
	return b.kubeClient
}

// RestConfig returns the REST config
func (b *BaseFramework) RestConfig() *rest.Config {
	return b.restCfg
}

// Config returns the framework configuration
func (b *BaseFramework) Config() *Config {
	return b.cfg
}

// CreateRandomNamespace creates a namespace with a random name
func (b *BaseFramework) CreateRandomNamespace() (*corev1.Namespace, error) {
	return b.kubeClient.CoreV1().Namespaces().Create(b.ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: RandomNamespacePrefix,
		},
	}, metav1.CreateOptions{})
}

// DeleteNamespace deletes the given namespace
func (b *BaseFramework) DeleteNamespace(ns *corev1.Namespace) error {
	// Small delay to let controllers finish any in-progress reconciliation
	// This reduces "namespace is being terminated" errors during test cleanup
	time.Sleep(100 * time.Millisecond)
	return b.kubeClient.CoreV1().Namespaces().Delete(b.ctx, ns.Name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

// SetContext sets the context and cancel function
func (b *BaseFramework) SetContext(ctx context.Context, cancel context.CancelFunc) {
	b.ctx = ctx
	b.cancel = cancel
}

// SetConfig sets the configuration
func (b *BaseFramework) SetConfig(cfg *Config) {
	b.cfg = cfg
}

// SetRestConfig sets the REST config
func (b *BaseFramework) SetRestConfig(cfg *rest.Config) {
	b.restCfg = cfg
}

// SetClient sets the controller-runtime client
func (b *BaseFramework) SetClient(c client.Client) {
	b.client = c
}

// SetKubeClient sets the kubernetes clientset
func (b *BaseFramework) SetKubeClient(c *kubernetes.Clientset) {
	b.kubeClient = c
}

// Cancel cancels the context
func (b *BaseFramework) Cancel() {
	if b.cancel != nil {
		b.cancel()
	}
}

// PodExec executes a command in a pod container and returns stdout
func (b *BaseFramework) PodExec(namespace, podName, container string, command ...string) (string, error) {
	pod, err := b.kubeClient.CoreV1().Pods(namespace).Get(b.ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	helper := k8s.NewPodHelper(b.kubeClient, b.restCfg, pod)
	stdout, stderr, err := helper.Exec(b.ctx, container, command)
	if err != nil {
		return "", fmt.Errorf("exec failed: %w, stderr: %s", err, stderr)
	}
	return stdout, nil
}

// RunAppCommand creates a temporary pod with the account secret mounted and runs a command.
// This is useful for running app CLI commands that require the validator account (e.g., submitting transactions).
// The mnemonic is imported into the keyring before running the command using the app binary only (no shell required).
func (b *BaseFramework) RunAppCommand(namespace, image, appBinary, accountSecretName string, args []string) (string, error) {
	podName := fmt.Sprintf("app-cmd-%d", time.Now().UnixNano())

	// Get the mnemonic from the secret
	secret, err := b.kubeClient.CoreV1().Secrets(namespace).Get(b.ctx, accountSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get account secret: %w", err)
	}
	mnemonic := string(secret.Data["mnemonic"])

	dataVolumeMount := corev1.VolumeMount{
		Name:      "data",
		MountPath: "/home/app",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: dataVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "load-account",
					Image:   image,
					Command: []string{appBinary},
					Args: []string{
						"keys", "add", "account", "--recover",
						"--keyring-backend", "test",
						"--home", "/home/app",
					},
					Stdin:        true,
					StdinOnce:    true,
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
			},
			Containers: []corev1.Container{
				{
					Name:         "run-command",
					Image:        image,
					Command:      []string{appBinary},
					Args:         args,
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
			},
		},
	}

	// Create the pod
	_, err = b.kubeClient.CoreV1().Pods(namespace).Create(b.ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %w", err)
	}

	// Ensure cleanup
	defer func() {
		_ = b.kubeClient.CoreV1().Pods(namespace).Delete(b.ctx, podName, metav1.DeleteOptions{
			GracePeriodSeconds: ptr.To[int64](0),
		})
	}()

	// Use PodHelper to handle init container stdin and wait for completion
	ph := k8s.NewPodHelper(b.kubeClient, b.restCfg, pod)

	// Wait for load-account init container to be running
	if err := ph.WaitForInitContainerRunning(b.ctx, "load-account", time.Minute); err != nil {
		return "", fmt.Errorf("failed waiting for load-account container: %w", err)
	}

	// Attach to load-account container to provide mnemonic via stdin
	var input bytes.Buffer
	input.WriteString(mnemonic + "\n")
	if _, _, err := ph.Attach(b.ctx, "load-account", &input); err != nil {
		return "", fmt.Errorf("failed to attach to load-account container: %w", err)
	}

	// Wait for pod to complete
	if err := ph.WaitForPodSucceeded(b.ctx, 5*time.Minute); err != nil {
		// Get logs for error message
		logs, _ := b.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: "run-command",
		}).Do(b.ctx).Raw()
		return "", fmt.Errorf("pod failed: %w, logs: %s", err, string(logs))
	}

	// Get logs from the run-command container
	logs, err := b.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "run-command",
	}).Do(b.ctx).Raw()
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs: %w", err)
	}
	return string(logs), nil
}
