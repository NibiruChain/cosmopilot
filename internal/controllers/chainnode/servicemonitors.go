package chainnode

import (
	"context"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/controllers"
)

func (r *Reconciler) ensureServiceMonitors(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	spec, err := r.getServiceMonitorSpec(chainNode)
	if err != nil {
		return err
	}

	if chainNode.Spec.Config.ServiceMonitorsEnabled() {
		current := &monitoring.ServiceMonitor{}
		err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), current)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("creating service monitor", "servicemonitor", spec.GetName())
				return r.Create(ctx, spec)
			}
			return err
		}

		patchResult, err := patch.DefaultPatchMaker.Calculate(current, spec)
		if err != nil {
			return err
		}

		if !patchResult.IsEmpty() {
			spec.ObjectMeta.ResourceVersion = current.ObjectMeta.ResourceVersion

			logger.Info("updating service monitor", "servicemonitor", spec.GetName())
			return r.Update(ctx, spec)
		}
	} else {
		_ = r.Delete(ctx, spec)
	}

	return nil
}

func (r *Reconciler) getServiceMonitorSpec(chainNode *appsv1.ChainNode) (*monitoring.ServiceMonitor, error) {
	spec := &monitoring.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.GetName(),
			Namespace: chainNode.GetNamespace(),
			Labels:    WithChainNodeLabels(chainNode, chainNode.Spec.Config.ServiceMonitorSelector()),
		},
		Spec: monitoring.ServiceMonitorSpec{
			Endpoints: []monitoring.Endpoint{
				{
					Port:     chainutils.PrometheusPortName,
					Interval: prometheusScrapeInterval,
					MetricRelabelConfigs: []*monitoring.RelabelConfig{
						{
							Regex:  "peer_id",
							Action: "labeldrop",
						},
					},
				},
				{
					Port:     controllers.CosmoGuardMetricsPortName,
					Interval: prometheusScrapeInterval,
					Path:     "/metrics",
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelNodeID:  chainNode.Status.NodeID,
					LabelChainID: chainNode.Status.ChainID,
				},
			},
			NamespaceSelector: monitoring.NamespaceSelector{
				MatchNames: []string{chainNode.GetNamespace()},
			},
		},
	}
	return spec, controllerutil.SetControllerReference(chainNode, spec, r.Scheme)
}
