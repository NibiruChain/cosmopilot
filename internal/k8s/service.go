package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
)

type ServiceHelper struct {
	client     *kubernetes.Clientset
	restConfig *rest.Config
	svc        *corev1.Service
}

func NewServiceHelper(client *kubernetes.Clientset, cfg *rest.Config, svc *corev1.Service) *ServiceHelper {
	return &ServiceHelper{
		client:     client,
		restConfig: cfg,
		svc:        svc,
	}
}

func (s *ServiceHelper) WaitForCondition(ctx context.Context, fn func(service *corev1.Service) (bool, error), timeout time.Duration) error {
	fs := fields.SelectorFromSet(map[string]string{
		"metadata.namespace": s.svc.Namespace,
		"metadata.name":      s.svc.Name,
	})

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return s.client.CoreV1().Services(s.svc.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return s.client.CoreV1().Services(s.svc.Namespace).Watch(ctx, options)
		},
	}

	ctx, cfn := context.WithTimeout(ctx, timeout)
	defer cfn()

	last, err := watch.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watchapi.Event) (bool, error) {
		switch event.Type {
		case watchapi.Error:
			return false, fmt.Errorf("error watching service")

		case watchapi.Deleted:
			return false, fmt.Errorf("service %s/%s was deleted", s.svc.Namespace, s.svc.Name)

		default:
			s.svc = event.Object.(*corev1.Service)
			return fn(s.svc)
		}
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for pod %s/%s", s.svc.Namespace, s.svc.Name)
	}
	return nil
}
