package v1

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	defaultReconcilePeriod = time.Minute

	defaultPersistenceSize = "50Gi"
	defaultImageVersion    = "latest"
	defaultUnbondingTime   = "1814400s"
	defaultVotingPeriod    = "120h"
	DefaultHDPath          = "m/44'/118'/0'/0/0"
	DefaultAccountPrefix   = "nibi"
	DefaultValPrefix       = "nibivaloper"
	defaultP2pPort         = 26656
	defaultBlockThreshold  = "30s"

	defaultAutoResize          = true
	defaultAutoResizeThreshold = 80
	defaultAutoResizeIncrement = "50Gi"
	defaultAutoResizeMaxSize   = "2Ti"

	defaultP2pExpose      = false
	defaultP2pServiceType = corev1.ServiceTypeNodePort
)

func (chainNode *ChainNode) GetReconcilePeriod() time.Duration {
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.ReconcilePeriod != nil {
		if d, err := time.ParseDuration(*chainNode.Spec.Config.ReconcilePeriod); err == nil {
			return d
		}
	}
	return defaultReconcilePeriod
}

func (chainNode *ChainNode) GetNodeFQDN() string {
	return fmt.Sprintf("%s-headless.%s.svc.cluster.local", chainNode.GetName(), chainNode.GetNamespace())
}

func (chainNode *ChainNode) GetPersistenceSize() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.Size != nil {
		return *chainNode.Spec.Persistence.Size
	}
	return defaultPersistenceSize
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
	return defaultAutoResize
}

func (chainNode *ChainNode) GetPersistenceAutoResizeThreshold() int {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeThreshold != nil {
		return *chainNode.Spec.Persistence.AutoResizeThreshold
	}
	return defaultAutoResizeThreshold
}

func (chainNode *ChainNode) GetPersistenceAutoResizeIncrement() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeIncrement != nil {
		return *chainNode.Spec.Persistence.AutoResizeIncrement
	}
	return defaultAutoResizeIncrement
}

func (chainNode *ChainNode) GetPersistenceAutoResizeMaxSize() string {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AutoResizeMaxSize != nil {
		return *chainNode.Spec.Persistence.AutoResizeMaxSize
	}
	return defaultAutoResizeMaxSize
}

func (chainNode *ChainNode) GetPersistenceInitCommands() []InitCommand {
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.AdditionalInitCommands != nil {
		return chainNode.Spec.Persistence.AdditionalInitCommands
	}
	return []InitCommand{}
}

// GetImage returns the versioned image to be used
func (chainNode *ChainNode) GetImage() string {
	version := defaultImageVersion
	if chainNode.Spec.App.Version != nil {
		version = *chainNode.Spec.App.Version
	}
	return fmt.Sprintf("%s:%s", chainNode.Spec.App.Image, version)
}

// GetImagePullPolicy returns the pull policy to be used for the app image
func (chainNode *ChainNode) GetImagePullPolicy() corev1.PullPolicy {
	if chainNode.Spec.App.ImagePullPolicy != "" {
		return chainNode.Spec.App.ImagePullPolicy
	}
	if chainNode.Spec.App.Version != nil && *chainNode.Spec.App.Version == defaultImageVersion {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
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

			if len(parts) == 1 || parts[1] == defaultImageVersion {
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

func (chainNode *ChainNode) GetValidatorPrivKeySecretName() string {
	if chainNode.Spec.Validator == nil {
		return ""
	}

	if chainNode.Spec.Validator.PrivateKeySecret != nil {
		return *chainNode.Spec.Validator.PrivateKeySecret
	}

	return fmt.Sprintf("%s-priv-key", chainNode.GetName())
}

func (chainNode *ChainNode) GetValidatorAccountHDPath() string {
	if chainNode.Spec.Validator != nil &&
		chainNode.Spec.Validator.Init != nil &&
		chainNode.Spec.Validator.Init.AccountHDPath != nil {
		return *chainNode.Spec.Validator.Init.AccountHDPath
	}
	return DefaultHDPath
}

func (chainNode *ChainNode) GetValidatorAccountSecretName() string {
	if chainNode.Spec.Validator == nil {
		return ""
	}

	if chainNode.Spec.Validator.Init != nil && chainNode.Spec.Validator.Init.AccountMnemonicSecret != nil {
		return *chainNode.Spec.Validator.Init.AccountMnemonicSecret
	}

	return fmt.Sprintf("%s-account", chainNode.GetName())
}

func (chainNode *ChainNode) GetValidatorAccountPrefix() string {
	if chainNode.Spec.Validator != nil &&
		chainNode.Spec.Validator.Init != nil &&
		chainNode.Spec.Validator.Init.AccountPrefix != nil {
		return *chainNode.Spec.Validator.Init.AccountPrefix
	}
	return DefaultAccountPrefix
}

func (chainNode *ChainNode) GetValidatorValPrefix() string {
	if chainNode.Spec.Validator != nil &&
		chainNode.Spec.Validator.Init != nil &&
		chainNode.Spec.Validator.Init.ValPrefix != nil {
		return *chainNode.Spec.Validator.Init.ValPrefix
	}
	return DefaultValPrefix
}

func (chainNode *ChainNode) GetInitUnbondingTime() string {
	if chainNode.Spec.Validator != nil &&
		chainNode.Spec.Validator.Init != nil &&
		chainNode.Spec.Validator.Init.UnbondingTime != nil {
		return *chainNode.Spec.Validator.Init.UnbondingTime
	}
	return defaultUnbondingTime
}

func (chainNode *ChainNode) GetInitVotingPeriod() string {
	if chainNode.Spec.Validator != nil &&
		chainNode.Spec.Validator.Init != nil &&
		chainNode.Spec.Validator.Init.VotingPeriod != nil {
		return *chainNode.Spec.Validator.Init.VotingPeriod
	}
	return defaultVotingPeriod
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
	return defaultBlockThreshold
}

func (chainNode *ChainNode) ExposesP2P() bool {
	if chainNode.Spec.Expose != nil && chainNode.Spec.Expose.P2P != nil {
		return *chainNode.Spec.Expose.P2P
	}
	return defaultP2pExpose
}

func (chainNode *ChainNode) GetP2pServiceType() corev1.ServiceType {
	if chainNode.Spec.Expose != nil && chainNode.Spec.Expose.P2pServiceType != nil {
		return *chainNode.Spec.Expose.P2pServiceType
	}
	return defaultP2pServiceType
}

// Peer helper methods

func (peer *Peer) GetPort() int {
	if peer.Port != nil {
		return *peer.Port
	}
	return defaultP2pPort
}

func (peer *Peer) IsUnconditional() bool {
	if peer.Unconditional != nil {
		return *peer.Unconditional
	}
	return false
}

func (peer *Peer) IsPrivate() bool {
	if peer.Private != nil {
		return *peer.Private
	}
	return false
}
