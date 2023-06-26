package v1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultReconcilePeriod = time.Minute
	DefaultImageVersion    = "latest"

	DefaultUnbondingTime = "1814400s"
	DefaultVotingPeriod  = "120h"
	DefaultHDPath        = "m/44'/118'/0'/0/0"
	DefaultAccountPrefix = "nibi"
	DefaultValPrefix     = "nibivaloper"

	DefaultP2pPort = 26656
)

// GetImage returns the versioned image to be used
func (app *AppSpec) GetImage() string {
	version := DefaultImageVersion
	if app.Version != nil {
		version = *app.Version
	}
	return fmt.Sprintf("%s:%s", app.Image, version)
}

// GetImagePullPolicy returns the pull policy to be used for the app image
func (app *AppSpec) GetImagePullPolicy() corev1.PullPolicy {
	if app.ImagePullPolicy != "" {
		return app.ImagePullPolicy
	}
	if app.Version != nil && *app.Version == DefaultImageVersion {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}

// Peer helper methods

func (peer *Peer) GetPort() int {
	if peer.Port != nil {
		return *peer.Port
	}
	return DefaultP2pPort
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
