package chainnode

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

func WithChainNodeLabels(chainNode *appsv1.ChainNode, additional ...map[string]string) map[string]string {
	labels := utils.ExcludeMapKeys(chainNode.ObjectMeta.Labels, controllers.LabelWorkerName)
	for _, m := range additional {
		labels = utils.MergeMaps(labels, m, controllers.LabelWorkerName)
	}
	return labels
}

func (r *Reconciler) filterNonReadyPeers(ctx context.Context, chainNode *appsv1.ChainNode, peers []appsv1.Peer) []appsv1.Peer {
	logger := log.FromContext(ctx)
	workingPeers := make([]appsv1.Peer, 0)

	for _, peer := range peers {
		client, err := r.getChainNodeClientByHost(fmt.Sprintf("%s.%s.svc.cluster.local", peer.Address, chainNode.GetNamespace()))
		if err != nil {
			logger.Info("excluding peer from rpc-servers list", "peer", peer.ID, "address", peer.Address)
			continue
		}
		_, err = client.GetNodeStatus(ctx)
		if err != nil {
			logger.Info("excluding peer from rpc-servers list", "peer", peer.ID, "address", peer.Address)
			continue
		}
		workingPeers = append(workingPeers, peer)
	}

	return workingPeers
}
