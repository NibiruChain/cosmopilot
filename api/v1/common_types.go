package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/NibiruChain/nibiru-operator/internal/tmkms"
)

// Reasons for events
const (
	ReasonPvcResized         = "PvcResized"
	ReasonPvcMaxReached      = "PvcMaxSizeReached"
	ReasonDataInitialized    = "DataInitialized"
	ReasonNodeKeyCreated     = "NodeKeyCreated"
	ReasonNodeKeyImported    = "NodeKeyImported"
	ReasonPrivateKeyCreated  = "PrivateKeyCreated"
	ReasonPrivateKeyImported = "PrivateKeyImported"
	ReasonAccountCreated     = "AccountCreated"
	ReasonAccountImported    = "AccountImported"
	ReasonGenesisInitialized = "GenesisCreated"
	ReasonGenesisImported    = "GenesisImported"
	ReasonConfigsCreated     = "ConfigsCreated"
	ReasonConfigsUpdated     = "ConfigsUpdated"
	ReasonNodeStarted        = "NodeStarted"
	ReasonNodeRestarted      = "NodeRestarted"
	ReasonNodeError          = "NodeError"
	ReasonNodeSyncing        = "NodeSyncing"
	ReasonNodeRunning        = "NodeRunning"
	ReasonValidatorJailed    = "ValidatorJailed"
	ReasonValidatorUnjailed  = "ValidatorUnjailed"
	ReasonNodeCreated        = "NodeCreated"
	ReasonNodeUpdated        = "NodeUpdated"
	ReasonNodeDeleted        = "NodeDeleted"
	ReasonInitGenesisFailure = "InitGenesisFail"
	ReasonUploadFailure      = "UploadFailed"
	ReasonGenesisWrongHash   = "GenesisWrongHash"
	ReasonNoTrustHeight      = "NoTrustHeight"
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

	// BlockThreshold specifies the time to wait for a block before considering node unhealthy
	// +optional
	BlockThreshold *string `json:"blockThreshold,omitempty"`

	// ReconcilePeriod is the period at which a reconcile loop will happen for this ChainNode.
	// Defaults to `1m`.
	// +optional
	// +default=1m
	ReconcilePeriod *string `json:"reconcilePeriod,omitempty"`

	// StateSync configures statesync snapshots for this node.
	// +optional
	StateSync *StateSyncConfig `json:"stateSync,omitempty"`
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
