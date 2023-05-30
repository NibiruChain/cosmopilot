package v1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	defaultPersistenceSize = "50Gi"
	defaultImageVersion    = "latest"
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
