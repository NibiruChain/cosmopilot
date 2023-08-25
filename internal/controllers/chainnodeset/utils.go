package chainnodeset

import appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"

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

func applyChainNodeSetLabels(nodeSet *appsv1.ChainNodeSet, chainNode *appsv1.ChainNode) {
	for k, v := range nodeSet.Labels {
		chainNode.Labels[k] = v
	}
}
