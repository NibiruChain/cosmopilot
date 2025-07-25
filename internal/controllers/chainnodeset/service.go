package chainnodeset

import (
	"context"
	"fmt"
	"strconv"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func (r *Reconciler) ensureServices(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	for _, group := range nodeSet.Spec.Nodes {
		svc, err := r.getServiceSpec(nodeSet, group)
		if err != nil {
			return err
		}
		if err = r.ensureService(ctx, svc); err != nil {
			return err
		}

		svc, err = r.getInternalServiceSpec(nodeSet, group)
		if err != nil {
			return err
		}
		if err = r.ensureService(ctx, svc); err != nil {
			return err
		}
	}

	for _, ingress := range nodeSet.Spec.Ingresses {
		svc, err := r.getGlobalServiceSpec(nodeSet, ingress)
		if err != nil {
			return err
		}
		if err = r.ensureService(ctx, svc); err != nil {
			return err
		}

		svc, err = r.getGlobalInternalServiceSpec(nodeSet, ingress)
		if err != nil {
			return err
		}
		if err = r.ensureService(ctx, svc); err != nil {
			return err
		}
	}

	// Clean up if necessary
	groupServices, err := r.listChainNodeSetServices(ctx, nodeSet, controllers.LabelScope, scopeGroup)
	if err != nil {
		return err
	}

	for _, svc := range groupServices.Items {
		if _, ok := svc.Labels[controllers.LabelChainNodeSetGroup]; !ok ||
			!ContainsGroup(nodeSet.Spec.Nodes, svc.Labels[controllers.LabelChainNodeSetGroup]) {
			logger.Info("deleting service", "svc", svc.GetName())
			if err = r.Delete(ctx, &svc); err != nil {
				return err
			}
		}
	}

	globalServices, err := r.listChainNodeSetServices(ctx, nodeSet, controllers.LabelScope, scopeGlobal)
	if err != nil {
		return err
	}

	for _, svc := range globalServices.Items {
		if _, ok := svc.Labels[controllers.LabelGlobalIngress]; !ok ||
			!ContainsGlobalIngress(nodeSet.Spec.Ingresses, svc.Labels[controllers.LabelGlobalIngress]) {
			logger.Info("deleting service", "svc", svc.GetName())
			if err = r.Delete(ctx, &svc); err != nil {
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
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
				controllers.LabelScope:             scopeGroup,
			}),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
			},
			Selector: map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
			},
		},
	}

	if group.Config.IsEvmEnabled() {
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

	if group.Config != nil && group.Config.CosmoGuardEnabled() {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt32(controllers.CosmoGuardRpcPort)
		svc.Spec.Ports[1].TargetPort = intstr.FromInt32(controllers.CosmoGuardLcdPort)
		svc.Spec.Ports[2].TargetPort = intstr.FromInt32(controllers.CosmoGuardGrpcPort)
		if group.Config.IsEvmEnabled() {
			svc.Spec.Ports[3].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcPort)
			svc.Spec.Ports[4].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcWsPort)
		}
	}

	return svc, controllerutil.SetControllerReference(nodeSet, svc, r.Scheme)
}

func (r *Reconciler) getInternalServiceSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-internal", group.GetServiceName(nodeSet)),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
				controllers.LabelScope:             scopeGroup,
			}),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
			},
			Selector: map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
			},
		},
	}

	if group.Config.IsEvmEnabled() {
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

	return svc, controllerutil.SetControllerReference(nodeSet, svc, r.Scheme)
}

func (r *Reconciler) getGlobalServiceSpec(nodeSet *appsv1.ChainNodeSet, globalIngress appsv1.GlobalIngressConfig) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      globalIngress.GetName(nodeSet),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				controllers.LabelGlobalIngress: globalIngress.Name,
				controllers.LabelScope:         scopeGlobal,
			}),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
					Name:       controllers.EvmRpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       controllers.EvmRpcPort,
					TargetPort: intstr.FromInt32(controllers.EvmRpcPort),
				},
				{
					Name:       controllers.EvmRpcWsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       controllers.EvmRpcWsPort,
					TargetPort: intstr.FromInt32(controllers.EvmRpcWsPort),
				},
			},
			Selector: map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				globalIngress.GetName(nodeSet): strconv.FormatBool(true),
			},
		},
	}

	if globalIngress.ShouldUseCosmoGuardPorts(nodeSet) {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt32(controllers.CosmoGuardRpcPort)
		svc.Spec.Ports[1].TargetPort = intstr.FromInt32(controllers.CosmoGuardLcdPort)
		svc.Spec.Ports[2].TargetPort = intstr.FromInt32(controllers.CosmoGuardGrpcPort)
		svc.Spec.Ports[3].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcPort)
		svc.Spec.Ports[4].TargetPort = intstr.FromInt32(controllers.CosmoGuardEvmRpcWsPort)
	}

	return svc, controllerutil.SetControllerReference(nodeSet, svc, r.Scheme)
}

func (r *Reconciler) getGlobalInternalServiceSpec(nodeSet *appsv1.ChainNodeSet, globalIngress appsv1.GlobalIngressConfig) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-internal", globalIngress.GetName(nodeSet)),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				controllers.LabelGlobalIngress: globalIngress.Name,
				controllers.LabelScope:         scopeGlobal,
			}),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
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
					Name:       controllers.EvmRpcPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       controllers.EvmRpcPort,
					TargetPort: intstr.FromInt32(controllers.EvmRpcPort),
				},
				{
					Name:       controllers.EvmRpcWsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       controllers.EvmRpcWsPort,
					TargetPort: intstr.FromInt32(controllers.EvmRpcWsPort),
				},
			},
			Selector: map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				globalIngress.GetName(nodeSet): strconv.FormatBool(true),
			},
		},
	}

	return svc, controllerutil.SetControllerReference(nodeSet, svc, r.Scheme)
}

func (r *Reconciler) listChainNodeSetServices(ctx context.Context, nodeSet *appsv1.ChainNodeSet, l ...string) (*corev1.ServiceList, error) {
	if len(l)%2 != 0 {
		return nil, fmt.Errorf("list of labels must contain pairs of key-value")
	}

	selectorMap := map[string]string{controllers.LabelChainNodeSet: nodeSet.GetName()}
	for i := 0; i < len(l); i += 2 {
		selectorMap[l[i]] = l[i+1]
	}

	serviceList := &corev1.ServiceList{}
	return serviceList, r.List(ctx, serviceList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(selectorMap),
	})
}
