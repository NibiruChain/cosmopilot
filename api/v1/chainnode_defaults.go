package v1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	defaultPersistenceSize = "50Gi"
	defaultImageVersion    = "latest"
	defaultUnbondingTime   = "1814400s"
	defaultVotingPeriod    = "120h"
	DefaultHDPath          = "m/44'/118'/0'/0/0"
	DefaultAccountPrefix   = "nibi"
	DefaultValPrefix       = "nibivaloper"
)

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
