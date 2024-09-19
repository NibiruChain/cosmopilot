package chainnodeset

import appsv1 "github.com/NibiruChain/cosmopilot/api/v1"

func setChainNodeServiceMonitor(nodeSet *appsv1.ChainNodeSet, chainNode *appsv1.ChainNode) {
	if nodeSet.Spec.ServiceMonitor == nil {
		return
	}

	if chainNode.Spec.Config == nil {
		chainNode.Spec.Config = &appsv1.Config{}
	}

	if chainNode.Spec.Config.ServiceMonitor == nil {
		chainNode.Spec.Config.ServiceMonitor = nodeSet.Spec.ServiceMonitor
	}
}
