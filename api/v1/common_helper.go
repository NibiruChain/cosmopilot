package v1

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kube-openapi/pkg/validation/strfmt"

	"github.com/NibiruChain/cosmopilot/internal/tmkms"
	"github.com/NibiruChain/cosmopilot/pkg/dataexporter"
)

const (
	DefaultReconcilePeriod         = 15 * time.Second
	DefaultImageVersion            = "latest"
	DefaultBlockThreshold          = "15s"
	DefaultStartupTime             = time.Hour
	DefaultNodeUtilsLogLevel       = "info"
	DefaultP2pExpose               = false
	DefaultP2pServiceType          = corev1.ServiceTypeNodePort
	DefaultUnbondingTime           = "1814400s"
	DefaultVotingPeriod            = "120h"
	DefaultHDPath                  = "m/44'/118'/0'/0/0"
	DefaultAccountPrefix           = "nibi"
	DefaultValPrefix               = "nibivaloper"
	DefaultP2pPort                 = 26656
	DefaultStateSyncKeepRecent     = 2
	DefaultSdkVersion              = V0_47
	DefaultCommissionMaxChangeRate = "0.1"
	DefaultCommissionMaxRate       = "0.1"
	DefaultCommissionRate          = "0.1"
	DefaultMinimumSelfDelegation   = "1"
	DefaultNodeUtilsCPU            = "300m"
	DefaultNodeUtilsMemory         = "100Mi"
	DefaultCosmoGuardCPU           = "200m"
	DefaultCosmoGuardMemory        = "250Mi"
	DefaultVpaCooldown             = 5 * time.Minute
	DefaultLimitPercentage         = 150
)

var (
	defaultServiceMonitorSelector = map[string]string{
		"release": "monitoring-stack",
	}
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

func (app *AppSpec) ShouldQueryGovUpgrades() bool {
	if app.CheckGovUpgrades != nil {
		return *app.CheckGovUpgrades
	}
	return true
}

// GenesisConfig helper methods

func (g *GenesisConfig) ShouldUseDataVolume() bool {
	if g != nil && g.UseDataVolume != nil {
		return *g.UseDataVolume
	}
	return false
}

func (g *GenesisConfig) ShouldDownloadUsingContainer() bool {
	if g != nil && g.ChainID != nil {
		return g.ShouldUseDataVolume()
	}
	return false
}

func (g *GenesisConfig) HasConfigMapSource() bool {
	if g != nil && g.ConfigMap != nil {
		return true
	}
	return false
}

func (g *GenesisConfig) GetConfigMapName(chainID string) string {
	if g != nil && g.ConfigMap != nil {
		return *g.ConfigMap
	}
	return fmt.Sprintf("%s-genesis", chainID)
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
			if c.Image != nil {
				parts := strings.Split(*c.Image, ":")
				if len(parts) == 1 || parts[1] == DefaultImageVersion {
					return corev1.PullAlways
				}
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

func (cfg *Config) ShouldIgnoreSyncing() bool {
	if cfg != nil && cfg.IgnoreSyncing != nil {
		return *cfg.IgnoreSyncing
	}
	return false
}

func (cfg *Config) GetEnv() []corev1.EnvVar {
	if cfg != nil && cfg.Env != nil {
		return cfg.Env
	}
	return []corev1.EnvVar{}
}

func (cfg *Config) GetNodeUtilsResources() corev1.ResourceRequirements {
	if cfg != nil && cfg.NodeUtilsResources != nil {
		return *cfg.NodeUtilsResources
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultNodeUtilsCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultNodeUtilsMemory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultNodeUtilsCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultNodeUtilsMemory),
		},
	}
}

func (cfg *Config) IsEvmEnabled() bool {
	if cfg != nil && cfg.EvmEnabled != nil {
		return *cfg.EvmEnabled
	}
	return false
}

func (cfg *Config) UseDashedConfigToml() bool {
	if cfg != nil && cfg.DashedConfigToml != nil {
		return *cfg.DashedConfigToml
	}
	return false
}

func (cfg *Config) GetBlockThreshold() string {
	if cfg != nil && cfg.BlockThreshold != nil {
		return *cfg.BlockThreshold
	}
	return DefaultBlockThreshold
}

func (cfg *Config) GetStartupTime() time.Duration {
	if cfg != nil && cfg.StartupTime != nil {
		if d, err := strfmt.ParseDuration(*cfg.StartupTime); err == nil {
			return d
		}
	}
	return DefaultStartupTime
}

func (cfg *Config) GetNodeUtilsLogLevel() string {
	if cfg != nil && cfg.NodeUtilsLogLevel != nil {
		return *cfg.NodeUtilsLogLevel
	}
	return DefaultNodeUtilsLogLevel
}

func (cfg *Config) ShouldPersistAddressBook() bool {
	if cfg != nil && cfg.PersistAddressBook != nil {
		return *cfg.PersistAddressBook
	}
	return false
}

func (cfg *Config) GetTerminationGracePeriodSeconds() *int64 {
	if cfg != nil {
		return cfg.TerminationGracePeriodSeconds
	}
	return nil
}

func (cfg *Config) CosmoGuardEnabled() bool {
	if cfg != nil && cfg.CosmoGuard != nil {
		return cfg.CosmoGuard.Enable
	}
	return false
}

func (cfg *Config) GetCosmoGuardConfig() *corev1.ConfigMapKeySelector {
	if cfg != nil && cfg.CosmoGuard != nil {
		return cfg.CosmoGuard.Config
	}
	return nil
}

func (cfg *Config) ShouldRestartPodOnCosmoGuardFailure() bool {
	if cfg == nil {
		return false
	}
	if cfg.CosmoGuard != nil && cfg.CosmoGuard.RestartPodOnFailure != nil {
		return *cfg.CosmoGuard.RestartPodOnFailure
	}
	return false
}

func (cfg *Config) GetCosmoGuardResources() corev1.ResourceRequirements {
	if cfg != nil && cfg.CosmoGuard != nil && cfg.CosmoGuard.Resources != nil {
		return *cfg.CosmoGuard.Resources
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultCosmoGuardCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultCosmoGuardMemory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultCosmoGuardCPU),
			corev1.ResourceMemory: resource.MustParse(DefaultCosmoGuardMemory),
		},
	}
}

