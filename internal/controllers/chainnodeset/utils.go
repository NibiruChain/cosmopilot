package chainnodeset

import (
	"context"
	"reflect"
	"strings"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

func AddOrUpdateNodeStatus(nodeSet *v1.ChainNodeSet, status v1.ChainNodeSetNodeStatus) {
	if nodeSet.Status.Nodes == nil {
		nodeSet.Status.Nodes = []v1.ChainNodeSetNodeStatus{status}
		return
	}

	found := false
	for i, s := range nodeSet.Status.Nodes {
		if s.Name == status.Name {
			found = true
			nodeSet.Status.Nodes[i] = status
		}
	}

	if !found {
		nodeSet.Status.Nodes = append(nodeSet.Status.Nodes, status)
	}
}

func DeleteNodeStatus(nodeSet *v1.ChainNodeSet, name string) {
	if nodeSet.Status.Nodes == nil {
		return
	}

	for i, s := range nodeSet.Status.Nodes {
		if s.Name == name {
			nodeSet.Status.Nodes = append(nodeSet.Status.Nodes[:i], nodeSet.Status.Nodes[i+1:]...)
			return
		}
	}
}

func WithChainNodeSetLabels(nodeSet *v1.ChainNodeSet, additional ...map[string]string) map[string]string {
	labels := make(map[string]string, len(nodeSet.ObjectMeta.Labels))
	for k, v := range nodeSet.ObjectMeta.Labels {
		labels[k] = v
	}
	for _, m := range additional {
		labels = utils.MergeMaps(labels, m)
	}
	return labels
}

func ContainsGroup(groups []v1.NodeGroupSpec, groupName string) bool {
	for _, group := range groups {
		if group.Name == groupName {
			return true
		}
	}
	return false
}

func ContainsGlobalIngress(ingresses []v1.GlobalIngressConfig, ingressName string) bool {
	for _, ingress := range ingresses {
		if ingress.Name == ingressName {
			return true
		}
	}
	return false
}

func (r *Reconciler) ensureConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	logger := log.FromContext(ctx).WithValues("cm", cm.GetName())

	currentCm := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(cm), currentCm)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating configmap")
			return r.Create(ctx, cm)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentCm, cm, patch.IgnoreStatusFields(), patch.IgnoreField("data"))
	if err != nil {
		return err
	}

	var shouldUpdate bool
	for file, data := range cm.Data {
		if oldData, ok := currentCm.Data[file]; !ok || data != oldData {
			shouldUpdate = true
			break
		}
	}

	if shouldUpdate || !patchResult.IsEmpty() || !reflect.DeepEqual(currentCm.Annotations, cm.Annotations) {
		logger.Info("updating configmap")
		cm.ObjectMeta.ResourceVersion = currentCm.ObjectMeta.ResourceVersion
		return r.Update(ctx, cm)
	}

	*cm = *currentCm
	return nil
}

func (r *Reconciler) ensureStatefulSet(ctx context.Context, ss *appsv1.StatefulSet) error {
	logger := log.FromContext(ctx).WithValues("statefulset", ss.GetName())

	currentStatefulset := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKeyFromObject(ss), currentStatefulset)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating statefulset")
			return r.Create(ctx, ss)
		}
		return err
	}

	currentStatefulset.Spec.VolumeClaimTemplates = ss.Spec.VolumeClaimTemplates
	patchResult, err := patch.DefaultPatchMaker.Calculate(currentStatefulset, ss, patch.IgnoreStatusFields())
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() || !reflect.DeepEqual(currentStatefulset.Annotations, ss.Annotations) {
		logger.Info("updating statefulset")
		ss.ObjectMeta.ResourceVersion = currentStatefulset.ObjectMeta.ResourceVersion
		return r.Update(ctx, ss)
	}

	*ss = *currentStatefulset
	return nil
}

func AddressWithPortFromFullAddress(fullAddress string) string {
	parts := strings.Split(fullAddress, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return fullAddress
}

func RemoveIdFromFullAddresses(fullAddresses []string) []string {
	result := make([]string, len(fullAddresses))
	for i, address := range fullAddresses {
		result[i] = AddressWithPortFromFullAddress(address)
	}
	return result
}
