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

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/internal/k8s"
	"github.com/NibiruChain/cosmopilot/pkg/nodeutils"
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
			logger.V(1).Info("waiting for nodePort to be available", "svc", p2p.GetName())
			if err := sh.WaitForCondition(ctx, func(svc *corev1.Service) (bool, error) {
				return svc.Spec.Ports[0].NodePort > 0, nil
			}, timeoutWaitServiceIP); err != nil {
				return err
			}
			port := p2p.Spec.Ports[0].NodePort

			var node *corev1.Node
			pod, err := r.getChainNodePod(ctx, chainNode)
			if err != nil {
				return err
			}
			if pod != nil {
				if pod.Spec.NodeName != "" {
					node, err = r.ClientSet.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
					if err != nil {
						return err
					}
				}
			}

			// pick any node if we could not retrieve pod's node
			if node == nil {
				nodes, err := r.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				if err != nil {
					return err
				}
				if len(nodes.Items) > 0 {
					node = &nodes.Items[0]
				}
			}

			if node == nil {
				return fmt.Errorf("no node found")
			}

			var address string
			addressPriority := []corev1.NodeAddressType{
				corev1.NodeExternalIP,
				corev1.NodeExternalDNS,
				corev1.NodeInternalIP,
				corev1.NodeInternalDNS,
				corev1.NodeHostName,
			}

			for _, addrType := range addressPriority {
				for _, addr := range node.Status.Addresses {
					if addr.Type == addrType {
						address = addr.Address
						break
					}
				}
				if address != "" {
					break
				}
			}

			if address == "" {
				return fmt.Errorf("no address found for nodeport")
			}

			externalAddress = fmt.Sprintf("%s@%s:%d", chainNode.Status.NodeID, address, port)

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
					TargetPort: intstr.FromInt32(chainutils.P2pPort),
				},
				{
					Name:       chainutils.RpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.RpcPort,
					TargetPort: intstr.FromInt32(chainutils.RpcPort),
				},
				{
					Name:       chainutils.LcdPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.LcdPort,
					TargetPort: intstr.FromInt32(chainutils.LcdPort),
				},
				{
					Name:       chainutils.GrpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.GrpcPort,
					TargetPort: intstr.FromInt32(chainutils.GrpcPort),
				},
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt32(nodeUtilsPort),
				},
				{
					Name:       chainutils.PrometheusPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.PrometheusPort,
					TargetPort: intstr.FromInt32(chainutils.PrometheusPort),
				},
			},
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelNodeID:  chainNode.Status.NodeID,
				controllers.LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}

	if chainNode.Spec.Config.IsEvmEnabled() {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.EvmRpcPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.EvmRpcPort,
			TargetPort: intstr.FromInt32(controllers.EvmRpcPort),
		})
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.EvmRpcWsPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.EvmRpcWsPort,
			TargetPort: intstr.FromInt32(controllers.EvmRpcWsPort),
		})
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.CosmoGuardEnabled() {
		svc.Spec.Ports[1].TargetPort = intstr.FromInt32(controllers.CosmoGuardRpcPort)
		svc.Spec.Ports[2].TargetPort = intstr.FromInt32(controllers.CosmoGuardLcdPort)
		svc.Spec.Ports[3].TargetPort = intstr.FromInt32(controllers.CosmoGuardGrpcPort)
		if chainNode.Spec.Config.IsEvmEnabled() {
			svc.Spec.Ports[5].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcPort)
			svc.Spec.Ports[6].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcWsPort)
		}
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.CosmoGuardMetricsPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.CosmoGuardMetricsPort,
			TargetPort: intstr.FromInt32(controllers.CosmoGuardMetricsPort),
		})
	}

	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) getInternalServiceSpec(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-internal", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelPeer:      controllers.StringValueTrue,
				controllers.LabelSeed:      controllers.StringValueFalse,
				controllers.LabelNodeID:    chainNode.Status.NodeID,
				controllers.LabelChainID:   chainNode.Status.ChainID,
				controllers.LabelValidator: strconv.FormatBool(chainNode.IsValidator()),
			}),
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.P2pPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.P2pPort,
					TargetPort: intstr.FromInt32(chainutils.P2pPort),
				},
				{
					Name:       chainutils.RpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.RpcPort,
					TargetPort: intstr.FromInt32(chainutils.RpcPort),
				},
				{
					Name:       chainutils.LcdPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.LcdPort,
					TargetPort: intstr.FromInt32(chainutils.LcdPort),
				},
				{
					Name:       chainutils.GrpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.GrpcPort,
					TargetPort: intstr.FromInt32(chainutils.GrpcPort),
				},
				{
					Name:       chainutils.PrometheusPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.PrometheusPort,
					TargetPort: intstr.FromInt32(chainutils.PrometheusPort),
				},
				{
					Name:       nodeUtilsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       nodeUtilsPort,
					TargetPort: intstr.FromInt32(nodeUtilsPort),
				},
			},
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelNodeID:  chainNode.Status.NodeID,
				controllers.LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}

	if chainNode.Spec.Config.IsEvmEnabled() {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.EvmRpcPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.EvmRpcPort,
			TargetPort: intstr.FromInt32(controllers.EvmRpcPort),
		})
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.EvmRpcWsPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.EvmRpcWsPort,
			TargetPort: intstr.FromInt32(controllers.EvmRpcWsPort),
		})
	}

	if chainNode.Spec.Config != nil && chainNode.Spec.Config.CosmoGuardEnabled() {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:       controllers.CosmoGuardMetricsPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       controllers.CosmoGuardMetricsPort,
			TargetPort: intstr.FromInt32(controllers.CosmoGuardMetricsPort),
		})
	}

	if err := r.addStateSyncAnnotations(ctx, chainNode, svc); err != nil {
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
			Name:        fmt.Sprintf("%s-p2p", chainNode.GetName()),
			Namespace:   chainNode.GetNamespace(),
			Labels:      WithChainNodeLabels(chainNode),
			Annotations: chainNode.Spec.Expose.GetAnnotations(),
		},
		Spec: corev1.ServiceSpec{
			Type:                     chainNode.Spec.Expose.GetServiceType(),
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{
					Name:       chainutils.P2pPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       chainutils.P2pPort,
					TargetPort: intstr.FromInt32(chainutils.P2pPort),
				},
			},
			Selector: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelNodeID:  chainNode.Status.NodeID,
				controllers.LabelChainID: chainNode.Status.ChainID,
			}),
		},
	}
	return svc, controllerutil.SetControllerReference(chainNode, svc, r.Scheme)
}