func (cfg *Config) GetHaltHeight() int64 {
	if cfg != nil && cfg.HaltHeight != nil {
		return *cfg.HaltHeight
	}
	return 0
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

func (exp *ExposeConfig) GetAnnotations() map[string]string {
	if exp != nil && exp.Annotations != nil {
		return exp.Annotations
	}
	return nil
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

func (kms *TmKMS) ShouldPersistState() bool {
	if kms != nil && kms.PersistState != nil {
		return *kms.PersistState
	}
	return true
}

func (kms *TmKMS) GetResources() corev1.ResourceRequirements {
	if kms != nil && kms.Resources != nil {
		return *kms.Resources
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(tmkms.DefaultTmkmsCpu),
			corev1.ResourceMemory: resource.MustParse(tmkms.DefaultTmkmsMemory),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(tmkms.DefaultTmkmsCpu),
			corev1.ResourceMemory: resource.MustParse(tmkms.DefaultTmkmsMemory),
		},
	}
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

// VolumeSnapshotsConfig helper methods

func (s *VolumeSnapshotsConfig) ShouldStopNode() bool {
	if s != nil && s.StopNode != nil {
		return *s.StopNode
	}
	return false
}

func (s *VolumeSnapshotsConfig) ShouldExportTarballs() bool {
	if s != nil && s.ExportTarball != nil {
		return true
	}
	return false
}

func (s *VolumeSnapshotsConfig) ShouldVerify() bool {
	if s != nil && s.Verify != nil {
		return *s.Verify
	}
	return false
}

func (s *VolumeSnapshotsConfig) ShouldDisableWhileSyncing() bool {
	if s != nil && s.DisableWhileSyncing != nil {
		return *s.DisableWhileSyncing
	}
	return true
}

// ExportTarballConfig helper methods

func (e *ExportTarballConfig) GetSuffix() string {
	if e != nil && e.Suffix != nil {
		return *e.Suffix
	}
	return ""
}

func (e *ExportTarballConfig) DeleteWhenExpired() bool {
	if e != nil && e.DeleteOnExpire != nil {
		return *e.DeleteOnExpire
	}
	return false
}

// GcsExporter helper methods

func (gcs *GcsExportConfig) GetSizeLimit() string {
	if gcs != nil && gcs.SizeLimit != nil {
		return *gcs.SizeLimit
	}
	return dataexporter.DefaultSizeLimit
}

func (gcs *GcsExportConfig) GetPartSize() string {
	if gcs != nil && gcs.PartSize != nil {
		return *gcs.PartSize
	}
	return dataexporter.DefaultPartSize
}

func (gcs *GcsExportConfig) GetChunkSize() string {
	if gcs != nil && gcs.ChunkSize != nil {
		return *gcs.ChunkSize
	}
	return dataexporter.DefaultChunkSize
}

func (gcs *GcsExportConfig) GetBufferSize() string {
	if gcs != nil && gcs.BufferSize != nil {
		return *gcs.BufferSize
	}
	return dataexporter.DefaultBufferSize
}

func (gcs *GcsExportConfig) GetConcurrentJobs() int {
	if gcs != nil && gcs.ConcurrentJobs != nil {
		return *gcs.ConcurrentJobs
	}
	return dataexporter.DefaultConcurrentJobs
}

// Upgrade helper methods

func (u *UpgradeSpec) GetVersion() string {
	if parts := strings.Split(u.Image, ":"); len(parts) == 2 {
		return parts[1]
	}
	return DefaultImageVersion
}

func (u *UpgradeSpec) ForceGovUpgrade() bool {
	if u != nil && u.ForceOnChain != nil {
		return *u.ForceOnChain
	}
	return false
}

func (u *Upgrade) GetVersion() string {
	if parts := strings.Split(u.Image, ":"); len(parts) == 2 {
		return parts[1]
	}
	return DefaultImageVersion
}

// Sidecar helper methods

func (s *SidecarSpec) ShouldRestartPodOnFailure() bool {
	if s.RestartPodOnFailure != nil {
		return *s.RestartPodOnFailure
	}
	return false
}

func (s *SidecarSpec) ShouldRunBeforeNode() bool {
	if s.RunBeforeNode != nil {
		return *s.RunBeforeNode
	}
	return false
}

func (s *SidecarSpec) GetImage(chainNode *ChainNode) string {
	if s.Image != nil {
		return *s.Image
	}
	return chainNode.GetAppImage()
}

// VolumeSpec methods

func (v *VolumeSpec) ShouldDeleteWithNode() bool {
	if v.DeleteWithNode != nil {
		return *v.DeleteWithNode
	}
	return false
}

// Vertical Pod Autoscaling

func (vpa *VerticalAutoscalingConfig) IsEnabled() bool {
	return vpa != nil && vpa.Enabled
}

func (vpam *VerticalAutoscalingMetricConfig) GetCooldownDuration() time.Duration {
	if vpam != nil && vpam.Cooldown != nil {
		if d, err := strfmt.ParseDuration(*vpam.Cooldown); err == nil {
			return d
		}
	}
	return DefaultVpaCooldown
}

func (vpam *VerticalAutoscalingMetricConfig) GetSource() LimitSource {
	if vpam != nil && vpam.Source != nil {
		return *vpam.Source
	}
	return EffectiveLimit
}

func (vpam *VerticalAutoscalingMetricConfig) GetLimitUpdateStrategy() LimitUpdateStrategy {
	if vpam != nil && vpam.LimitStrategy != nil {
		return *vpam.LimitStrategy
	}
	return LimitRetain
}

func (vpam *VerticalAutoscalingMetricConfig) GetLimitPercentage() int {
	if vpam != nil && vpam.LimitPercentage != nil {
		return *vpam.LimitPercentage
	}
	return DefaultLimitPercentage
}

func (vpar *VerticalAutoscalingRule) GetDuration() time.Duration {
	if vpar != nil && vpar.Duration != nil {
		if d, err := strfmt.ParseDuration(*vpar.Duration); err == nil {
			return d
		}
	}
	return DefaultVpaCooldown
}

// Peer helper methods

func (peer *Peer) String() string {
	if peer == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s@%s:%d", peer.ID, peer.Address, peer.GetPort())
}

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

func (peer *Peer) IsSeed() bool {
	if peer.Seed != nil {
		return *peer.Seed
	}
	return false
}

func (p PeerList) String() string {
	stringPeers := make([]string, len(p))
	for i, peer := range p {
		stringPeers[i] = peer.String()
	}
	return strings.Join(stringPeers, ",")
}

func (p PeerList) ExcludeSeeds() PeerList {
	l := make(PeerList, 0)
	for _, peer := range p {
		if !peer.IsSeed() {
			l = append(l, peer)
		}
	}
	return l
}
