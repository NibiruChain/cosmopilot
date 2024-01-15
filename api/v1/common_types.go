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
	ReasonSnapshotIntegrityStart = "IntegrityCheckStart"
	ReasonSnapshotDataOk         = "DataCorrupted"
	ReasonSnapshotDataCorrupted  = "DataOk"
	ReasonUpgradeCompleted       = "UpgradeCompleted"
	ReasonUpgradeFailed          = "UpgradeFailed"
	ReasonUpgradeMissingData     = "UpgradeMissingData"
	ReasonCreateValidatorFailure = "FailedCreateValidator"
	ReasonCreateValidatorSuccess = "CreateValidatorSuccess"
	ReasonInvalid                = "Invalid"
)

// SdkVersion specifies the cosmos-sdk version used by this application.
// +kubebuilder:validation:Enum=v0.45;v0.47
type SdkVersion string

const (
	// Cosmos-sdk version v0.47.x.
	V0_47 SdkVersion = "v0.47"

	// Cosmos-sdk version v0.45.x and below.
	V0_45 SdkVersion = "v0.45"
)

// ValidatorStatus represents the current status of a validator.
type ValidatorStatus string

const (
	// ValidatorStatusBonded indicates that validator is bonded and in the validator set.
	ValidatorStatusBonded = "bonded"

	// ValidatorStatusUnbonded indicates that validator is unbonded.
	ValidatorStatusUnbonded = "unbonded"

	// ValidatorStatusUnbonding indicates that validator is unbonding.
	ValidatorStatusUnbonding = "unbonding"

	// ValidatorStatusUnknown indicates that validator status is unknown.
	ValidatorStatusUnknown = "unknown"
)

// AppSpec specifies the source image, version and binary name of the app to run. Also allows
// specifying upgrades for the app and enabling automatic check of upgrade proposals on chain.
type AppSpec struct {
	// Container image to be used.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Image tag to be used. Once there are completed or skipped upgrades this will be ignored.
	// For a new node that will be state-synced, this will be the version used during state-sync. Only after
	// that, the operator will switch to the version of last upgrade.
	// Defaults to `latest`.
	// +optional
	// +default=latest
	Version *string `json:"version,omitempty"`

	// Indicates the desired pull policy when creating nodes. Defaults to `Always` if `version`
	// is `latest` and `IfNotPresent` otherwise.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Binary name of the application to be run.
	// +kubebuilder:validation:MinLength=1
	App string `json:"app"`

	// SdkVersion specifies the version of cosmos-sdk used by this app.
	// Valid options are:
	// - "v0.47" (default)
	// - "v0.45"
	// +optional
	// +default=v0.47
	SdkVersion *SdkVersion `json:"sdkVersion,omitempty"`

	// Whether the operator should query gov proposals to find and schedule upgrades.
	// Defaults to `true`.
	// +optional
	// +default=true
	CheckGovUpgrades *bool `json:"checkGovUpgrades,omitempty"`

	// List of upgrades to schedule for this node.
	// +optional
	Upgrades []UpgradeSpec `json:"upgrades,omitempty"`
}

// Config allows setting specific configurations for a node, including overriding configs in app.toml and config.toml.
type Config struct {
	// Allows overriding configs on `.toml` configuration files.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Override *map[string]runtime.RawExtension `json:"override,omitempty"`

	// Allows configuring additional containers to run alongside the node.
	// +optional
	Sidecars []SidecarSpec `json:"sidecars,omitempty"`

	// Optional list of references to secrets in the same namespace to use for pulling any of the images used by this node.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// The time to wait for a block before considering node unhealthy.
	// Defaults to `15s`.
	// +optional
	// +default=15s
	// +kubebuilder:validation:Format=duration
	BlockThreshold *string `json:"blockThreshold,omitempty"`

	// Period at which a reconcile loop will happen for this ChainNode.
	// Defaults to `15s`.
	// +optional
	// +default=15s
	// +kubebuilder:validation:Format=duration
	ReconcilePeriod *string `json:"reconcilePeriod,omitempty"`

	// Allows configuring this node to perform state-sync snapshots.
	// +optional
	StateSync *StateSyncConfig `json:"stateSync,omitempty"`

	// Configures this node to run on seed mode. Defaults to `false`.
	// +optional
	// +default=false
	SeedMode *bool `json:"seedMode,omitempty"`

	// List of environment variables to set in the app container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// SafeToEvict sets cluster-autoscaler.kubernetes.io/safe-to-evict annotation to the given value. It allows/disallows
	// cluster-autoscaler to evict this node's pod.
	// +optional
	SafeToEvict *bool `json:"safeToEvict,omitempty"`

	// ServiceMonitor allows deploying prometheus service monitor for this node.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`

	// Deploys cosmos-firewall to protect API endpoints to the node.
	// +optional
	Firewall *FirewallConfig `json:"firewall,omitempty"`

	// Log level for node-utils container. Defaults to `info`.
	// +optional
	NodeUtilsLogLevel *string `json:"nodeUtilsLogLevel,omitempty"`
}

