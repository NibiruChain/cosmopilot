package v1

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultPersistenceSize = "50Gi"
	DefaultBlockThreshold  = "30s"

	DefaultAutoResize          = true
	DefaultAutoResizeThreshold = 80
	DefaultAutoResizeIncrement = "50Gi"
	DefaultAutoResizeMaxSize   = "2Ti"

	DefaultP2pExpose      = false
	DefaultP2pServiceType = corev1.ServiceTypeNodePort
)

func (chainNode *ChainNode) GetReconcilePeriod() time.Duration {
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.ReconcilePeriod != nil {
		if d, err := time.ParseDuration(*chainNode.Spec.Config.ReconcilePeriod); err == nil {
			return d
		}
	}
	return DefaultReconcilePeriod
}

func (chainNode *ChainNode) GetNodeFQDN() string {
	return fmt.Sprintf("%s-headless.%s.svc.cluster.local", chainNode.GetName(), chainNode.GetNamespace())
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

// GetSidecarImagePullPolicy returns the pull policy to be used for the sidecar container image
func (chainNode *ChainNode) GetSidecarImagePullPolicy(name string) corev1.PullPolicy {
	if chainNode.Spec.Config == nil || chainNode.Spec.Config.Sidecars == nil {
		return corev1.PullIfNotPresent
	}

	for _, c := range chainNode.Spec.Config.Sidecars {
		if c.Name == name {
			if c.ImagePullPolicy != "" {
				return c.ImagePullPolicy
			}
			parts := strings.Split(c.Image, ":")

			if len(parts) == 1 || parts[1] == DefaultImageVersion {
				return corev1.PullAlways
			}

			return corev1.PullIfNotPresent
		}
	}
	return corev1.PullIfNotPresent
}

func (chainNode *ChainNode) IsValidator() bool {
	return chainNode.Spec.Validator != nil
}

func (chainNode *ChainNode) ShouldInitGenesis() bool {
	return chainNode.Spec.Validator != nil && chainNode.Spec.Validator.Init != nil
}

func (chainNode *ChainNode) AutoDiscoverPeersEnabled() bool {
	if chainNode.Spec.AutoDiscoverPeers != nil {
		return *chainNode.Spec.AutoDiscoverPeers
	}
	return true
}

func (chainNode *ChainNode) GetBlockThreshold() string {
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.BlockThreshold != nil {
		return *chainNode.Spec.Config.BlockThreshold
	}
	return DefaultBlockThreshold
}

func (chainNode *ChainNode) ExposesP2P() bool {
	if chainNode.Spec.Expose != nil && chainNode.Spec.Expose.P2P != nil {
		return *chainNode.Spec.Expose.P2P
	}
	return DefaultP2pExpose
}

func (chainNode *ChainNode) GetP2pServiceType() corev1.ServiceType {
	if chainNode.Spec.Expose != nil && chainNode.Spec.Expose.P2pServiceType != nil {
		return *chainNode.Spec.Expose.P2pServiceType
	}
	return DefaultP2pServiceType
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
	return DefaultAccountPrefix
}

func (val *ValidatorConfig) GetValPrefix() string {
	if val.Init != nil && val.Init.ValPrefix != nil {
		return *val.Init.ValPrefix
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
