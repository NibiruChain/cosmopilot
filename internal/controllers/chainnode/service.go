package chainnode

import (
	"context"
	"fmt"
	"reflect"
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
	"github.com/NibiruChain/nibiru-operator/internal/controllers"
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

func (r *Reconciler) ensureServices(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

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
		logger.Info("updating .status.IP", "IP", svc.Spec.ClusterIP)
		chainNode.Status.IP = svc.Spec.ClusterIP
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
	}

	// Ensure internal service
	internal, err := r.getInternalServiceSpec(ctx, chainNode)
	if err != nil {
		return err
	}

	if err := r.ensureService(ctx, internal); err != nil {
		return err
	}

	// Ensure P2P service if enabled
	p2p, err := r.getP2pServiceSpec(chainNode)
	if err != nil {
		return err
	}
	if chainNode.Spec.Expose.Enabled() {
		if err := r.ensureService(ctx, p2p); err != nil {
			return err
		}

		// Get External IP address
		var externalAddress string
		sh := k8s.NewServiceHelper(r.ClientSet, r.RestConfig, p2p)

		switch chainNode.Spec.Expose.GetServiceType() {
		case corev1.ServiceTypeNodePort:
			// Wait for NodePort to be available
			logger.V(1).Info("waiting for nodePort address to be available", "svc", p2p.GetName())
			if err := sh.WaitForCondition(ctx, func(svc *corev1.Service) (bool, error) {
				return svc.Spec.Ports[0].NodePort > 0, nil
			}, timeoutWaitServiceIP); err != nil {
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
			logger.V(1).Info("waiting for load balancer address to be available", "svc", p2p.GetName())
			if err := sh.WaitForCondition(ctx, func(svc *corev1.Service) (bool, error) {
				return svc.Status.LoadBalancer.Ingress != nil && len(svc.Status.LoadBalancer.Ingress) > 0, nil
			}, timeoutWaitServiceIP); err != nil {
				return err
			}
			externalAddress = fmt.Sprintf("%s@%s:%d", chainNode.Status.NodeID, p2p.Status.LoadBalancer.Ingress[0].IP, chainutils.P2pPort)
		}

		if chainNode.Status.PublicAddress != externalAddress {
			logger.Info("updating .status.publicAddress", "address", externalAddress)
			chainNode.Status.PublicAddress = externalAddress
			return r.Status().Update(ctx, chainNode)
		}

	} else {
		// Delete the service if it exists
		if err := r.Delete(ctx, p2p); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		} else {
			logger.Info("deleted service", "svc", p2p.GetName())
		}
	}

	return nil
}

func (r *Reconciler) ensureService(ctx context.Context, svc *corev1.Service) error {
	logger := log.FromContext(ctx).WithValues("svc", svc.GetName())

	currentSvc := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(svc), currentSvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating service")
			return r.Create(ctx, svc)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentSvc, svc)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() || !reflect.DeepEqual(currentSvc.Annotations, svc.Annotations) {
		logger.Info("updating service")
		svc.ObjectMeta.ResourceVersion = currentSvc.ObjectMeta.ResourceVersion
		return r.Update(ctx, svc)
	}

	*svc = *currentSvc
	return nil
}

func (r *Reconciler) getServiceSpec(chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.GetName(),
			Namespace: chainNode.GetNamespace(),
			Labels:    WithChainNodeLabels(chainNode),
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
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				LabelNodeID:  chainNode.Status.NodeID,
				LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Firewall.Enabled() {
		svc.Spec.Ports[1].TargetPort = intstr.FromInt(controllers.FirewallRpcPort)
		svc.Spec.Ports[2].TargetPort = intstr.FromInt(controllers.FirewallLcdPort)
		svc.Spec.Ports[3].TargetPort = intstr.FromInt(controllers.FirewallGrpcPort)
	}

	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) getInternalServiceSpec(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-internal", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				LabelNodeID:    chainNode.Status.NodeID,
				LabelChainID:   chainNode.Status.ChainID,
				LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
			}),
		},
		Spec: corev1.ServiceSpec{
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
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				LabelNodeID:  chainNode.Status.NodeID,
				LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}

	if err := r.maybeAddStateSyncAnnotations(ctx, chainNode, svc); err != nil {
		r.recorder.Event(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonNoTrustHeight,
			fmt.Sprintf("not adding state-sync details: %v", err),
		)
	}

	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) getP2pServiceSpec(chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-p2p", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
			Labels:    WithChainNodeLabels(chainNode),
		},
		Spec: corev1.ServiceSpec{
			Type:                     chainNode.Spec.Expose.GetServiceType(),
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.P2pPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.P2pPort,
					TargetPort: intstr.FromInt(chainutils.P2pPort),
				},
			},
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				LabelNodeID:  chainNode.Status.NodeID,
				LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}
	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) maybeAddStateSyncAnnotations(ctx context.Context, chainNode *appsv1.ChainNode, svc *corev1.Service) error {
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.StateSync.Enabled() &&
		chainNode.Status.LatestHeight > int64(chainNode.Spec.Config.StateSync.SnapshotInterval*3) {
		c, err := r.getClient(chainNode)
		if err != nil {
			return err
		}

		snapshotInterval := chainNode.Spec.Config.StateSync.SnapshotInterval
		trustHeight := (chainNode.Status.LatestHeight / int64(snapshotInterval) * int64(snapshotInterval)) - (int64(snapshotInterval) * 3)

		if trustHeight > 0 {
			trustHash, err := c.GetBlockHash(ctx, trustHeight)
			if err != nil {
				return err
			}

			if svc.Annotations == nil {
				svc.Annotations = make(map[string]string)
			}
			svc.Annotations[AnnotationStateSyncTrustHeight] = strconv.FormatInt(trustHeight, 10)
			svc.Annotations[AnnotationStateSyncTrustHash] = trustHash
		}
	}
	return nil
}
