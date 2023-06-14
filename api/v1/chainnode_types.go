package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(&ChainNode{}, &ChainNodeList{})
}

// ChainNodePhase is a label for the condition of a node at the current time.
type ChainNodePhase string

// These are the valid phases for nodes.
const (
	PhaseInitData    ChainNodePhase = "InitializingData"
	PhaseInitGenesis ChainNodePhase = "InitGenesis"
	PhaseStarting    ChainNodePhase = "Starting"
	PhaseRunning     ChainNodePhase = "Running"
	PhaseSyncing     ChainNodePhase = "Syncing"
	PhaseRestarting  ChainNodePhase = "Restarting"
)

// ChainNode events
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
	ReasonNodeSyncing        = "NodeSyncing"
	ReasonNodeRunning        = "NodeRunning"
	ReasonValidatorJailed    = "ValidatorJailed"
	ReasonValidatorUnjailed  = "ValidatorUnjailed"
)

//+kubebuilder:object:root=true

// ChainNodeList contains a list of ChainNode
type ChainNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChainNode `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
//+kubebuilder:printcolumn:name="ChainID",type=string,JSONPath=`.status.chainID`
//+kubebuilder:printcolumn:name="Validator",type=boolean,JSONPath=`.status.validator`
//+kubebuilder:printcolumn:name="Jailed",type=boolean,JSONPath=`.status.jailed`
//+kubebuilder:printcolumn:name="DataUsage",type=string,JSONPath=`.status.dataUsage`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChainNode is the Schema for the chainnodes API
type ChainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainNodeSpec   `json:"spec,omitempty"`
	Status ChainNodeStatus `json:"status,omitempty"`
}

// ChainNodeSpec defines the desired state of ChainNode
type ChainNodeSpec struct {
	// Genesis indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified.
	// +optional
	Genesis *GenesisConfig `json:"genesis"`

	// App specifies image and binary name of the chain application to run
	App AppSpec `json:"app"`

	// Config allows setting specific configurations for this node
	// +optional
	Config *Config `json:"config,omitempty"`

	// Persistence configures pvc for persisting data on nodes
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

	// Validator configures this node as a validator and configures it.
	// +optional
	Validator *ValidatorConfig `json:"validator,omitempty"`

	// AutoDiscoverPeers ensures peers with same chain ID are connected with each other. By default, it is enabled.
	// +optional
	AutoDiscoverPeers *bool `json:"autoDiscoverPeers,omitempty"`

	// Peers are additional persistent peers that should be added to this node.
	// +optional
	Peers []Peer `json:"peers,omitempty"`
}

// ChainNodeStatus defines the observed state of ChainNode
type ChainNodeStatus struct {
	// Jailed indicates the current phase for this ChainNode.
	// +optional
	Phase ChainNodePhase `json:"phase,omitempty"`

	// NodeID show this node's ID
	// +optional
	NodeID string `json:"nodeID,omitempty"`

	// IP of this node.
	// +optional
	IP string `json:"ip,omitempty"`

	// ChainID shows the chain ID
	// +optional
	ChainID string `json:"chainID,omitempty"`

	// PvcSize shows the current size of the pvc of this node
	// +optional
	PvcSize string `json:"pvcSize,omitempty"`

	// DataUsage shows the percentage of data usage.
	// +optional
	DataUsage string `json:"dataUsage,omitempty"`

	// Validator indicates if this node is a validator.
	Validator bool `json:"validator"`

	// AccountAddress is the account address of this validator. Omitted when not a validator
	// +optional
	AccountAddress string `json:"accountAddress,omitempty"`

	// ValidatorAddress is the valoper address of this validator. Omitted when not a validator
	// +optional
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// Jailed indicates if this validator is jailed. Always false if not a validator node.
	Jailed bool `json:"jailed"`
}

// GenesisConfig specifies how genesis will be retrieved
type GenesisConfig struct {
	// URL to download the genesis from.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Url *string `json:"url,omitempty"`

	// ConfigMap specifies a configmap to load the genesis from
	// +optional
	ConfigMap *string `json:"configMap,omitempty"`
}

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

// Config allows setting specific configurations for this node such has overrides to app.toml and config.toml
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
}

// ValidatorConfig turns this node into a validator and specifies how it will do it.
type ValidatorConfig struct {
	// PrivateKeySecret indicates the secret containing the private key to be use by this validator.
	// Defaults to `<chainnode>-priv-key`. Will be created if it does not exist.
	// +optional
	PrivateKeySecret *string `json:"privateKeySecret,omitempty"`

	// Info contains information details about this validator.
	// +optional
	Info *ValidatorInfo `json:"info,omitempty"`

	// Init specifies configs and initialization commands for creating a new chain and its genesis.
	// +optional
	Init *GenesisInitConfig `json:"init,omitempty"`
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
