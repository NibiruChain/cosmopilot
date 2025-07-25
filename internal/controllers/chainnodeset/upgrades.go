package chainnodeset

import (
	"context"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func (r *Reconciler) ensureUpgrades(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	if nodeSet.Status.Upgrades == nil {
		nodeSet.Status.Upgrades = make([]appsv1.Upgrade, 0)
	}

	statusCopy := nodeSet.Status.DeepCopy()

	// Grab all nodes for this ChainNodeSet
	selector := labels.SelectorFromSet(map[string]string{
		controllers.LabelChainNodeSet: nodeSet.GetName(),
	})
	chainNodeList := &appsv1.ChainNodeList{}
	if err := r.List(ctx, chainNodeList, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return err
	}

	for _, node := range chainNodeList.Items {
		for _, upgrade := range node.Status.Upgrades {
			nodeSet.Status.Upgrades = AddOrUpdateUpgrade(nodeSet.Status.Upgrades, upgrade)
		}
		if node.Status.LatestHeight > nodeSet.Status.LatestHeight {
			nodeSet.Status.LatestHeight = node.Status.LatestHeight
		}
	}

	// Sort upgrades by height
	sort.Slice(nodeSet.Status.Upgrades, func(i, j int) bool {
		return nodeSet.Status.Upgrades[i].Height < nodeSet.Status.Upgrades[j].Height
	})

	if statusCopy.LatestHeight != nodeSet.Status.LatestHeight || !reflect.DeepEqual(nodeSet.Status.Upgrades, statusCopy.Upgrades) {
		logger.Info("updating .status.upgrades")
		return r.Status().Update(ctx, nodeSet)
	}
	return nil
}

func AddOrUpdateUpgrade(upgrades []appsv1.Upgrade, upgrade appsv1.Upgrade) []appsv1.Upgrade {
	for i, u := range upgrades {
		if u.Height == upgrade.Height {
			// ChainNodeSet might contain nodes that actually did the upgrade and others that skipped it.
			// Set lets mark all of them as completed
			if u.Status == appsv1.UpgradeSkipped || (upgrade.Status == appsv1.UpgradeCompleted || upgrade.Status == appsv1.UpgradeSkipped) {
				upgrades[i].Status = appsv1.UpgradeCompleted
			}
			return upgrades
		}
	}
	upgrades = append(upgrades, upgrade)
	return upgrades
}
