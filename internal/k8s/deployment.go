package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
)

type DeploymentHelper struct {
	client     *kubernetes.Clientset
	restConfig *rest.Config
	deployment *appsv1.Deployment
}

func NewDeploymentHelper(client *kubernetes.Clientset, cfg *rest.Config, svc *appsv1.Deployment) *DeploymentHelper {
	return &DeploymentHelper{
		client:     client,
		restConfig: cfg,
		deployment: svc,
	}
}

func (s *DeploymentHelper) WaitForCondition(ctx context.Context, fn func(*appsv1.Deployment) (bool, error), timeout time.Duration) error {
	fs := fields.SelectorFromSet(map[string]string{
		"metadata.namespace": s.deployment.Namespace,
		"metadata.name":      s.deployment.Name,
	})

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return s.client.AppsV1().Deployments(s.deployment.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return s.client.AppsV1().Deployments(s.deployment.Namespace).Watch(ctx, options)
		},
	}

	ctx, cfn := context.WithTimeout(ctx, timeout)
	defer cfn()

	last, err := watch.UntilWithSync(ctx, lw, &appsv1.Deployment{}, nil, func(event watchapi.Event) (bool, error) {
		switch event.Type {
		case watchapi.Error:
			return false, fmt.Errorf("error watching deployment")

		case watchapi.Deleted:
			return false, fmt.Errorf("deployment %s/%s was deleted", s.deployment.Namespace, s.deployment.Name)

		default:
			s.deployment = event.Object.(*appsv1.Deployment)
			return fn(s.deployment)
		}
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for deployment %s/%s", s.deployment.Namespace, s.deployment.Name)
	}
	return nil
}
