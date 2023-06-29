package chainnode

import (
	"context"
	"fmt"
	"strconv"
	"time"

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
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

func (r *Reconciler) ensureServices(ctx context.Context, chainNode *appsv1.ChainNode) error {
	// Ensure main service
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

	if err := r.ensureService(ctx, headless); err != nil {
		return err
	}

	// Ensure P2P service if enabled
	p2p, err := r.getP2pServiceSpec(chainNode)
	if err != nil {
		return err
	}
	if chainNode.ExposesP2P() {
		if err := r.ensureService(ctx, p2p); err != nil {
			return err
		}

		// Get External IP address
		var externalAddress string
		sh := k8s.NewServiceHelper(r.ClientSet, r.RestConfig, p2p)

		switch chainNode.GetP2pServiceType() {
		case corev1.ServiceTypeNodePort:
			// Wait for NodePort to be available
			if err := sh.WaitForCondition(ctx, func(svc *corev1.Service) (bool, error) {
				return svc.Spec.Ports[0].NodePort > 0, nil
			}, time.Minute); err != nil {
				return err
			}
			port := p2p.Spec.Ports[0].NodePort

			// TODO: maybe get IP address from the node hosting the pod
			// get a public address from one of the nodes
			nodes, err := r.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			availableIPs := make([]string, 0)
			for _, node := range nodes.Items {
				for _, address := range node.Status.Addresses {
					if address.Type == corev1.NodeExternalIP {
						availableIPs = append(availableIPs, address.Address)
					}
				}
			}

			if len(availableIPs) > 0 {
				externalAddress = fmt.Sprintf("%s@%s:%d", chainNode.Status.NodeID, availableIPs[0], port)
			} else {
				externalAddress = fmt.Sprintf("%s@<NODE_ADDRESS>:%d", chainNode.Status.NodeID, port)
			}

		case corev1.ServiceTypeLoadBalancer:
			// Wait for LoadBalancer to be available
			if err := sh.WaitForCondition(ctx, func(svc *corev1.Service) (bool, error) {
				return svc.Status.LoadBalancer.Ingress != nil && len(svc.Status.LoadBalancer.Ingress) > 0, nil
			}, time.Minute); err != nil {
				return err
			}
			externalAddress = fmt.Sprintf("%s@%s:%d", chainNode.Status.NodeID, p2p.Status.LoadBalancer.Ingress[0].IP, chainutils.P2pPort)
		}

		if chainNode.Status.PublicAddress != externalAddress {
			chainNode.Status.PublicAddress = externalAddress
			return r.Status().Update(ctx, chainNode)
		}

	} else {
		// Delete the service if it exists
		if err := r.Delete(ctx, p2p); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
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
			Labels: utils.MergeMaps(
				map[string]string{
					LabelNodeID:    chainNode.Status.NodeID,
					LabelChainID:   chainNode.Status.ChainID,
					LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
				},
				chainNode.Labels),
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
					Name:       chainutils.PrometheusPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.PrometheusPort,
					TargetPort: intstr.FromInt(chainutils.PrometheusPort),
				},
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt(nodeUtilsPort),
				},
			},
			Selector: utils.MergeMaps(
				map[string]string{
					LabelNodeID:  chainNode.Status.NodeID,
					LabelChainID: chainNode.Status.ChainID,
				},
				chainNode.Labels),
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
					Name:       chainutils.PrometheusPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.PrometheusPort,
					TargetPort: intstr.FromInt(chainutils.PrometheusPort),
				},
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt(nodeUtilsPort),
				},
			},
			Selector: utils.MergeMaps(
				map[string]string{
					LabelNodeID:  chainNode.Status.NodeID,
					LabelChainID: chainNode.Status.ChainID,
				},
				chainNode.Labels),
		},
	}
	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) getP2pServiceSpec(chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-p2p", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Type:                     chainNode.GetP2pServiceType(),
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.P2pPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.P2pPort,
					TargetPort: intstr.FromInt(chainutils.P2pPort),
				},
			},
			Selector: utils.MergeMaps(
				map[string]string{
					LabelNodeID:  chainNode.Status.NodeID,
					LabelChainID: chainNode.Status.ChainID,
				},
				chainNode.Labels),
		},
	}
	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}
