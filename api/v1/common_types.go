package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/NibiruChain/nibiru-operator/internal/tmkms"
)

// Reasons for events
const (
	ReasonPvcResized             = "PvcResized"
	ReasonPvcMaxReached          = "PvcMaxSizeReached"
	ReasonDataInitialized        = "DataInitialized"
	ReasonNodeKeyCreated         = "NodeKeyCreated"
	ReasonNodeKeyImported        = "NodeKeyImported"
	ReasonPrivateKeyCreated      = "PrivateKeyCreated"
	ReasonPrivateKeyImported     = "PrivateKeyImported"
	ReasonAccountCreated         = "AccountCreated"
	ReasonAccountImported        = "AccountImported"
	ReasonGenesisInitialized     = "GenesisCreated"
	ReasonGenesisImported        = "GenesisImported"
	ReasonConfigsCreated         = "ConfigsCreated"
	ReasonConfigsUpdated         = "ConfigsUpdated"
	ReasonNodeStarted            = "NodeStarted"
	ReasonNodeRestarted          = "NodeRestarted"
	ReasonNodeError              = "NodeError"
	ReasonNodeSyncing            = "NodeSyncing"
	ReasonNodeRunning            = "NodeRunning"
	ReasonValidatorJailed        = "ValidatorJailed"
	ReasonValidatorUnjailed      = "ValidatorUnjailed"
	ReasonNodeCreated            = "NodeCreated"
	ReasonNodeUpdated            = "NodeUpdated"
	ReasonNodeDeleted            = "NodeDeleted"
	ReasonInitGenesisFailure     = "InitGenesisFail"
	ReasonUploadFailure          = "UploadFailed"
	ReasonGenesisWrongHash       = "GenesisWrongHash"
	ReasonNoTrustHeight          = "NoTrustHeight"
	ReasonNoPeers                = "NoPeers"
	ReasonStartedSnapshot        = "SnapshotStarted"
	ReasonFinishedSnapshot       = "SnapshotFinished"
	ReasonDeletedSnapshot        = "SnapshotDeleted"
	ReasonTarballExportStart     = "ExportingTarball"
	ReasonTarballExportFinish    = "TarballFinished"
	ReasonTarballDeleted         = "TarballDeleted"
	ReasonTarballExportError     = "TarballExportError"
	ReasonUpgradeCompleted       = "UpgradeCompleted"
	ReasonUpgradeFailed          = "UpgradeFailed"
	ReasonUpgradeMissingData     = "UpgradeMissingData"
	ReasonCreateValidatorFailure = "FailedCreateValidator"
	ReasonCreateValidatorSuccess = "CreateValidatorSuccess"
)

// SdkVersion specifies the cosmos-sdk version.
// +kubebuilder:validation:Enum=v0.45;v0.47
type SdkVersion string

const (
	V0_47 SdkVersion = "v0.47"
	V0_45 SdkVersion = "v0.45"
)

type ValidatorStatus string

const (
	ValidatorStatusBonded    = "bonded"
	ValidatorStatusUnbonded  = "unbonded"
	ValidatorStatusUnbonding = "unbonding"
	ValidatorStatusUnknown   = "unknown"
)

// AppSpec specifies the source image and binary name of the app to run
type AppSpec struct {
	// Image indicates the docker image to be used
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Version is the image tag to be used. Defaults to `latest`.
	// +optional
	// +default=latest
	Version *string `json:"version,omitempty"`

	// ImagePullPolicy indicates the desired pull policy when creating nodes. Defaults to `Always` if `version`
	// is `latest` and `IfNotPresent` otherwise.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// App is the name of the binary of the application to be run
	App string `json:"app"`

	// SdkVersion specifies the version of cosmos-sdk used by this app. Defaults to `v0.47`.
	// +optional
	// +default=v0.47
	SdkVersion *SdkVersion `json:"sdkVersion,omitempty"`

	// CheckGovUpgrades indicates that operator should query gov proposals to find and schedule upgrades.
	// Defaults to `true`.
	// +optional
	// +default=true
	CheckGovUpgrades *bool `json:"checkGovUpgrades,omitempty"`

	// Upgrades contains manually scheduled upgrades
	// +optional
	Upgrades []UpgradeSpec `json:"upgrades,omitempty"`
}

