package chainnode

import (
	"context"
	"strconv"

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
)

func (r *Reconciler) ensureService(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	// Prepare service spec
	svc, err := r.getServiceSpec(chainNode)
	if err != nil {
		return err
	}

	currentSvc := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), currentSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating service")
			if err := r.Create(ctx, svc); err != nil {
				return err
			}
			chainNode.Status.IP = svc.Spec.ClusterIP
			return r.Status().Update(ctx, chainNode)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentSvc, svc)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		logger.Info("updating service")

		svc.ObjectMeta.ResourceVersion = currentSvc.ObjectMeta.ResourceVersion
		svc.Spec.ClusterIP = currentSvc.Spec.ClusterIP

		if err := r.Update(ctx, svc); err != nil {
			return err
		}
	}

	if chainNode.Status.IP != currentSvc.Spec.ClusterIP {
		chainNode.Status.IP = currentSvc.Spec.ClusterIP
		return r.Status().Update(ctx, chainNode)
	}

	return nil
}

func (r *Reconciler) getServiceSpec(chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.GetName(),
			Namespace: chainNode.GetNamespace(),
			Labels: map[string]string{
				labelNodeID:    chainNode.Status.NodeID,
				labelChainID:   chainNode.Status.ChainID,
				labelValidator: strconv.FormatBool(chainNode.IsValidator()),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.P2pPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.P2pPort,
					TargetPort: intstr.FromInt(chainutils.P2pPort),
				},
				{
					Name:       chainutils.RpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.Rpcport,
					TargetPort: intstr.FromInt(chainutils.Rpcport),
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
				labelNodeID:  chainNode.Status.NodeID,
				labelChainID: chainNode.Status.ChainID,
			},
		},
	}
	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}
