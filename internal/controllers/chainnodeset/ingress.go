package chainnodeset

import (
	"context"
	"fmt"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func (r *Reconciler) ensureIngresses(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	for _, group := range nodeSet.Spec.Nodes {
		if group.Ingress != nil {
			ingress, err := r.getIngressSpec(nodeSet, group)
			if err != nil {
				return err
			}

			if err = r.ensureIngress(ctx, ingress); err != nil {
				return err
			}

			grpcIngress, err := r.getGrpcIngressSpec(nodeSet, group)
			if err != nil {
				return err
			}

			if !group.Ingress.EnableGRPC {
				if err = r.Delete(ctx, grpcIngress); err != nil && !errors.IsNotFound(err) {
					return err
				}
			} else {
				if err = r.ensureIngress(ctx, grpcIngress); err != nil {
					return err
				}
			}
		}
	}

	for _, globalIngress := range nodeSet.Spec.Ingresses {
		if !globalIngress.CreateServicesOnly() {
			ingress, err := r.getGlobalIngressSpec(nodeSet, globalIngress)
			if err != nil {
				return err
			}

			if err = r.ensureIngress(ctx, ingress); err != nil {
				return err
			}

			grpcIngress, err := r.getGrpcGlobalIngressSpec(nodeSet, globalIngress)
			if err != nil {
				return err
			}

			if !globalIngress.EnableGRPC {
				if err = r.Delete(ctx, &v1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      globalIngress.GetGrpcName(nodeSet),
						Namespace: nodeSet.Namespace,
					},
				}); err != nil && !errors.IsNotFound(err) {
					return err
				}
			} else {
				if err = r.ensureIngress(ctx, grpcIngress); err != nil {
					return err
				}
			}
		}
	}

	// Clean up if necessary
	groupIngresses, err := r.listChainNodeSetIngresses(ctx, nodeSet, controllers.LabelScope, scopeGroup)
	if err != nil {
		return err
	}

	for _, ing := range groupIngresses.Items {
		if _, ok := ing.Labels[controllers.LabelChainNodeSetGroup]; !ok ||
			!ContainsGroup(nodeSet.Spec.Nodes, ing.Labels[controllers.LabelChainNodeSetGroup]) {
			logger.Info("deleting ingress", "ingress", ing.GetName())
			if err = r.Delete(ctx, &ing); err != nil {
				return err
			}
		}
	}

	globalIngresses, err := r.listChainNodeSetIngresses(ctx, nodeSet, controllers.LabelScope, scopeGlobal)
	if err != nil {
		return err
	}

	for _, ing := range globalIngresses.Items {
		if _, ok := ing.Labels[controllers.LabelGlobalIngress]; !ok ||
			!ContainsGlobalIngress(nodeSet.Spec.Ingresses, ing.Labels[controllers.LabelGlobalIngress], true) {
			logger.Info("deleting ingress", "ingress", ing.GetName())
			if err = r.Delete(ctx, &ing); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) ensureIngress(ctx context.Context, ingress *v1.Ingress) error {
	logger := log.FromContext(ctx)

	currentIg := &v1.Ingress{}
	err := r.Get(ctx, client.ObjectKeyFromObject(ingress), currentIg)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating ingress", "ingress", ingress.GetName())
			return r.Create(ctx, ingress)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentIg, ingress)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		logger.Info("updating ingress", "ingress", ingress.GetName())

		ingress.ObjectMeta.ResourceVersion = currentIg.ObjectMeta.ResourceVersion
		if err := r.Update(ctx, ingress); err != nil {
			return err
		}
	}

	*ingress = *currentIg
	return nil
}

func (r *Reconciler) getIngressSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) (*v1.Ingress, error) {
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", nodeSet.GetName(), group.Name),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
				controllers.LabelScope:             scopeGroup,
			}),
			Annotations: group.Ingress.Annotations,
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(group.GetIngressClass()),
			Rules:            make([]v1.IngressRule, 0),
		},
	}

	if !group.Ingress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{},
				SecretName: group.GetIngressSecretName(nodeSet),
			},
		}
	}

	pathType := v1.PathTypeImplementationSpecific

	if group.Ingress.EnableRPC {
		host := fmt.Sprintf("rpc.%s", group.Ingress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: group.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: chainutils.RpcPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if group.Ingress.EnableLCD {
		host := fmt.Sprintf("lcd.%s", group.Ingress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: group.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: chainutils.LcdPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if group.Ingress.EnableEvmRPC {
		host := fmt.Sprintf("evm-rpc.%s", group.Ingress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: group.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: controllers.EvmRpcPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if group.Ingress.EnableEvmRpcWs {
		host := fmt.Sprintf("evm-rpc-ws.%s", group.Ingress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: group.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: controllers.EvmRpcWsPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if group.Ingress.EnableGRPC && !group.Ingress.DisableTLS {
		// We just append the hostname to TLS config and add no rule as it will be handled by a separate ingress
		// but will use the same certificate
		ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, fmt.Sprintf("grpc.%s", group.Ingress.Host))
	}

	return ingress, controllerutil.SetControllerReference(nodeSet, ingress, r.Scheme)
}

func (r *Reconciler) getGrpcIngressSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) (*v1.Ingress, error) {
	pathType := v1.PathTypeImplementationSpecific
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-grpc", nodeSet.GetName(), group.Name),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:      nodeSet.GetName(),
				controllers.LabelChainNodeSetGroup: group.Name,
				controllers.LabelScope:             scopeGroup,
			}),
			Annotations: group.GetGrpcAnnotations(),
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(group.GetIngressClass()),
			Rules: []v1.IngressRule{
				{
					Host: fmt.Sprintf("grpc.%s", group.Ingress.Host),
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{Paths: []v1.HTTPIngressPath{
							{
								PathType: &pathType,
								Backend: v1.IngressBackend{
									Service: &v1.IngressServiceBackend{
										Name: group.GetServiceName(nodeSet),
										Port: v1.ServiceBackendPort{
											Number: chainutils.GrpcPort,
										},
									},
								},
							},
						}},
					},
				},
			},
		},
	}
	if !group.Ingress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{fmt.Sprintf("grpc.%s", group.Ingress.Host)},
				SecretName: group.GetIngressSecretName(nodeSet),
			},
		}
	}
	return ingress, controllerutil.SetControllerReference(nodeSet, ingress, r.Scheme)
}

