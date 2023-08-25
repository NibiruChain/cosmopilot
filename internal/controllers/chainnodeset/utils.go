package chainnodeset

import (
	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

func (r *Reconciler) AddOrUpdateNodeStatus(nodeSet *appsv1.ChainNodeSet, status appsv1.ChainNodeSetNodeStatus) {
	if nodeSet.Status.Nodes == nil {
		nodeSet.Status.Nodes = []appsv1.ChainNodeSetNodeStatus{status}
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

func WithChainNodeSetLabels(nodeSet *appsv1.ChainNodeSet, additional ...map[string]string) map[string]string {
	labels := make(map[string]string, len(nodeSet.ObjectMeta.Labels))
	for k, v := range nodeSet.ObjectMeta.Labels {
		labels[k] = v
	}
	for _, m := range additional {
		labels = utils.MergeMaps(labels, m)
	}
	return labels
}
