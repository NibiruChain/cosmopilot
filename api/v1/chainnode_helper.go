package v1

import (
	"fmt"
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultPersistenceSize = "50Gi"

	DefaultAutoResize          = true
	DefaultAutoResizeThreshold = 80
	DefaultAutoResizeIncrement = "50Gi"
	DefaultAutoResizeMaxSize   = "2Ti"
)

func (chainNode *ChainNode) Equal(n *ChainNode) bool {
	if !reflect.DeepEqual(chainNode.Labels, n.Labels) {
		return false
	}

	if !reflect.DeepEqual(chainNode.Spec, n.Spec) {
		return false
	}

	return true
}

func (chainNode *ChainNode) GetReconcilePeriod() time.Duration {
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.ReconcilePeriod != nil {
		if d, err := time.ParseDuration(*chainNode.Spec.Config.ReconcilePeriod); err == nil {
			return d
		}
	}
	return DefaultReconcilePeriod
}

func (chainNode *ChainNode) GetNodeFQDN() string {
	return fmt.Sprintf("%s-internal.%s.svc.cluster.local", chainNode.GetName(), chainNode.GetNamespace())
}

func (chainNode *ChainNode) GetPersistenceSize() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.Size != nil {
		return *chainNode.Spec.Persistence.Size
	}
	return DefaultPersistenceSize
}

// GetPersistenceStorageClass returns the configured storage class to be used in pvc, or nil if not specified.
func (chainNode *ChainNode) GetPersistenceStorageClass() *string {
	if chainNode.Spec.Persistence == nil {
		return nil
	}
	return chainNode.Spec.Persistence.StorageClassName
}

func (chainNode *ChainNode) GetPersistenceAutoResizeEnabled() bool {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResize != nil {
		return *chainNode.Spec.Persistence.AutoResize
	}
	return DefaultAutoResize
}

func (chainNode *ChainNode) GetPersistenceAutoResizeThreshold() int {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeThreshold != nil {
		return *chainNode.Spec.Persistence.AutoResizeThreshold
	}
	return DefaultAutoResizeThreshold
}

func (chainNode *ChainNode) GetPersistenceAutoResizeIncrement() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeIncrement != nil {
		return *chainNode.Spec.Persistence.AutoResizeIncrement
	}
	return DefaultAutoResizeIncrement
}

func (chainNode *ChainNode) GetPersistenceAutoResizeMaxSize() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeMaxSize != nil {
		return *chainNode.Spec.Persistence.AutoResizeMaxSize
	}
	return DefaultAutoResizeMaxSize
}

func (chainNode *ChainNode) GetPersistenceInitCommands() []InitCommand {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AdditionalInitCommands != nil {
		return chainNode.Spec.Persistence.AdditionalInitCommands
	}
	return []InitCommand{}
}

func (chainNode *ChainNode) SnapshotsEnabled() bool {
	return chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.Snapshots != nil
}

func (chainNode *ChainNode) ShouldRestoreFromSnapshot() bool {
	return chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.RestoreFromSnapshot != nil
}

func (chainNode *ChainNode) IsValidator() bool {
	return chainNode.Spec.Validator != nil
}

func (chainNode *ChainNode) ShouldInitGenesis() bool {
	return chainNode.Spec.Validator != nil && chainNode.Spec.Validator.Init != nil
}

func (chainNode *ChainNode) UsesTmKms() bool {
	return chainNode.Spec.Validator != nil && chainNode.Spec.Validator.TmKMS != nil
}

func (chainNode *ChainNode) ShouldUploadVaultKey() bool {
	if chainNode.ShouldInitGenesis() {
		return true
	}

	if chainNode.Spec.Validator.TmKMS != nil && chainNode.Spec.Validator.TmKMS.Provider.Vault != nil {
		return chainNode.Spec.Validator.TmKMS.Provider.Vault.UploadGenerated
	}

	return false
}

func (chainNode *ChainNode) ShouldCreateValidator() bool {
	return chainNode.Spec.Validator != nil && chainNode.Spec.Validator.CreateValidator != nil
}

func (chainNode *ChainNode) RequiresPrivKey() bool {
	if !chainNode.IsValidator() {
		return false
	}

	if chainNode.Status.PubKey == "" && chainNode.ShouldInitGenesis() {
		// For key upload when we are initializing a chain
		if chainNode.Spec.Validator.TmKMS != nil && chainNode.Spec.Validator.TmKMS.Provider.Vault != nil {
			chainNode.Spec.Validator.TmKMS.Provider.Vault.UploadGenerated = true
		}
		return true
	}

	if chainNode.Status.PubKey == "" && chainNode.ShouldCreateValidator() {
		return true
	}

	return false
}

func (chainNode *ChainNode) RequiresAccount() bool {
	// If we already have an account lets ignore this
	if chainNode.Status.AccountAddress != "" {
		return false
	}

	if !chainNode.IsValidator() {
		return false
	}

	if chainNode.ShouldInitGenesis() {
		return true
	}

	if chainNode.ShouldCreateValidator() {
		return true
	}

	return false
}