func (r *Reconciler) addStateSyncAnnotations(ctx context.Context, chainNode *appsv1.ChainNode, svc *corev1.Service) error {
	logger := log.FromContext(ctx)

	availableHeights, err := nodeutils.NewClient(chainNode.GetNodeFQDN()).ListSnapshots()
	if err != nil {
		return err
	}

	if len(availableHeights) == 0 {
		return fmt.Errorf("no state-sync snapshots available")
	}

	c, err := r.getChainNodeClient(chainNode)
	if err != nil {
		return err
	}

	// Get the most recent height with a retrievable block hash
	var trustHeight int64
	var trustHash string
	for i := len(availableHeights) - 1; i >= 0; i-- {
		height := availableHeights[i]
		if trustHash, err = c.GetBlockHash(ctx, height); err == nil {
			trustHeight = height
			break
		}
		logger.Info("state-sync: ignoring snapshot height", "height", height, "error", err)
	}

	if trustHeight == 0 {
		return fmt.Errorf("no valid state-sync snapshot heights with retrievable block hashes")
	}

	if lastUpgradeHeight := chainNode.GetLastUpgradeHeight(); lastUpgradeHeight > trustHeight {
		trustHeight = lastUpgradeHeight + 1
		trustHash, err = c.GetBlockHash(ctx, trustHeight)
		if err != nil {
			return err
		}
		logger.Info("adjusting trust height due to upgrade", "newTrustHeight", trustHeight)
	}

	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[controllers.AnnotationStateSyncTrustHeight] = strconv.FormatInt(trustHeight, 10)
	svc.Annotations[controllers.AnnotationStateSyncTrustHash] = trustHash

	return nil
}
