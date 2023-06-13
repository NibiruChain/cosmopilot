package chainnode

import (
	"context"
	"fmt"
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

func (r *Reconciler) ensureServices(ctx context.Context, chainNode *appsv1.ChainNode) error {
	// Ensure service
	svc, err := r.getServiceSpec(chainNode)
	if err != nil {
		return err
	}

	if err := r.ensureService(ctx, svc); err != nil {
		return err
	}

	// Update ChainNode IP address
	if chainNode.Status.IP != svc.Spec.ClusterIP {
		chainNode.Status.IP = svc.Spec.ClusterIP
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
	}

	// Ensure headless service
	headless, err := r.getHeadlessServiceSpec(chainNode)
	if err != nil {
		return err
	}

	return r.ensureService(ctx, headless)
}

func (r *Reconciler) ensureService(ctx context.Context, svc *corev1.Service) error {
	logger := log.FromContext(ctx)

	currentSvc := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(svc), currentSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating service", "name", svc.GetName())
			return r.Create(ctx, svc)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentSvc, svc)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		logger.Info("updating service", "name", svc.GetName())

		svc.ObjectMeta.ResourceVersion = currentSvc.ObjectMeta.ResourceVersion
		if err := r.Update(ctx, svc); err != nil {
			return err
		}
	}

	*svc = *currentSvc
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
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt(nodeUtilsPort),
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

func (r *Reconciler) getHeadlessServiceSpec(chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-headless", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
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
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt(nodeUtilsPort),
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
