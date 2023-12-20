package chainnodeset

import (
	"context"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/controllers"
)

func (r *Reconciler) ensureServices(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	for _, group := range nodeSet.Spec.Nodes {
		svc, err := r.getServiceSpec(nodeSet, group)
		if err != nil {
			return err
		}
		if err := r.ensureService(ctx, svc); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) ensureService(ctx context.Context, svc *corev1.Service) error {
	logger := log.FromContext(ctx)

	currentSvc := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(svc), currentSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating service", "svc", svc.GetName())
			return r.Create(ctx, svc)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentSvc, svc)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		logger.Info("updating service", "svc", svc.GetName())

		svc.ObjectMeta.ResourceVersion = currentSvc.ObjectMeta.ResourceVersion
		if err := r.Update(ctx, svc); err != nil {
			return err
		}
	}

	*svc = *currentSvc
	return nil
}

func (r *Reconciler) getServiceSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      group.GetServiceName(nodeSet),
			Namespace: nodeSet.GetNamespace(),
			Labels:    WithChainNodeSetLabels(nodeSet),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.RpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.RpcPort,
					TargetPort: intstr.FromInt(chainutils.RpcPort),
				},
				{
					Name:       chainutils.LcdPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.LcdPort,
					TargetPort: intstr.FromInt(chainutils.LcdPort),
				},
				{
					Name:       chainutils.GrpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.GrpcPort,
					TargetPort: intstr.FromInt(chainutils.GrpcPort),
				},
			},
			Selector: map[string]string{
				LabelChainNodeSet:      nodeSet.GetName(),
				LabelChainNodeSetGroup: group.Name,
			},
		},
	}

	if group.Config != nil && group.Config.Firewall.Enabled() {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(controllers.FirewallRpcPort)
		svc.Spec.Ports[1].TargetPort = intstr.FromInt(controllers.FirewallLcdPort)
		svc.Spec.Ports[2].TargetPort = intstr.FromInt(controllers.FirewallGrpcPort)
	}

	return svc, controllerutil.SetControllerReference(nodeSet, svc, r.Scheme)
}
