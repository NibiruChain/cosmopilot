package chainnode

import (
	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/controllers"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

func WithChainNodeLabels(chainNode *appsv1.ChainNode, additional ...map[string]string) map[string]string {
	labels := utils.ExcludeMapKeys(chainNode.ObjectMeta.Labels, controllers.LabelWorkerName)
	for _, m := range additional {
		labels = utils.MergeMaps(labels, m, controllers.LabelWorkerName)
	}
	return labels
}