// Config allows setting specific configurations for a chainnode such as overrides to app.toml and config.toml
type Config struct {
	// Override allows overriding configs on toml configuration files
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Override *map[string]runtime.RawExtension `json:"override,omitempty"`

	// Sidecars allow configuring additional containers to run alongside the node
	// +optional
	Sidecars []SidecarSpec `json:"sidecars,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this node.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// BlockThreshold specifies the time to wait for a block before considering node unhealthy.
	// Defaults to `15s`.
	// +optional
	// +default=15s
	BlockThreshold *string `json:"blockThreshold,omitempty"`

	// ReconcilePeriod is the period at which a reconcile loop will happen for this ChainNode.
	// Defaults to `15s`.
	// +optional
	// +default=15s
	ReconcilePeriod *string `json:"reconcilePeriod,omitempty"`

	// StateSync configures statesync snapshots for this node.
	// +optional
	StateSync *StateSyncConfig `json:"stateSync,omitempty"`

	// SeedMode configures this node to run on seed mode. Defaults to `false`.
	// +optional
	SeedMode *bool `json:"seedMode,omitempty"`

	// Env refers to the list of environment variables to set in the app container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// SafeToEvict sets cluster-autoscaler.kubernetes.io/safe-to-evict annotation to the given value. It allows/disallows
	// cluster-autoscaler to evict this node's pod.
	// +optional
	SafeToEvict *bool `json:"safeToEvict,omitempty"`

	// ServiceMonitor allows deploying prometheus service monitor for this node.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

type ServiceMonitorSpec struct {
	// Enable indicates a service monitor should be deployed for this node.
	Enable bool `json:"enable"`

	// Selector indicates the prometheus installation that will be using this service monitor.
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

// SidecarSpec allow configuring additional containers to run alongside the node
type SidecarSpec struct {
	// Name refers to the name to be assigned to the container
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Image refers to the docker image to be used by the container
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// ImagePullPolicy indicates the desired pull policy when creating nodes. Defaults to `Always` if `version`
	// is `latest` and `IfNotPresent` otherwise.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// MountDataVolume indicates where data volume will be mounted on this container. It is not mounted if not specified.
	// +optional
	MountDataVolume *string `json:"mountDataVolume,omitempty"`

	// Command to be run by this container. Defaults to entrypoint defined in image.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args to be passed to this container. Defaults to cmd defined in image.
	// +optional
	Args []string `json:"args,omitempty"`

	// Env sets environment variables to be passed to this container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// SecurityContext defines the security options the container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext, which defaults to
	// user ID 1000.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Compute Resources required by the sidecar container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ValidatorInfo contains information about this validator.
type ValidatorInfo struct {
	// Moniker to be used by this validator. Defaults to the ChainNode name.
	// +optional
	Moniker *string `json:"moniker,omitempty"`

	// Details of this validator.
	// +optional
	Details *string `json:"details,omitempty"`

	// Website indicates this validator's website.
	// +optional
	Website *string `json:"website,omitempty"`

	// Identity signature of this validator.
	// +optional
	Identity *string `json:"identity,omitempty"`
}

// GenesisInitConfig specifies configs and initialization commands for creating a new chain and its genesis
type GenesisInitConfig struct {
	// ChainID of the chain to initialize.
	ChainID string `json:"chainID"`

	// AccountMnemonicSecret is the name of the secret containing the mnemonic of the account to be used by
	// this validator. Defaults to `<chainnode>-account`. Will be created if does not exist.
	AccountMnemonicSecret *string `json:"accountMnemonicSecret,omitempty"`

	// AccountHDPath is the HD path for the validator account. Defaults to `m/44'/118'/0'/0/0`.
	// +optional
	AccountHDPath *string `json:"accountHDPath,omitempty"`

	// AccountPrefix is the prefix for accounts. Defaults to `nibi`.
	// +optional
	AccountPrefix *string `json:"accountPrefix,omitempty"`

	// ValPrefix is the prefix for validator accounts. Defaults to `nibivaloper`.
	// +optional
	ValPrefix *string `json:"valPrefix,omitempty"`

	// CommissionMaxChangeRate is the maximum commission change rate percentage (per day). Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxChangeRate *string `json:"commissionMaxChangeRate,omitempty"`

	// CommissionMaxRate is the maximum commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxRate *string `json:"commissionMaxRate,omitempty"`

	// CommissionRate is the initial commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionRate *string `json:"commissionRate,omitempty"`

	// MinSelfDelegation is the minimum self delegation required on the validator. Defaults to `1`.
	// +optional
	// +default="0.1"
	MinSelfDelegation *string `json:"minSelfDelegation,omitempty"`

	// Assets is the list of tokens and their amounts to be assigned to this validators account.
	Assets []string `json:"assets"`

	// StakeAmount represents the amount to be staked by this validator.
	StakeAmount string `json:"stakeAmount"`

	// Accounts specify additional accounts and respective assets to be added to this chain.
	// +optional
	Accounts []AccountAssets `json:"accounts,omitempty"`

	// UnbondingTime is the time that takes to unbond delegations. Defaults to `1814400s`.
	// +optional
	UnbondingTime *string `json:"unbondingTime,omitempty"`

	// VotingPeriod indicates the voting period for this chain. Defaults to `120h`.
	// +optional
	VotingPeriod *string `json:"votingPeriod,omitempty"`

	// AdditionalInitCommands are additional commands to run on genesis initialization.
	// App home is at `/home/app` and `/temp` is a temporary volume shared by all init containers.
	// +optional
	AdditionalInitCommands []InitCommand `json:"additionalInitCommands,omitempty"`
}