func (r *Reconciler) getGlobalIngressSpec(nodeSet *appsv1.ChainNodeSet, globalIngress appsv1.GlobalIngressConfig) (*v1.Ingress, error) {
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      globalIngress.GetName(nodeSet),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				controllers.LabelGlobalIngress: globalIngress.Name,
				controllers.LabelScope:         scopeGlobal,
			}),
			Annotations: globalIngress.Annotations,
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(globalIngress.GetIngressClass()),
			Rules:            make([]v1.IngressRule, 0),
		},
	}

	if !globalIngress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{},
				SecretName: globalIngress.GetTlsSecretName(nodeSet),
			},
		}
	}

	pathType := v1.PathTypeImplementationSpecific

	if globalIngress.EnableRPC {
		host := fmt.Sprintf("rpc.%s", globalIngress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: globalIngress.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: chainutils.RpcPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if globalIngress.EnableLCD {
		host := fmt.Sprintf("lcd.%s", globalIngress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: globalIngress.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: chainutils.LcdPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if globalIngress.EnableEvmRPC {
		host := fmt.Sprintf("evm-rpc.%s", globalIngress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: globalIngress.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: controllers.EvmRpcPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if globalIngress.EnableEvmRpcWs {
		host := fmt.Sprintf("evm-rpc-ws.%s", globalIngress.Host)
		if ingress.Spec.TLS != nil {
			ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
		}
		ingress.Spec.Rules = append(ingress.Spec.Rules, v1.IngressRule{
			Host: host,
			IngressRuleValue: v1.IngressRuleValue{
				HTTP: &v1.HTTPIngressRuleValue{
					Paths: []v1.HTTPIngressPath{
						{
							PathType: &pathType,
							Backend: v1.IngressBackend{
								Service: &v1.IngressServiceBackend{
									Name: globalIngress.GetServiceName(nodeSet),
									Port: v1.ServiceBackendPort{
										Number: controllers.EvmRpcWsPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		})
	}

	if globalIngress.EnableGRPC && !globalIngress.DisableTLS {
		// We just append the hostname to TLS config and add no rule as it will be handled by a separate ingress
		// but will use the same certificate
		ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, fmt.Sprintf("grpc.%s", globalIngress.Host))
	}

	return ingress, controllerutil.SetControllerReference(nodeSet, ingress, r.Scheme)
}

func (r *Reconciler) getGrpcGlobalIngressSpec(nodeSet *appsv1.ChainNodeSet, globalIngress appsv1.GlobalIngressConfig) (*v1.Ingress, error) {
	pathType := v1.PathTypeImplementationSpecific
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      globalIngress.GetGrpcName(nodeSet),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				controllers.LabelChainNodeSet:  nodeSet.GetName(),
				controllers.LabelGlobalIngress: globalIngress.Name,
				controllers.LabelScope:         scopeGlobal,
			}),
			Annotations: globalIngress.GetGrpcAnnotations(),
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(globalIngress.GetIngressClass()),
			Rules: []v1.IngressRule{
				{
					Host: fmt.Sprintf("grpc.%s", globalIngress.Host),
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{Paths: []v1.HTTPIngressPath{
							{
								PathType: &pathType,
								Backend: v1.IngressBackend{
									Service: &v1.IngressServiceBackend{
										Name: globalIngress.GetServiceName(nodeSet),
										Port: v1.ServiceBackendPort{
											Number: chainutils.GrpcPort,
										},
									},
								},
							},
						}},
					},
				},
			},
		},
	}

	if !globalIngress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{fmt.Sprintf("grpc.%s", globalIngress.Host)},
				SecretName: globalIngress.GetTlsSecretName(nodeSet),
			},
		}
	}
	return ingress, controllerutil.SetControllerReference(nodeSet, ingress, r.Scheme)
}

func (r *Reconciler) listChainNodeSetIngresses(ctx context.Context, nodeSet *appsv1.ChainNodeSet, l ...string) (*v1.IngressList, error) {
	if len(l)%2 != 0 {
		return nil, fmt.Errorf("list of labels must contain pairs of key-value")
	}

	selectorMap := map[string]string{controllers.LabelChainNodeSet: nodeSet.GetName()}
	for i := 0; i < len(l); i += 2 {
		selectorMap[l[i]] = l[i+1]
	}

	ingressList := &v1.IngressList{}
	return ingressList, r.List(ctx, ingressList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(selectorMap),
	})
}
