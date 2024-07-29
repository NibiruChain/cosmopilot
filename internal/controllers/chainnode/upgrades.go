package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/pkg/nodeutils"
)

func (r *Reconciler) ensureUpgrades(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	if chainNode.Status.Upgrades == nil {
		chainNode.Status.Upgrades = make([]appsv1.Upgrade, 0)
	}

	statusCopy := chainNode.Status.DeepCopy()

	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning && chainNode.Spec.App.ShouldQueryGovUpgrades() {
		// Get gov upgrades
		govUpgrades, err := r.getGovUpgrades(ctx, chainNode)
		if err != nil {
			logger.Error(err, "could not retrieve upgrade plans")
		} else {
			for _, upgrade := range govUpgrades {
				chainNode.Status.Upgrades = AddOrUpdateUpgrade(chainNode.Status.Upgrades, upgrade, chainNode.Status.LatestHeight)
			}
		}
	}

	for _, upgrade := range chainNode.Spec.App.Upgrades {
		u := appsv1.Upgrade{
			Height: upgrade.Height,
			Image:  upgrade.Image,
			Status: appsv1.UpgradeScheduled,
			Source: appsv1.ManualUpgrade,
		}
		// If this upgrade is in the past, lets set it as skipped
		if chainNode.Status.LatestHeight > u.Height {
			u.Status = appsv1.UpgradeSkipped
		}

		// Maybe set this upgrade as gov planned upgraded
		if upgrade.ForceGovUpgrade() {
			u.Source = appsv1.OnChainUpgrade
		}

		chainNode.Status.Upgrades = AddOrUpdateUpgrade(chainNode.Status.Upgrades, u, chainNode.Status.LatestHeight)
	}

	// Sort upgrades by height
	sort.Slice(chainNode.Status.Upgrades, func(i, j int) bool {
		return chainNode.Status.Upgrades[i].Height < chainNode.Status.Upgrades[j].Height
	})

	if err := r.ensureUpgradesConfig(ctx, chainNode); err != nil {
		return err
	}

	if !reflect.DeepEqual(chainNode.Status.Upgrades, statusCopy.Upgrades) {
		logger.Info("updating .status.upgrades")
		return r.Status().Update(ctx, chainNode)
	}
	return nil
}

func (r *Reconciler) ensureUpgradesConfig(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

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
			logger.Info("creating configmap with upgrades", "configmap", cm.GetName())
			return r.Create(ctx, spec)
		}
		return err
	}

	// Update when config changes
	if cm.Data[upgradesConfigFile] != string(b) {
		logger.Info("updating configmap with upgrades", "configmap", cm.GetName())
		cm.Data[upgradesConfigFile] = string(b)
		return r.Update(ctx, cm)
	}
	return nil
}

func (r *Reconciler) requiresUpgrade(chainNode *appsv1.ChainNode) (bool, error) {
	return nodeutils.NewClient(chainNode.GetNodeFQDN()).RequiresUpgrade()
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
	logger := log.FromContext(ctx)

	for i, u := range chainNode.Status.Upgrades {
		if u.Height == upgrade.Height {
			chainNode.Status.Upgrades[i].Status = status
			if status == appsv1.UpgradeCompleted {
				addUpgradeStatusCondition(chainNode, upgrade)
			}
			logger.Info("setting upgrade status", "height", upgrade.Height, "status", status)
			if err := r.Status().Update(ctx, chainNode); err != nil {
				return err
			}
			// always update upgrades configmap so node-utils is aware of upgrade status
			return r.ensureUpgradesConfig(ctx, chainNode)
		}
	}
	return fmt.Errorf("cant update upgrade phase: upgrade not found")
}

func (r *Reconciler) getGovUpgrades(ctx context.Context, chainNode *appsv1.ChainNode) ([]appsv1.Upgrade, error) {
	c, err := r.getClient(chainNode)
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

func AddOrUpdateUpgrade(upgrades []appsv1.Upgrade, upgrade appsv1.Upgrade, currentHeight int64) []appsv1.Upgrade {
	for i, u := range upgrades {
		if u.Height == upgrade.Height {
			// Update if we are adding a missing image
			if u.Status == appsv1.UpgradeImageMissing && upgrade.Image != "" {
				upgrades[i].Image = upgrade.Image
				upgrades[i].Source = upgrade.Source
				upgrades[i].Status = appsv1.UpgradeScheduled
			}

			// If we are updating an upgrade with a past height, and it was not completed, lets set it
			// as skipped
			if u.Status != appsv1.UpgradeCompleted && u.Height < currentHeight {
				upgrades[i].Status = appsv1.UpgradeSkipped
			}

			upgrades[i].Source = upgrade.Source
			return upgrades
		}
	}
	upgrades = append(upgrades, upgrade)
	return upgrades
}

func addUpgradeStatusCondition(chainNode *appsv1.ChainNode, upgrade *appsv1.Upgrade) {
	if chainNode.Status.Conditions == nil {
		chainNode.Status.Conditions = make([]metav1.Condition, 0)
	}
	chainNode.Status.Conditions = append(chainNode.Status.Conditions, metav1.Condition{
		Type:               appsv1.ConditionUpgrade,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             appsv1.ReasonUpgradeSuccess,
		Message:            fmt.Sprintf("Successfully upgraded node to image %s", upgrade.Image),
	})
}