type AccountAssets struct {
	// Address of the account.
	Address string `json:"address"`

	// Assets to be assigned to this account.
	Assets []string `json:"assets"`
}

type InitCommand struct {
	// Image to be used to run this command. Defaults to app image.
	// +optional
	Image *string `json:"image,omitempty"`

	// Command to be used. Defaults to image entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args to be passed to this command.
	Args []string `json:"args"`
}

// GenesisConfig specifies how genesis will be retrieved
type GenesisConfig struct {
	// URL to download the genesis from.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Url *string `json:"url,omitempty"`

	// Get the genesis from the existing node RPC endpoint.
	// +optional
	FromNodeRPC *FromNodeRPCConfig `json:"fromNodeRPC,omitempty"`

	// GenesisSHA is the 256 SHA to validate the genesis.
	// +optional
	GenesisSHA *string `json:"genesisSHA,omitempty"`

	// ConfigMap specifies a configmap to load the genesis from
	// +optional
	ConfigMap *string `json:"configMap,omitempty"`

	// UseDataVolume indicates that the operator should save the genesis in the same volume as node data
	// instead of a ConfigMap. This is useful for genesis whose size is bigger than ConfigMap limit of 1MiB.
	// +optional
	UseDataVolume *bool `json:"useDataVolume,omitempty"`
}

type Peer struct {
	// ID refers to tendermint node ID for this node
	ID string `json:"id"`

	// Address is the hostname or IP address of this peer
	Address string `json:"address"`

	// Port is the P2P port to be used. Defaults to `26656`.
	// +optional
	Port *int `json:"port,omitempty"`

	// Unconditional marks this peer as unconditional.
	// +optional
	Unconditional *bool `json:"unconditional,omitempty"`

	// Private marks this peer as private.
	// +optional
	Private *bool `json:"private,omitempty"`
}

type ExposeConfig struct {
	// P2P indicates whether to expose p2p endpoint for this node. Defaults to `false`.
	// +optional
	// +default=false
	P2P *bool `json:"p2p,omitempty"`

	// P2pServiceType indicates how p2p port will be exposed. Either `LoadBalancer` or `NodePort`.
	// Defaults to `NodePort`.
	// +optional
	// +default="NodePort"
	P2pServiceType *corev1.ServiceType `json:"p2pServiceType,omitempty"`
}

type TmKMS struct {
	// Provider specifies the signing provider to be used by tmkms
	Provider TmKmsProvider `json:"provider"`

	// KeyFormat specifies the format and type of key for chain.
	// Defaults to `{"type": "bech32", "account_key_prefix": "nibipub", "consensus_key_prefix": "nibivalconspub"}`.
	// +optional
	KeyFormat *TmKmsKeyFormat `json:"keyFormat,omitempty"`

	// ValidatorProtocol specifies the tendermint protocol version to be used.
	// One of `legacy`, `v0.33` or `v0.34`. Defaults to `v0.34`.
	// +optional
	ValidatorProtocol *tmkms.ProtocolVersion `json:"validatorProtocol,omitempty"`
}

type TmKmsKeyFormat struct {
	Type               string `json:"type"`
	AccountKeyPrefix   string `json:"account_key_prefix"`
	ConsensusKeyPrefix string `json:"consensus_key_prefix"`
}