// FirewallConfig allows configuring cosmos-firewall rules.
type FirewallConfig struct {
	// Whether to enable cosmos-firewall on this node.
	Enable bool `json:"enable"`

	// ConfigMap which cosmos-firewall configuration for this node.
	Config *corev1.ConfigMapKeySelector `json:"config"`
}

// ServiceMonitorSpec allows enabling/disabling deployment of ServiceMonitor for this node.
type ServiceMonitorSpec struct {
	// Whether a service monitor should be deployed for this node.
	Enable bool `json:"enable"`

	// Indicates the prometheus installation that will be using this service monitor.
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

// SidecarSpec allows configuring additional containers to run alongside the node.
type SidecarSpec struct {
	// Name to be assigned to the container.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Container image to be used.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Indicates the desired pull policy when creating nodes. Defaults to `Always` if `version`
	// is `latest` and `IfNotPresent` otherwise.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Where data volume will be mounted on this container. It is not mounted if not specified.
	// +optional
	MountDataVolume *string `json:"mountDataVolume,omitempty"`

	// Command to be run by this container. Defaults to entrypoint defined in image.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args to be passed to this container. Defaults to cmd defined in image.
	// +optional
	Args []string `json:"args,omitempty"`

	// Environment variables to be passed to this container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Security options the container should be run with.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Compute Resources for the sidecar container.
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

	// Website of the validator.
	// +optional
	Website *string `json:"website,omitempty"`

	// Identity signature of this validator.
	// +optional
	Identity *string `json:"identity,omitempty"`
}

// GenesisInitConfig specifies configs and initialization commands for creating a new genesis.
type GenesisInitConfig struct {
	// ChainID of the chain to initialize.
	// +kubebuilder:validation:MinLength=1
	ChainID string `json:"chainID"`

	// Name of the secret containing the mnemonic of the account to be used by
	// this validator. Defaults to `<chainnode>-account`. Will be created if it does not exist.
	// +optional
	AccountMnemonicSecret *string `json:"accountMnemonicSecret,omitempty"`

	// HD path of accounts. Defaults to `m/44'/118'/0'/0/0`.
	// +optional
	// +default=`m/44'/118'/0'/0/0`
	AccountHDPath *string `json:"accountHDPath,omitempty"`

	// Prefix for accounts. Defaults to `nibi`.
	// +optional
	// +default=nibi
	AccountPrefix *string `json:"accountPrefix,omitempty"`

	// Prefix for validator operator accounts. Defaults to `nibivaloper`.
	// +optional
	// +default=nibivaloper
	ValPrefix *string `json:"valPrefix,omitempty"`

	// Maximum commission change rate percentage (per day). Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxChangeRate *string `json:"commissionMaxChangeRate,omitempty"`

	// Maximum commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxRate *string `json:"commissionMaxRate,omitempty"`

	// Initial commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionRate *string `json:"commissionRate,omitempty"`

	// Minimum self delegation required on the validator. Defaults to `1`.
	// +optional
	// +default="1"
	MinSelfDelegation *string `json:"minSelfDelegation,omitempty"`

	// Assets is the list of tokens and their amounts to be assigned to this validators account.
	Assets []string `json:"assets"`

	// Amount to be staked by this validator.
	StakeAmount string `json:"stakeAmount"`

	// Accounts specify additional accounts and respective assets to be added to this chain.
	// +optional
	Accounts []AccountAssets `json:"accounts,omitempty"`

	// Time required to totally unbond delegations. Defaults to `1814400s` (21 days).
	// +optional
	// +default=1814400s
	// +kubebuilder:validation:Format=duration
	UnbondingTime *string `json:"unbondingTime,omitempty"`

	// Voting period for this chain. Defaults to `120h`.
	// +optional
	// +default=120h
	// +kubebuilder:validation:Format=duration
	VotingPeriod *string `json:"votingPeriod,omitempty"`

	// Additional commands to run on genesis initialization.
	// Note: App home is at `/home/app` and `/temp` is a temporary volume shared by all init containers.
	// +optional
	AdditionalInitCommands []InitCommand `json:"additionalInitCommands,omitempty"`
}

// AccountAssets represents the assets associated with an account.
type AccountAssets struct {
	// Address of the account.
	Address string `json:"address"`

	// Assets assigned to this account.
	Assets []string `json:"assets"`
}

// InitCommand represents an initialization command. It may be used for running addtional operators
// on genesis or volume initialization.
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

// GenesisConfig specifies how genesis will be retrieved.
type GenesisConfig struct {
	// URL to download the genesis from.
	// +optional
	Url *string `json:"url,omitempty"`

	// Get the genesis from an existing node using its RPC endpoint.
	// +optional
	FromNodeRPC *FromNodeRPCConfig `json:"fromNodeRPC,omitempty"`

	// SHA256 to validate the genesis.
	// +optional
	GenesisSHA *string `json:"genesisSHA,omitempty"`

	// ConfigMap specifies a configmap to load the genesis from.
	// +optional
	ConfigMap *string `json:"configMap,omitempty"`

	// UseDataVolume indicates that the operator should save the genesis in the same volume as node data
	// instead of a ConfigMap. This is useful for genesis whose size is bigger than ConfigMap limit of 1MiB.
	// +optional
	UseDataVolume *bool `json:"useDataVolume,omitempty"`
}

// Peer represents a persistent peer.
type Peer struct {
	// Tendermint node ID for this node.
	ID string `json:"id"`

	// Hostname or IP address of this peer.
	Address string `json:"address"`

	// P2P port to be used. Defaults to `26656`.
	// +optional
	// +default=26656
	Port *int `json:"port,omitempty"`

	// Indicates this peer is unconditional.
	// +optional
	Unconditional *bool `json:"unconditional,omitempty"`

	// Indicates this peer is private.
	// +optional
	Private *bool `json:"private,omitempty"`
}

// ExposeConfig allows configuring how P2P endpoint is exposed to public.
type ExposeConfig struct {
	// Whether to expose p2p endpoint for this node. Defaults to `false`.
	// +optional
	// +default=false
	P2P *bool `json:"p2p,omitempty"`

	// P2pServiceType indicates how P2P port will be exposed.
	// Valid values are:
	// - `LoadBalancer`
	// - `NodePort` (default)
	// +optional
	// +default="NodePort"
	P2pServiceType *corev1.ServiceType `json:"p2pServiceType,omitempty"`
}

// TmKMS allows configuring tmkms for signing for this validator node instead of
// using plaintext private key file.
type TmKMS struct {
	// Signing provider to be used by tmkms. Currently only `vault` is supported.
	Provider TmKmsProvider `json:"provider"`

	// Format and type of key for chain.
	// Defaults to `{"type": "bech32", "account_key_prefix": "nibipub", "consensus_key_prefix": "nibivalconspub"}`.
	// +optional
	// +default={type: bech32, account_key_prefix: nibipub, consensus_key_prefix: nibivalconspub}
	KeyFormat *TmKmsKeyFormat `json:"keyFormat,omitempty"`

	// Tendermint's protocol version to be used.
	// Valid options are:
	// - `v0.34` (default)
	// - `v0.33`
	// - `legacy`
	// +optional
	// +default="v0.34"
	ValidatorProtocol *tmkms.ProtocolVersion `json:"validatorProtocol,omitempty"`
}

// TmKmsKeyFormat represents key format for tmKMS.
type TmKmsKeyFormat struct {
	// Key type
	Type string `json:"type"`

	// Account keys prefixes
	AccountKeyPrefix string `json:"account_key_prefix"`

	// Consensus keys prefix
	ConsensusKeyPrefix string `json:"consensus_key_prefix"`
}

// TmKmsProvider allows configuring providers for tmKMS. Note that only one should be configured.
type TmKmsProvider struct {
	// Vault provider.
	// +optional
	Vault *TmKmsVaultProvider `json:"vault,omitempty"`
}

// TmKmsVaultProvider holds `vault` provider specific configurations.
type TmKmsVaultProvider struct {
	// Full address of the Vault cluster.
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

	// Whether to automatically renew vault token. Defaults to `false`.
	// +optional
	AutoRenewToken bool `json:"autoRenewToken,omitempty"`
}

// StateSyncConfig holds configurations for enabling state-sync snapshots on a node.
type StateSyncConfig struct {
	// Block interval at which local state sync snapshots are taken (0 to disable).
	SnapshotInterval int `json:"snapshotInterval"`

	// Number of recent snapshots to keep and serve (0 to keep all). Defaults to 2.
	// +optional
	SnapshotKeepRecent *int `json:"snapshotKeepRecent,omitempty"`
}

// FromNodeRPCConfig holds configuration to retrieve genesis from an existing node
// using RPC endpoint.
type FromNodeRPCConfig struct {
	// Defines protocol to use. Defaults to `false`.
	// +optional
	// +default=false
	Secure bool `json:"secure,omitempty"`

	// Hostname or IP address of the RPC server
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`

	// TCP port used for RPC queries on the RPC server. Defaults to `26657`.
	// +optional
	// +default=26657
	Port *int `json:"port,omitempty"`
}

// Persistence configuration for a node.
type Persistence struct {
	// Size of the persistent volume for storing data. Can't be updated when autoResize is enabled.
	// Defaults to `50Gi`.
	// +optional
	// +default="50Gi"
	Size *string `json:"size,omitempty"`

	// Name of the storage class to use for the PVC. Uses the default class if not specified.
	// to create persistent volumes.
	// +optional
	StorageClassName *string `json:"storageClass,omitempty"`

	// Automatically resize PVC.
	// Defaults to `true`.
	// +optional
	// +default=true
	AutoResize *bool `json:"autoResize,omitempty"`

	// Percentage of data usage at which an auto-resize event should occur.
	// Defaults to `80`.
	// +optional
	// +default=80
	AutoResizeThreshold *int `json:"autoResizeThreshold,omitempty"`

	// Increment size on each auto-resize event.
	// Defaults to `50Gi`.
	// +optional
	// +default=50Gi
	AutoResizeIncrement *string `json:"autoResizeIncrement,omitempty"`

	// Size at which auto-resize will stop incrementing PVC size.
	// Defaults to `2Ti`.
	// +optional
	// +default=2Ti
	AutoResizeMaxSize *string `json:"autoResizeMaxSize,omitempty"`

	// Additional commands to run on data initialization. Useful for downloading and
	// extracting snapshots.
	// App home is at `/home/app` and data dir is at `/home/app/data`. There is also `/temp`, a temporary volume
	// shared by all init containers.
	// +optional
	AdditionalInitCommands []InitCommand `json:"additionalInitCommands,omitempty"`

	// Whether the operator should create volume snapshots according to this config.
	// +optional
	Snapshots *VolumeSnapshotsConfig `json:"snapshots,omitempty"`

	// Restore from the specified snapshot when creating the PVC for this node.
	// +optional
	RestoreFromSnapshot *PvcSnapshot `json:"restoreFromSnapshot,omitempty"`
}

// VolumeSnapshotsConfig holds the configuration of snapshotting feature.
type VolumeSnapshotsConfig struct {
	// How often a snapshot should be created.
	// +kubebuilder:validation:Format=duration
	Frequency string `json:"frequency"`

	// How long a snapshot should be retained. Default is indefinite retention.
	// +optional
	// +kubebuilder:validation:Format=duration
	Retention *string `json:"retention,omitempty"`

	// Name of the volume snapshot class to be used. Uses the default class if not specified.
	// +optional
	SnapshotClassName *string `json:"snapshotClass,omitempty"`

	// Whether the node should be stopped while the snapshot is taken. Defaults to `false`.
	// +optional
	// +default=false
	StopNode *bool `json:"stopNode,omitempty"`

	// Whether to create a tarball of data directory in each snapshot and upload it to external storage.
	// +optional
	ExportTarball *ExportTarballConfig `json:"exportTarball,omitempty"`

	// Whether the operator should verify the snapshot for corruption after it is ready. Defaults to `false`.
	// +optional
	// +default=false
	Verify *bool `json:"verify,omitempty"`
}

// PvcSnapshot represents a snapshot to be used to restore a PVC.
type PvcSnapshot struct {
	// Name of resource being referenced.
	Name string `json:"name"`

	// Type of resource being referenced. Defaults to `VolumeSnapshot`.
	// +optional
	// +default=VolumeSnapshot
	Kind *string `json:"kind,omitempty"`

	// Group for the resource being referenced. Defaults to `snapshot.storage.k8s.io`.
	// +optional
	// +default=snapshot.storage.k8s.io
	APIGroup *string `json:"apiGroup,omitempty"`
}

// ExportTarballConfig holds config options for tarball upload.
type ExportTarballConfig struct {
	// Suffix to add to archive name. The name of the tarball will be `<chain-id>-<timestamp>-<suffix>`.
	// +optional
	Suffix *string `json:"suffix,omitempty"`

	// Whether to delete the tarball when the snapshot expires. Default is `false`.
	// +optional
	// +default=false
	DeleteOnExpire *bool `json:"deleteOnExpire,omitempty"`

	// Configuration to upload tarballs to a GCS bucket.
	// +optional
	GCS *GcsExportConfig `json:"gcs,omitempty"`
}

// GcsExportConfig holds required settings to upload to GCS.
type GcsExportConfig struct {
	// Name of the bucket to upload tarballs to.
	Bucket string `json:"bucket"`

	// Secret with the JSON credentials to upload to bucket.
	CredentialsSecret *corev1.SecretKeySelector `json:"credentialsSecret"`
}

// UpgradePhase indicates the current phase of an upgrade.
type UpgradePhase string

const (
	// UpgradeImageMissing indicates that a scheduled upgrade is missing the image.
	UpgradeImageMissing UpgradePhase = "image-missing"

	// UpgradeScheduled indicates that the upgrade is scheduled and will be
	// performed by the operator.
	UpgradeScheduled UpgradePhase = "scheduled"

	// UpgradeOnGoing indicates that the upgrade is on going.
	UpgradeOnGoing UpgradePhase = "ongoing"

	// UpgradeCompleted indicates that the upgrade was successfully finished.
	// Note: successfully finished means the container was restarted with the
	// new image. Application issues after the upgrade won't be detected.
	UpgradeCompleted UpgradePhase = "completed"

	// UpgradeSkipped indicates that the operator will not perform the upgrade
	// because it is in the past.
	UpgradeSkipped UpgradePhase = "skipped"
)

// UpgradeSource indicates the source of a scheduled upgrade.
type UpgradeSource string

const (
	// OnChainUpgrade represents an upgrade that was retrieved from governance
	// on chain.
	OnChainUpgrade UpgradeSource = "on-chain"

	// ManualUpgrade represents an upgrade that was manually specified by the user.
	ManualUpgrade UpgradeSource = "manual"
)

// UpgradeSpec represents a manual upgrade.
type UpgradeSpec struct {
	// Height at which the upgrade should occur.
	Height int64 `json:"height"`

	// Container image replacement to be used in the upgrade.
	Image string `json:"image"`
}

// Upgrade represents an upgrade processed by the operator and added to status.
type Upgrade struct {
	// Height at which the upgrade should occur.
	Height int64 `json:"height"`

	// Container image replacement to be used in the upgrade.
	Image string `json:"image"`

	// Upgrade status.
	Status UpgradePhase `json:"status"`

	// Where the operator got this upgrade from.
	Source UpgradeSource `json:"source"`
}

// CreateValidatorConfig holds configuration for the operator to submit a create-validator transaction.
type CreateValidatorConfig struct {
	// Name of the secret containing the mnemonic of the account to be used by
	// this validator. Defaults to `<chainnode>-account`. Will be created if it does not exist.
	// +optional
	AccountMnemonicSecret *string `json:"accountMnemonicSecret,omitempty"`

	// HD path of accounts. Defaults to `m/44'/118'/0'/0/0`.
	// +optional
	// +default=`m/44'/118'/0'/0/0`
	AccountHDPath *string `json:"accountHDPath,omitempty"`

	// Prefix for accounts. Defaults to `nibi`.
	// +optional
	// +default=nibi
	AccountPrefix *string `json:"accountPrefix,omitempty"`

	// Prefix for validator operator accounts. Defaults to `nibivaloper`.
	// +optional
	// +default=nibivaloper
	ValPrefix *string `json:"valPrefix,omitempty"`

	// Maximum commission change rate percentage (per day). Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxChangeRate *string `json:"commissionMaxChangeRate,omitempty"`

	// Maximum commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionMaxRate *string `json:"commissionMaxRate,omitempty"`

	// Initial commission rate percentage. Defaults to `0.1`.
	// +optional
	// +default="0.1"
	CommissionRate *string `json:"commissionRate,omitempty"`

	// Minimum self delegation required on the validator. Defaults to `1`.
	// +optional
	// +default="1"
	MinSelfDelegation *string `json:"minSelfDelegation,omitempty"`

	// Amount to be staked by this validator.
	StakeAmount string `json:"stakeAmount"`

	// Gas prices in decimal format to determine the transaction fee.
	GasPrices string `json:"gasPrices"`
}
