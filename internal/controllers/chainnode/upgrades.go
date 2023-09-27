package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

func (r *Reconciler) ensureUpgrades(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	if chainNode.Status.Upgrades == nil {
		chainNode.Status.Upgrades = make([]appsv1.Upgrade, 0)
	}

	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning && chainNode.Spec.App.ShouldQueryGovUpgrades() {
		// Get gov upgrades
		govUpgrades, err := r.getGovUpgrades(ctx, chainNode)
		if err != nil {
			logger.Error(err, "could not retrieve upgrade plans")
		} else {
			for _, upgrade := range govUpgrades {
				chainNode.Status.Upgrades = AddOrUpdateUpgrade(chainNode.Status.Upgrades, upgrade)
			}
		}
	}

	for _, upgrade := range chainNode.Spec.App.Upgrades {
		chainNode.Status.Upgrades = AddOrUpdateUpgrade(chainNode.Status.Upgrades, appsv1.Upgrade{
			Height: upgrade.Height,
			Image:  upgrade.Image,
			Status: appsv1.UpgradeScheduled,
			Source: appsv1.ManualUpgrade,
		})
	}

	// Sort upgrades by height
	sort.Slice(chainNode.Status.Upgrades, func(i, j int) bool {
		return chainNode.Status.Upgrades[i].Height < chainNode.Status.Upgrades[j].Height
	})

	if err := r.ensureUpgradesConfig(ctx, chainNode); err != nil {
		return err
	}

	return r.Status().Update(ctx, chainNode)
}

func (r *Reconciler) ensureUpgradesConfig(ctx context.Context, chainNode *appsv1.ChainNode) error {
	upgrades := struct {
		Upgrades []appsv1.Upgrade `json:"upgrades"`
	}{
		Upgrades: chainNode.Status.Upgrades,
	}
	b, err := json.Marshal(upgrades)
	if err != nil {
		return err
	}

	spec := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-upgrades", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
		},
		Data: map[string]string{upgradesConfigFile: string(b)},
	}
	if err := controllerutil.SetControllerReference(chainNode, spec, r.Scheme); err != nil {
		return err
	}

	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKeyFromObject(spec), cm)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, spec)
		}
		return err
	}

	// Update when config changes
	if cm.Data[upgradesConfigFile] != string(b) {
		cm.Data[upgradesConfigFile] = string(b)
		return r.Update(ctx, cm)
	}
	return nil
}

func (r *Reconciler) getUpgrade(chainNode *appsv1.ChainNode, height int64) *appsv1.Upgrade {
	for _, upgrade := range chainNode.Status.Upgrades {
		if upgrade.Height == height && upgrade.Status == appsv1.UpgradeScheduled {
			return &upgrade
		}
	}
	return nil
}

func (r *Reconciler) setUpgradeStatus(ctx context.Context, chainNode *appsv1.ChainNode, upgrade *appsv1.Upgrade, status appsv1.UpgradePhase) error {
	for i, u := range chainNode.Status.Upgrades {
		if u.Height == upgrade.Height {
			chainNode.Status.Upgrades[i].Status = status
			return r.Status().Update(ctx, chainNode)
		}
	}
	return fmt.Errorf("can update upgrade phase: upgrade not found")
}

func (r *Reconciler) getGovUpgrades(ctx context.Context, chainNode *appsv1.ChainNode) ([]appsv1.Upgrade, error) {
	c, err := r.getQueryClient(chainNode)
	if err != nil {
		return nil, err
	}

	plannedUpgrade, err := c.GetNextUpgrade(ctx)
	if err != nil {
		return nil, err
	}

	upgrades := make([]appsv1.Upgrade, 0)
	if plannedUpgrade != nil {
		upgrade := appsv1.Upgrade{
			Height: plannedUpgrade.Height,
			Status: appsv1.UpgradeScheduled,
			Source: appsv1.OnChainUpgrade,
		}

		info := struct {
			Binaries struct {
				Docker string `json:"docker"`
			} `json:"binaries"`
		}{}
		if err := json.Unmarshal([]byte(plannedUpgrade.Info), &info); err == nil && info.Binaries.Docker != "" {
			upgrade.Image = info.Binaries.Docker
		} else {
			upgrade.Status = appsv1.UpgradeImageMissing
		}
		upgrades = append(upgrades, upgrade)
	}
	return upgrades, nil
}

func AddOrUpdateUpgrade(upgrades []appsv1.Upgrade, upgrade appsv1.Upgrade) []appsv1.Upgrade {
	for i, u := range upgrades {
		if u.Height == upgrade.Height {
			// Only source and image can be updated, unless we are adding a missing image
			// where we also update status.
			upgrades[i].Source = upgrade.Source
			upgrades[i].Image = upgrade.Image
			if u.Status == appsv1.UpgradeImageMissing && upgrade.Image != "" {
				upgrades[i].Status = appsv1.UpgradeScheduled
			}
			return upgrades
		}
	}
	upgrades = append(upgrades, upgrade)
	return upgrades
}
