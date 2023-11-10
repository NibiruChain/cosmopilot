package chainnodeset

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

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

func (r *Reconciler) ensureIngresses(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	for _, group := range nodeSet.Spec.Nodes {
		if group.Ingress == nil {
			// let's try to delete ingresses if they exist
			if err := r.Delete(ctx, &v1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", nodeSet.GetName(), group.Name),
					Namespace: nodeSet.Namespace,
				},
			}); err != nil && !errors.IsNotFound(err) {
				return err
			}
			if err := r.Delete(ctx, &v1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-grpc", nodeSet.GetName(), group.Name),
					Namespace: nodeSet.Namespace,
				},
			}); err != nil && !errors.IsNotFound(err) {
				return err
			}
		} else {
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
				if err = r.Delete(ctx, &v1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s-grpc", nodeSet.GetName(), group.Name),
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
			Name:        fmt.Sprintf("%s-%s", nodeSet.GetName(), group.Name),
			Namespace:   nodeSet.GetNamespace(),
			Labels:      WithChainNodeSetLabels(nodeSet),
			Annotations: group.Ingress.Annotations,
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(ingressClassNameNginx),
			TLS: []v1.IngressTLS{
				{
					Hosts:      []string{},
					SecretName: group.GetIngressSecretName(nodeSet),
				},
			},
			Rules: make([]v1.IngressRule, 0),
		},
	}

	pathType := v1.PathTypeImplementationSpecific

	if group.Ingress.EnableRPC {
		host := fmt.Sprintf("rpc.%s", group.Ingress.Host)
		ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
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
		ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, host)
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

	if group.Ingress.EnableGRPC {
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
			Name:        fmt.Sprintf("%s-%s-grpc", nodeSet.GetName(), group.Name),
			Namespace:   nodeSet.GetNamespace(),
			Labels:      WithChainNodeSetLabels(nodeSet),
			Annotations: nginxGrpcAnnotations,
		},
		Spec: v1.IngressSpec{
			IngressClassName: pointer.String(ingressClassNameNginx),
			TLS: []v1.IngressTLS{
				{
					Hosts:      []string{fmt.Sprintf("grpc.%s", group.Ingress.Host)},
					SecretName: group.GetIngressSecretName(nodeSet),
				},
			},
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
	return ingress, controllerutil.SetControllerReference(nodeSet, ingress, r.Scheme)
}