type TmKmsProvider struct {
	// Vault provider
	// +optional
	Vault *TmKmsVaultProvider `json:"vault,omitempty"`
}

type TmKmsVaultProvider struct {
	// Address of the Vault cluster
	Address string `json:"address"`

	// Key to be used by this validator.
	Key string `json:"key"`

	// Secret containing the CA certificate of the Vault cluster.
	// +optional
	CertificateSecret *corev1.SecretKeySelector `json:"certificateSecret,omitempty"`

	// Secret containing the token to be used.
	TokenSecret *corev1.SecretKeySelector `json:"tokenSecret"`

	// UploadGenerated indicates if the controller should upload the generated private key to vault.
	// Defaults to `false`. Will be set to `true` if this validator is initializing a new genesis.
	// This should not be used in production.
	// +optional
	UploadGenerated bool `json:"uploadGenerated,omitempty"`
}

type StateSyncConfig struct {
	// SnapshotInterval specifies the block interval at which local state sync snapshots are
	// taken (0 to disable).
	SnapshotInterval int `json:"snapshotInterval"`

	// SnapshotKeepRecent specifies the number of recent snapshots to keep and serve (0 to keep all). Defaults to 2.
	// +optional
	SnapshotKeepRecent *int `json:"snapshotKeepRecent,omitempty"`
}