func (chainNode *ChainNode) AutoDiscoverPeersEnabled() bool {
	if chainNode.Spec.AutoDiscoverPeers != nil {
		return *chainNode.Spec.AutoDiscoverPeers
	}
	return true
}

func (chainNode *ChainNode) StateSyncRestoreEnabled() bool {
	if chainNode.Spec.StateSyncRestore != nil {
		return *chainNode.Spec.StateSyncRestore
	}
	return false
}

func (chainNode *ChainNode) GetMoniker() string {
	if chainNode.IsValidator() && chainNode.Spec.Validator.Info != nil && chainNode.Spec.Validator.Info.Moniker != nil {
		return *chainNode.Spec.Validator.Info.Moniker
	}
	return chainNode.GetName()
}

func (chainNode *ChainNode) HasCompletedUpgrades() bool {
	for _, upgrade := range chainNode.Status.Upgrades {
		if upgrade.Status == UpgradeCompleted {
			return true
		}
	}
	return false
}

func (chainNode *ChainNode) GetAppVersion() string {
	if chainNode.HasCompletedUpgrades() {
		return chainNode.Status.AppVersion
	}
	return chainNode.Spec.App.GetImageVersion()
}

func (chainNode *ChainNode) GetAppImage() string {
	return fmt.Sprintf("%s:%s", chainNode.Spec.App.Image, chainNode.GetAppVersion())
}

// Validator methods

func (val *ValidatorConfig) GetPrivKeySecretName(obj client.Object) string {
	if val.PrivateKeySecret != nil {
		return *val.PrivateKeySecret
	}
	return fmt.Sprintf("%s-priv-key", obj.GetName())
}

func (val *ValidatorConfig) GetAccountHDPath() string {
	if val.Init != nil && val.Init.AccountHDPath != nil {
		return *val.Init.AccountHDPath
	}
	if val.CreateValidator != nil && val.CreateValidator.AccountHDPath != nil {
		return *val.CreateValidator.AccountHDPath
	}
	return DefaultHDPath
}

func (val *ValidatorConfig) GetAccountSecretName(obj client.Object) string {
	if val.Init != nil && val.Init.AccountMnemonicSecret != nil {
		return *val.Init.AccountMnemonicSecret
	}

	return fmt.Sprintf("%s-account", obj.GetName())
}

func (val *ValidatorConfig) GetAccountPrefix() string {
	if val.Init != nil && val.Init.AccountPrefix != nil {
		return *val.Init.AccountPrefix
	}
	if val.CreateValidator != nil && val.CreateValidator.AccountPrefix != nil {
		return *val.CreateValidator.AccountPrefix
	}
	return DefaultAccountPrefix
}

func (val *ValidatorConfig) GetValPrefix() string {
	if val.Init != nil && val.Init.ValPrefix != nil {
		return *val.Init.ValPrefix
	}
	if val.CreateValidator != nil && val.CreateValidator.ValPrefix != nil {
		return *val.CreateValidator.ValPrefix
	}
	return DefaultValPrefix
}

func (val *ValidatorConfig) GetInitUnbondingTime() string {
	if val.Init != nil && val.Init.UnbondingTime != nil {
		return *val.Init.UnbondingTime
	}
	return DefaultUnbondingTime
}

func (val *ValidatorConfig) GetInitVotingPeriod() string {
	if val.Init != nil && val.Init.VotingPeriod != nil {
		return *val.Init.VotingPeriod
	}
	return DefaultVotingPeriod
}

func (val *ValidatorConfig) GetCommissionMaxChangeRate() string {
	if val.Init != nil && val.Init.CommissionMaxChangeRate != nil {
		return *val.Init.CommissionMaxChangeRate
	}
	if val.CreateValidator != nil && val.CreateValidator.CommissionMaxChangeRate != nil {
		return *val.CreateValidator.CommissionMaxChangeRate
	}
	return DefaultCommissionMaxChangeRate
}

func (val *ValidatorConfig) GetCommissionMaxRate() string {
	if val.Init != nil && val.Init.CommissionMaxRate != nil {
		return *val.Init.CommissionMaxRate
	}
	if val.CreateValidator != nil && val.CreateValidator.CommissionMaxRate != nil {
		return *val.CreateValidator.CommissionMaxRate
	}
	return DefaultCommissionMaxRate
}

func (val *ValidatorConfig) GetCommissionRate() string {
	if val.Init != nil && val.Init.CommissionRate != nil {
		return *val.Init.CommissionRate
	}
	if val.CreateValidator != nil && val.CreateValidator.CommissionRate != nil {
		return *val.CreateValidator.CommissionRate
	}
	return DefaultCommissionRate
}

func (val *ValidatorConfig) GetMinSelfDelegation() string {
	if val.Init != nil && val.Init.MinSelfDelegation != nil {
		return *val.Init.MinSelfDelegation
	}
	if val.CreateValidator != nil && val.CreateValidator.MinSelfDelegation != nil {
		return *val.CreateValidator.MinSelfDelegation
	}
	return DefaultMinimumSelfDelegation
}
