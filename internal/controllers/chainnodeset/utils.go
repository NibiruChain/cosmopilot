package chainnodeset

import (
	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/utils"
)

func AddOrUpdateNodeStatus(nodeSet *appsv1.ChainNodeSet, status appsv1.ChainNodeSetNodeStatus) {
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

func DeleteNodeStatus(nodeSet *appsv1.ChainNodeSet, name string) {
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

func ContainsGroup(groups []appsv1.NodeGroupSpec, groupName string) bool {
	for _, group := range groups {
		if group.Name == groupName {
			return true
		}
	}
	return false
}

func ContainsGlobalIngress(ingresses []appsv1.GlobalIngressConfig, ingressName string) bool {
	for _, ingress := range ingresses {
		if ingress.Name == ingressName {
			return true
		}
	}
	return false
}
