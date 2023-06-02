package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/tools/watch"
)

type PodHelper struct {
	client     *kubernetes.Clientset
	restConfig *rest.Config
	pod        *corev1.Pod
}

func NewPodHelper(client *kubernetes.Clientset, cfg *rest.Config, pod *corev1.Pod) *PodHelper {
	return &PodHelper{
		client:     client,
		restConfig: cfg,
		pod:        pod,
	}
}

func (p *PodHelper) Create(ctx context.Context) error {
	var err error
	p.pod, err = p.client.CoreV1().Pods(p.pod.GetNamespace()).Create(ctx, p.pod, metav1.CreateOptions{})
	return err
}

func (p *PodHelper) Delete(ctx context.Context) error {
	return p.client.CoreV1().Pods(p.pod.GetNamespace()).Delete(ctx, p.pod.GetName(), metav1.DeleteOptions{})
}

func (p *PodHelper) WaitForPodRunning(ctx context.Context, timeout time.Duration) error {
	return p.WaitForPodCondition(ctx, timeout, corev1.PodRunning)
}

func (p *PodHelper) WaitForPodSucceeded(ctx context.Context, timeout time.Duration) error {
	return p.WaitForPodCondition(ctx, timeout, corev1.PodSucceeded)
}

func (p *PodHelper) WaitForPodCondition(ctx context.Context, timeout time.Duration, phase corev1.PodPhase) error {
	fs := fields.SelectorFromSet(map[string]string{
		"metadata.namespace": p.pod.Namespace,
		"metadata.name":      p.pod.Name,
	})

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return p.client.CoreV1().Pods(p.pod.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return p.client.CoreV1().Pods(p.pod.Namespace).Watch(ctx, options)
		},
	}

	ctx, cfn := context.WithTimeout(ctx, timeout)
	defer cfn()

	last, err := watch.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watchapi.Event) (bool, error) {
		switch event.Type {
		case watchapi.Error:
			return false, fmt.Errorf("error watching pod")

		case watchapi.Deleted:
			return false, fmt.Errorf("pod %s/%s was deleted", p.pod.Namespace, p.pod.Name)

		default:
			p.pod = event.Object.(*corev1.Pod)
			if p.pod.Status.Phase == corev1.PodFailed {
				return false, fmt.Errorf("pod failed")
			}
			return p.pod.Status.Phase == phase, nil
		}
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for pod %s/%s", p.pod.Namespace, p.pod.Name)
	}
	return nil
}

func (p *PodHelper) WaitForInitContainerRunning(ctx context.Context, container string, timeout time.Duration) error {
	return p.WaitForInitContainerCondition(ctx, timeout, container, corev1.PodRunning)
}

func (p *PodHelper) WaitForInitContainerCondition(ctx context.Context, timeout time.Duration, container string, phase corev1.PodPhase) error {
	fs := fields.SelectorFromSet(map[string]string{
		"metadata.namespace": p.pod.Namespace,
		"metadata.name":      p.pod.Name,
	})

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return p.client.CoreV1().Pods(p.pod.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return p.client.CoreV1().Pods(p.pod.Namespace).Watch(ctx, options)
		},
	}

	ctx, cfn := context.WithTimeout(ctx, timeout)
	defer cfn()

	last, err := watch.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watchapi.Event) (bool, error) {
		switch event.Type {
		case watchapi.Error:
			return false, fmt.Errorf("error watching pod")

		case watchapi.Deleted:
			return false, fmt.Errorf("pod %s/%s was deleted", p.pod.Namespace, p.pod.Name)

		default:
			p.pod = event.Object.(*corev1.Pod)
			if p.pod.Status.Phase == corev1.PodFailed {
				return false, fmt.Errorf("pod failed")
			}
			for _, c := range p.pod.Status.InitContainerStatuses {
				if c.Name == container {
					return c.State.Running != nil, nil
				}
			}
			return false, nil
		}
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for pod %s/%s", p.pod.Namespace, p.pod.Name)
	}
	return nil
}

func (p *PodHelper) Exec(ctx context.Context, container string, cmd []string) (string, string, error) {
	req := p.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(p.pod.Name).
		Namespace(p.pod.Namespace).
		SubResource("exec")

	req.VersionedParams(
		&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
		},
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(p.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error executing command on pod: %v", err)
	}

	var execOut bytes.Buffer
	var execErr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
	})
	if err != nil {
		return "", "", fmt.Errorf("error executing command on pod: %s: %v", execErr.String(), err)
	}
	return execOut.String(), execErr.String(), nil
}

func (p *PodHelper) Attach(ctx context.Context, container string, stdin io.Reader) (string, string, error) {
	req := p.client.CoreV1().RESTClient().Post().Resource("pods").Name(p.pod.Name).Namespace(p.pod.Namespace).SubResource("attach")
	req.VersionedParams(
		&corev1.PodAttachOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: container,
		},
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(p.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var execOut, execErr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &execOut,
		Stderr: &execErr,
	})

	return execOut.String(), execErr.String(), err
}
