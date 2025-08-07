package chainnode

import (
	"context"
	"fmt"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func (r *Reconciler) ensureIngresses(ctx context.Context, chainNode *appsv1.ChainNode) error {
	if chainNode.Spec.Ingress == nil {
		// let's try to delete ingresses if they exist
		if err := r.Delete(ctx, &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      chainNode.GetName(),
				Namespace: chainNode.GetNamespace(),
			},
		}); err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err := r.Delete(ctx, &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-grpc", chainNode.GetName()),
				Namespace: chainNode.GetNamespace(),
			},
		}); err != nil && !errors.IsNotFound(err) {
			return err
		}
	} else {
		ingress, err := r.getIngressSpec(chainNode)
		if err != nil {
			return err
		}

		if err = r.ensureIngress(ctx, ingress); err != nil {
			return err
		}

		grpcIngress, err := r.getGrpcIngressSpec(chainNode)
		if err != nil {
			return err
		}

		if !chainNode.Spec.Ingress.EnableGRPC {
			if err = r.Delete(ctx, grpcIngress); err != nil && !errors.IsNotFound(err) {
				return err
			}
		} else {
			if err = r.ensureIngress(ctx, grpcIngress); err != nil {
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

func (r *Reconciler) getIngressSpec(chainNode *appsv1.ChainNode) (*v1.Ingress, error) {
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        chainNode.GetName(),
			Namespace:   chainNode.GetNamespace(),
			Labels:      WithChainNodeLabels(chainNode),
			Annotations: chainNode.Spec.Ingress.Annotations,
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(chainNode.GetIngressClass()),
			Rules:            make([]v1.IngressRule, 0),
		},
	}

	if !chainNode.Spec.Ingress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{},
				SecretName: chainNode.GetIngressSecretName(),
			},
		}
	}

	pathType := v1.PathTypeImplementationSpecific

	if chainNode.Spec.Ingress.EnableRPC {
		host := fmt.Sprintf("rpc.%s", chainNode.Spec.Ingress.Host)
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
									Name: chainNode.GetServiceName(),
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

	if chainNode.Spec.Ingress.EnableLCD {
		host := fmt.Sprintf("lcd.%s", chainNode.Spec.Ingress.Host)
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
									Name: chainNode.GetServiceName(),
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

	if chainNode.Spec.Ingress.EnableEvmRPC {
		host := fmt.Sprintf("evm-rpc.%s", chainNode.Spec.Ingress.Host)
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
									Name: chainNode.GetServiceName(),
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

	if chainNode.Spec.Ingress.EnableEvmRpcWs {
		host := fmt.Sprintf("evm-rpc-ws.%s", chainNode.Spec.Ingress.Host)
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
									Name: chainNode.GetServiceName(),
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

	if chainNode.Spec.Ingress.EnableGRPC && !chainNode.Spec.Ingress.DisableTLS {
		// We just append the hostname to TLS config and add no rule as it will be handled by a separate ingress
		// but will use the same certificate
		ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, fmt.Sprintf("grpc.%s", chainNode.Spec.Ingress.Host))
	}

	return ingress, controllerutil.SetControllerReference(chainNode, ingress, r.Scheme)
}

func (r *Reconciler) getGrpcIngressSpec(chainNode *appsv1.ChainNode) (*v1.Ingress, error) {
	pathType := v1.PathTypeImplementationSpecific
	ingress := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-grpc", chainNode.GetName()),
			Namespace:   chainNode.GetNamespace(),
			Labels:      WithChainNodeLabels(chainNode),
			Annotations: chainNode.GetGrpcAnnotations(),
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(chainNode.GetIngressClass()),
			Rules: []v1.IngressRule{
				{
					Host: fmt.Sprintf("grpc.%s", chainNode.Spec.Ingress.Host),
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{Paths: []v1.HTTPIngressPath{
							{
								PathType: &pathType,
								Backend: v1.IngressBackend{
									Service: &v1.IngressServiceBackend{
										Name: chainNode.GetServiceName(),
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
	if !chainNode.Spec.Ingress.DisableTLS {
		ingress.Spec.TLS = []v1.IngressTLS{
			{
				Hosts:      []string{fmt.Sprintf("grpc.%s", chainNode.Spec.Ingress.Host)},
				SecretName: chainNode.GetIngressSecretName(),
			},
		}
	}
	return ingress, controllerutil.SetControllerReference(chainNode, ingress, r.Scheme)
}
