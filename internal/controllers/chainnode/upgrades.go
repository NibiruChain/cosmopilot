package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

func (r *Reconciler) ensureUpgrades(ctx context.Context, chainNode *appsv1.ChainNode, nodePodRunning bool) error {
	logger := log.FromContext(ctx)

	if chainNode.Status.Upgrades == nil {
		chainNode.Status.Upgrades = make([]appsv1.Upgrade, 0)
	}

	statusCopy := chainNode.Status.DeepCopy()

	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning && nodePodRunning && chainNode.Spec.App.ShouldQueryGovUpgrades() {
		// Get gov upgrades
		govUpgrades, err := r.getGovUpgrades(ctx, chainNode)
		if err != nil {
			logger.Error(err, "could not retrieve upgrade plans")
		} else {
			// CurrentPlan is authoritative for future governance upgrades. A
			// successful empty response retracts a cancelled plan; a replacement
			// retracts the previous height before publishing the new one.
			chainNode.Status.Upgrades = retainCurrentGovUpgrades(chainNode.Status.Upgrades, govUpgrades)
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
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
	}
	return r.finishUpgradeCleanup(ctx, chainNode)
}

func retainCurrentGovUpgrades(cached, current []appsv1.Upgrade) []appsv1.Upgrade {
	heights := make(map[int64]bool, len(current))
	for _, upgrade := range current {
		heights[upgrade.Height] = true
	}
	retained := make([]appsv1.Upgrade, 0, len(cached))
	for _, upgrade := range cached {
		pending := upgrade.Status == appsv1.UpgradeScheduled || upgrade.Status == appsv1.UpgradeImageMissing
		// The controller marks an upgrade ongoing before switching its pod.
		// Merely reaching a scheduled height does not prove it was executed;
		// cancellation may have happened between discovery polls.
		if upgrade.Source == appsv1.OnChainUpgrade && pending && !heights[upgrade.Height] {
			continue
		}
		retained = append(retained, upgrade)
	}
	return retained
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
		return fmt.Errorf("marshaling upgrades config: %w", err)
	}

	spec := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-upgrades", chainNode.GetName()),
			Namespace: chainNode.GetNamespace(),
		},
		Data: map[string]string{upgradesConfigFile: string(b)},
	}
	if err := controllerutil.SetControllerReference(chainNode, spec, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
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

func (r *Reconciler) requiresUpgrade(ctx context.Context, chainNode *appsv1.ChainNode) (bool, error) {
	return r.getNodeStatusClient(chainNode).RequiresUpgrade(ctx)
}

func (r *Reconciler) getUpgrade(chainNode *appsv1.ChainNode, height int64) *appsv1.Upgrade {
	for _, upgrade := range chainNode.Status.Upgrades {
		if upgrade.Height == height && (upgrade.Status == appsv1.UpgradeScheduled || upgrade.Status == appsv1.UpgradeOnGoing) {
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

func (r *Reconciler) completeUpgrade(ctx context.Context, chainNode *appsv1.ChainNode, upgrade *appsv1.Upgrade) error {
	if err := r.ensureUpgradeCleanupMarker(ctx, chainNode, upgrade); err != nil {
		return err
	}
	return r.finishUpgradeCleanup(ctx, chainNode)
}

type upgradeCleanupMarker struct {
	Height     int64     `json:"height"`
	Version    string    `json:"version"`
	CooldownAt time.Time `json:"cooldownAt"`
}

func (r *Reconciler) ensureUpgradeCleanupMarker(ctx context.Context, chainNode *appsv1.ChainNode, upgrade *appsv1.Upgrade) error {
	marker := upgradeCleanupMarker{Height: upgrade.Height, Version: upgrade.GetVersion(), CooldownAt: time.Now().UTC()}
	if raw := chainNode.GetAnnotations()[controllers.AnnotationUpgradeCleanup]; raw != "" {
		var existing upgradeCleanupMarker
		if err := json.Unmarshal([]byte(raw), &existing); err != nil {
			return fmt.Errorf("decode upgrade cleanup marker: %w", err)
		}
		if existing.Height != marker.Height || existing.Version != marker.Version || existing.CooldownAt.IsZero() {
			return fmt.Errorf("upgrade cleanup marker does not match target %s at height %d", marker.Version, marker.Height)
		}
		return nil
	}
	body, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	if chainNode.Annotations == nil {
		chainNode.Annotations = map[string]string{}
	}
	chainNode.Annotations[controllers.AnnotationUpgradeCleanup] = string(body)
	if err := r.Update(ctx, chainNode); err != nil {
		delete(chainNode.Annotations, controllers.AnnotationUpgradeCleanup)
		return fmt.Errorf("persist upgrade cleanup marker: %w", err)
	}
	return nil
}

func (r *Reconciler) finishUpgradeCleanup(ctx context.Context, chainNode *appsv1.ChainNode) error {
	raw := chainNode.GetAnnotations()[controllers.AnnotationUpgradeCleanup]
	if raw == "" {
		return nil
	}
	var marker upgradeCleanupMarker
	if err := json.Unmarshal([]byte(raw), &marker); err != nil {
		return fmt.Errorf("decode upgrade cleanup marker: %w", err)
	}
	var upgrade *appsv1.Upgrade
	for i := range chainNode.Status.Upgrades {
		candidate := &chainNode.Status.Upgrades[i]
		if candidate.Height == marker.Height && candidate.GetVersion() == marker.Version &&
			(candidate.Status == appsv1.UpgradeOnGoing || candidate.Status == appsv1.UpgradeCompleted || candidate.Status == appsv1.UpgradeSkipped) {
			upgrade = candidate
			break
		}
	}
	if upgrade == nil || marker.Version == "" || marker.CooldownAt.IsZero() {
		return fmt.Errorf("upgrade cleanup marker has no matching pending or completed upgrade")
	}
	if upgrade.Status != appsv1.UpgradeCompleted {
		chainNode.Status.AppVersion = marker.Version
		if err := r.setUpgradeStatus(ctx, chainNode, upgrade, appsv1.UpgradeCompleted); err != nil {
			return err
		}
	} else if chainNode.Status.AppVersion != marker.Version {
		chainNode.Status.AppVersion = marker.Version
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
		if err := r.ensureUpgradesConfig(ctx, chainNode); err != nil {
			return err
		}
	}
	if err := r.resetVpaAfterUpgradeAt(ctx, chainNode, marker.CooldownAt); err != nil {
		return fmt.Errorf("failed to reset VPA after upgrade for %s: %w", chainNode.GetName(), err)
	}
	delete(chainNode.Annotations, controllers.AnnotationUpgradeCleanup)
	if err := r.Update(ctx, chainNode); err != nil {
		chainNode.Annotations[controllers.AnnotationUpgradeCleanup] = raw
		return fmt.Errorf("clear upgrade cleanup marker: %w", err)
	}
	return nil
}

func (r *Reconciler) getGovUpgrades(ctx context.Context, chainNode *appsv1.ChainNode) ([]appsv1.Upgrade, error) {
	c, err := r.getChainNodeClient(chainNode)
	if err != nil {
		return nil, fmt.Errorf("creating chain client: %w", err)
	}

	plannedUpgrade, err := c.GetNextUpgrade(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying next upgrade: %w", err)
	}

	upgrades := make([]appsv1.Upgrade, 0)
	if plannedUpgrade != nil {
		if plannedUpgrade.Height <= 0 {
			return nil, fmt.Errorf("upgrade plan has invalid height %d", plannedUpgrade.Height)
		}
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
			if u.Source == appsv1.OnChainUpgrade && upgrade.Source == appsv1.OnChainUpgrade &&
				(u.Status == appsv1.UpgradeScheduled || u.Status == appsv1.UpgradeImageMissing) {
				// Governance may replace a plan without changing its height.
				// Use the newly discovered image and readiness, not the old plan's.
				upgrades[i].Image = upgrade.Image
				upgrades[i].Status = upgrade.Status
			}
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
