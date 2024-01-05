package v1

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

const (
	DefaultGroupInstances = 1
)

func (nodeSet *ChainNodeSet) GetNamespacedName() string {
	return types.NamespacedName{Namespace: nodeSet.GetNamespace(), Name: nodeSet.GetName()}.String()
}

func (nodeSet *ChainNodeSet) HasValidator() bool {
	return nodeSet.Spec.Validator != nil
}

func (nodeSet *ChainNodeSet) ShouldInitGenesis() bool {
	return nodeSet.Spec.Validator != nil && nodeSet.Spec.Validator.Init != nil
}

func (nodeSet *ChainNodeSet) GetLastUpgradeVersion() string {
	version := nodeSet.Spec.App.GetImageVersion()
	var h int64 = 0
	for _, u := range nodeSet.Status.Upgrades {
		if (u.Status == UpgradeCompleted || u.Status == UpgradeSkipped) && u.Height > h && u.Height <= nodeSet.Status.LatestHeight {
			h = u.Height
			version = u.GetVersion()
		}
	}
	return version
}

func (nodeSet *ChainNodeSet) GetAppSpecWithUpgrades() AppSpec {
	spec := nodeSet.Spec.App.DeepCopy()

	for _, u := range nodeSet.Status.Upgrades {
		upgradeSpec := UpgradeSpec{
			Height: u.Height,
			Image:  u.Image,
		}
		if !utils.SliceContainsObj(spec.Upgrades, upgradeSpec, func(a UpgradeSpec, b UpgradeSpec) bool {
			return a.Height == b.Height
		}) {
			spec.Upgrades = append(spec.Upgrades, upgradeSpec)
		}
	}

	// Sort upgrades by height
	sort.Slice(nodeSet.Status.Upgrades, func(i, j int) bool {
		return nodeSet.Status.Upgrades[i].Height < nodeSet.Status.Upgrades[j].Height
	})

	return *spec
}

func (nodeSet *ChainNodeSet) RollingUpdatesEnabled() bool {
	if nodeSet.Spec.RollingUpdates != nil {
		return *nodeSet.Spec.RollingUpdates
	}
	return false
}

// Node group methods

func (group *NodeGroupSpec) GetInstances() int {
	if group.Instances != nil {
		return *group.Instances
	}
	return DefaultGroupInstances
}

func (group *NodeGroupSpec) GetIngressSecretName(owner client.Object) string {
	return fmt.Sprintf("%s-%s-tls", owner.GetName(), group.Name)
}

func (group *NodeGroupSpec) GetServiceName(owner client.Object) string {
	return fmt.Sprintf("%s-%s", owner.GetName(), group.Name)
}

// Validator methods

func (val *NodeSetValidatorConfig) GetPrivKeySecretName(obj client.Object) string {
	if val.PrivateKeySecret != nil {
		return *val.PrivateKeySecret
	}
	return fmt.Sprintf("%s-priv-key", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountHDPath() string {
	if val.Init != nil && val.Init.AccountHDPath != nil {
		return *val.Init.AccountHDPath
	}
	return DefaultHDPath
}

func (val *NodeSetValidatorConfig) GetAccountSecretName(obj client.Object) string {
	if val.Init != nil && val.Init.AccountMnemonicSecret != nil {
		return *val.Init.AccountMnemonicSecret
	}

	return fmt.Sprintf("%s-account", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountPrefix() string {
	if val.Init != nil && val.Init.AccountPrefix != nil {
		return *val.Init.AccountPrefix
	}
	return DefaultAccountPrefix
}

func (val *NodeSetValidatorConfig) GetValPrefix() string {
	if val.Init != nil && val.Init.ValPrefix != nil {
		return *val.Init.ValPrefix
	}
	return DefaultValPrefix
}

func (val *NodeSetValidatorConfig) GetInitUnbondingTime() string {
	if val.Init != nil && val.Init.UnbondingTime != nil {
		return *val.Init.UnbondingTime
	}
	return DefaultUnbondingTime
}

func (val *NodeSetValidatorConfig) GetInitVotingPeriod() string {
	if val.Init != nil && val.Init.VotingPeriod != nil {
		return *val.Init.VotingPeriod
	}
	return DefaultVotingPeriod
}