type FromNodeRPCConfig struct {
	// Defines protocol to use. Defaults to false.
	// +optional
	Secure bool `json:"secure,omitempty"`

	// Hostname or IP address of the RPC server
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`

	// TCP port used for RPC queries on the RPC server. Defaults to `26657`.
	// +optional
	Port *int `json:"port,omitempty"`
}

// Persistence configuration for this node
type Persistence struct {
	// Size of the persistent volume for storing data. Can't be updated when autoResize is enabled.
	// Defaults to `50Gi`.
	// +optional
	// +default="50Gi"
	// +kubebuilder:validation:MinLength=1
	Size *string `json:"size,omitempty"`

	// StorageClassName specifies the name of the storage class to use
	// to create persistent volumes.
	// +optional
	StorageClassName *string `json:"storageClass,omitempty"`

	// AutoResize specifies configurations to automatically resize PVC.
	// Defaults to `true`.
	// +optional
	// +default=true
	AutoResize *bool `json:"autoResize,omitempty"`

	// AutoResizeThreshold is the percentage of data usage at which an auto-resize event should occur.
	// Defaults to `80`.
	// +optional
	// +default=80
	AutoResizeThreshold *int `json:"autoResizeThreshold,omitempty"`

	// AutoResizeIncrement specifies the size increment on each auto-resize event.
	// Defaults to `50Gi`.
	// +optional
	// +default=50Gi
	AutoResizeIncrement *string `json:"autoResizeIncrement,omitempty"`

	// AutoResizeMaxSize specifies the maximum size the PVC can have.
	// Defaults to `2Ti`.
	// +optional
	// +default=2Ti
	AutoResizeMaxSize *string `json:"autoResizeMaxSize,omitempty"`

	// AdditionalInitCommands are additional commands to run on data initialization. Useful for downloading and
	// extracting snapshots.
	// App home is at `/home/app` and data dir is at `/home/app/data`. There is also `/temp`, a temporary volume
	// shared by all init containers.
	// +optional
	AdditionalInitCommands []InitCommand `json:"additionalInitCommands,omitempty"`

	// Snapshots indicates that the operator should create volume snapshots according to this config.
	// +optional
	Snapshots *VolumeSnapshotsConfig `json:"snapshots,omitempty"`

	// RestoreFromSnapshot indicates that the operator should restore from the specified snapshot when creating
	// the PVC for this node.
	// +optional
	RestoreFromSnapshot *PvcSnapshot `json:"restoreFromSnapshot,omitempty"`
}

type VolumeSnapshotsConfig struct {
	// Frequency indicates how often a snapshot should be created. Specified as a duration with suffix `s`, `m` or `h`.
	Frequency string `json:"frequency"`

	// Retention indicates for how long a snapshot should be retained. Default is indefinite retention.
	// Specified as a duration with suffix `s`, `m` or `h`.
	// +optional
	Retention *string `json:"retention,omitempty"`

	// SnapshotClassName is the name of the volume snapshot class to be used.
	// +optional
	SnapshotClassName *string `json:"snapshotClass,omitempty"`

	// StopNode indicates that the node should be stopped while the snapshot is taken. Defaults to `false`.
	// +optional
	StopNode *bool `json:"stopNode,omitempty"`

	// ExportTarball creates a tarball of data directory in each snapshot and uploads it to external storage.
	// +optional
	ExportTarball *ExportTarballConfig `json:"exportTarball,omitempty"`
}

type PvcSnapshot struct {
	// Name is the name of resource being referenced
	Name string `json:"name"`

	// Kind is the type of resource being referenced. Defaults to `VolumeSnapshot`.
	// +optional
	Kind *string `json:"kind,omitempty"`

	// APIGroup is the group for the resource being referenced. Defaults to `snapshot.storage.k8s.io`.
	APIGroup *string `json:"apiGroup,omitempty"`
}

type ExportTarballConfig struct {
	// Suffix to add to archive name. The name of the tarball is `<chain-id>-<timestamp>-<suffix>`.
	// +optional
	Suffix *string `json:"suffix,omitempty"`

	// DeleteOnExpire makes sure the tarball is deleted when the snapshot expires. Default is `false`.
	// +optional
	DeleteOnExpire *bool `json:"deleteOnExpire,omitempty"`

	// GCS allows configuring to upload tarballs to a GCS bucket
	// +optional
	GCS *GcsExportConfig `json:"gcs,omitempty"`
}

type GcsExportConfig struct {
	// Name of the bucket to upload tarballs to.
	Bucket string `json:"bucket"`

	// CredentialsSecret is the secret that contains the credentials to upload to bucket.
	CredentialsSecret *corev1.SecretKeySelector `json:"credentialsSecret"`
}

// Upgrades

type UpgradePhase string

const (
	UpgradeImageMissing UpgradePhase = "image-missing"
	UpgradeScheduled    UpgradePhase = "scheduled"
	UpgradeOnGoing      UpgradePhase = "ongoing"
	UpgradeCompleted    UpgradePhase = "completed"
)

type UpgradeSource string

const (
	OnChainUpgrade UpgradeSource = "on-chain"
	ManualUpgrade  UpgradeSource = "manual"
)

type UpgradeSpec struct {
	// Height at which the upgrade should occur.
	Height int64 `json:"height"`

	// Image replacement to be used in the upgrade.
	Image string `json:"image"`
}

type Upgrade struct {
	// Height at which the upgrade should occur.
	Height int64 `json:"height"`

	// Image replacement to be used in the upgrade.
	Image string `json:"image"`

	// Status indicates the upgrade status.
	Status UpgradePhase `json:"status"`

	// Source indicates where the operator got this upgrade from.
	Source UpgradeSource `json:"source"`
}

type CreateValidatorConfig struct {
	// AccountMnemonicSecret is the name of the secret containing the mnemonic of the account to be used by
	// this validator. Defaults to `<chainnode>-account`. Will be created if does not exist.
	AccountMnemonicSecret *string `json:"accountMnemonicSecret,omitempty"`

	// AccountHDPath is the HD path for the validator account. Defaults to `m/44'/118'/0'/0/0`.
	// +optional
	AccountHDPath *string `json:"accountHDPath,omitempty"`

	// AccountPrefix is the prefix for accounts. Defaults to `nibi`.
	// +optional
	AccountPrefix *string `json:"accountPrefix,omitempty"`

	// ValPrefix is the prefix for validator accounts. Defaults to `nibivaloper`.
	// +optional
	ValPrefix *string `json:"valPrefix,omitempty"`

	// StakeAmount represents the amount to be staked by this validator.
	StakeAmount string `json:"stakeAmount"`

	// GasPrices in decimal format to determine the transaction fee.
	GasPrices string `json:"gasPrices"`

	// CommissionMaxChangeRate is the maximum commission change rate percentage (per day). Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxChangeRate *string `json:"commissionMaxChangeRate,omitempty"`

	// CommissionMaxRate is the maximum commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxRate *string `json:"commissionMaxRate,omitempty"`

	// CommissionRate is the initial commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionRate *string `json:"commissionRate,omitempty"`

	// MinSelfDelegation is the minimum self delegation required on the validator. Defaults to `1`.
	// +optional
	// +default="0.1"
	MinSelfDelegation *string `json:"minSelfDelegation,omitempty"`
}
