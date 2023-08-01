package v1

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/NibiruChain/nibiru-operator/internal/tmkms"
)

const (
	DefaultReconcilePeriod     = time.Minute
	DefaultImageVersion        = "latest"
	DefaultBlockThreshold      = "30s"
	DefaultP2pExpose           = false
	DefaultP2pServiceType      = corev1.ServiceTypeNodePort
	DefaultUnbondingTime       = "1814400s"
	DefaultVotingPeriod        = "120h"
	DefaultHDPath              = "m/44'/118'/0'/0/0"
	DefaultAccountPrefix       = "nibi"
	DefaultValPrefix           = "nibivaloper"
	DefaultP2pPort             = 26656
	DefaultStateSyncKeepRecent = 2
	DefaultSdkVersion          = V0_47
)

// GetImage returns the versioned image to be used
func (app *AppSpec) GetImage() string {
	return fmt.Sprintf("%s:%s", app.Image, app.GetImageVersion())
}

// GetImageVersion returns the image version to be used
func (app *AppSpec) GetImageVersion() string {
	if app.Version != nil {
		return *app.Version
	}
	return DefaultImageVersion
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

func (app *AppSpec) GetSdkVersion() SdkVersion {
	if app.SdkVersion != nil {
		return *app.SdkVersion
	}
	return DefaultSdkVersion
}

// GetSidecarImagePullPolicy returns the pull policy to be used for the sidecar container image
func (cfg *Config) GetSidecarImagePullPolicy(name string) corev1.PullPolicy {
	if cfg == nil || cfg.Sidecars == nil {
		return corev1.PullIfNotPresent
	}

	for _, c := range cfg.Sidecars {
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

func (cfg *Config) SeedModeEnabled() bool {
	if cfg != nil && cfg.SeedMode != nil {
		return *cfg.SeedMode
	}
	return false
}

func (cfg *Config) GetEnv() []corev1.EnvVar {
	if cfg != nil && cfg.Env != nil {
		return cfg.Env
	}
	return []corev1.EnvVar{}
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

func (cfg *Config) GetBlockThreshold() string {
	if cfg != nil && cfg.BlockThreshold != nil {
		return *cfg.BlockThreshold
	}
	return DefaultBlockThreshold
}

func (exp *ExposeConfig) Enabled() bool {
	if exp != nil && exp.P2P != nil {
		return *exp.P2P
	}
	return DefaultP2pExpose
}

func (exp *ExposeConfig) GetServiceType() corev1.ServiceType {
	if exp != nil && exp.P2pServiceType != nil {
		return *exp.P2pServiceType
	}
	return DefaultP2pServiceType
}

// TmKMS helper methods

func (kms *TmKMS) GetKeyFormat() *TmKmsKeyFormat {
	if kms.KeyFormat != nil {
		return kms.KeyFormat
	}
	return &TmKmsKeyFormat{
		Type:               tmkms.DefaultKeyType,
		AccountKeyPrefix:   tmkms.DefaultAccountPrefix,
		ConsensusKeyPrefix: tmkms.DefaultConsensusPrefix,
	}
}

func (kms *TmKMS) GetProtocolVersion() tmkms.ProtocolVersion {
	if kms.ValidatorProtocol != nil {
		return *kms.ValidatorProtocol
	}
	return tmkms.ProtocolVersionV0_34
}

// StateSync methods

func (ss *StateSyncConfig) Enabled() bool {
	return ss != nil && ss.SnapshotInterval > 0
}

func (ss *StateSyncConfig) GetKeepRecent() int {
	if ss != nil && ss.SnapshotKeepRecent != nil {
		return *ss.SnapshotKeepRecent
	}
	return DefaultStateSyncKeepRecent
}

// GetGenesisFromRPCUrl transforms struct into url string

func (gg *FromNodeRPCConfig) GetGenesisFromRPCUrl() string {
	if gg == nil {
		return ""
	}

	port := 26657
	protocol := "http://"
	hostname := gg.Hostname

	if gg.Secure {
		protocol = "https://"
		port = 443
	}

	if gg.Port != nil {
		port = *gg.Port
	}

	return fmt.Sprintf("%s%s:%d/genesis", protocol, hostname, port)
}
