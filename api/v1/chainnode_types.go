package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ChainNode{}, &ChainNodeList{})
}

// ChainNodePhase is a label for the condition of a node at the current time.
type ChainNodePhase string

// These are the valid phases for ChainNodes.
const (
	PhaseChainNodeInitData     ChainNodePhase = "InitializingData"
	PhaseChainNodeInitGenesis  ChainNodePhase = "InitGenesis"
	PhaseChainNodeStarting     ChainNodePhase = "Starting"
	PhaseChainNodeRunning      ChainNodePhase = "Running"
	PhaseChainNodeSyncing      ChainNodePhase = "Syncing"
	PhaseChainNodeRestarting   ChainNodePhase = "Restarting"
	PhaseChainNodeError        ChainNodePhase = "Error"
	PhaseChainNodeSnapshotting ChainNodePhase = "Snapshotting"
	PhaseChainNodeUpgrading    ChainNodePhase = "Upgrading"
)

const (
	ConditionUpgrade = "Upgrade"

	ReasonUpgradeSuccess = "UpgradeSuccessful"
)

//+kubebuilder:object:root=true

// ChainNodeList contains a list of ChainNode.
type ChainNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChainNode `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.ip`
//+kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.appVersion`
//+kubebuilder:printcolumn:name="ChainID",type=string,JSONPath=`.status.chainID`
//+kubebuilder:printcolumn:name="Validator",type=boolean,JSONPath=`.status.validator`
//+kubebuilder:printcolumn:name="BondStatus",type=string,JSONPath=`.status.validatorStatus`
//+kubebuilder:printcolumn:name="Jailed",type=boolean,JSONPath=`.status.jailed`
//+kubebuilder:printcolumn:name="DataUsage",type=string,JSONPath=`.status.dataUsage`
//+kubebuilder:printcolumn:name="LatestHeight",type=integer,JSONPath=`.status.latestHeight`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChainNode is the Schema for the chainnodes API.
type ChainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainNodeSpec   `json:"spec,omitempty"`
	Status ChainNodeStatus `json:"status,omitempty"`
}

// ChainNodeSpec defines the desired state of ChainNode.
type ChainNodeSpec struct {
	// Indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified.
	// +optional
	Genesis *GenesisConfig `json:"genesis"`

	// Specifies image, version and binary name of the chain application to run. It also allows to schedule upgrades,
	// or setting/updating the image for an on-chain upgrade.
	App AppSpec `json:"app"`

	// Allows setting specific configurations for this node.
	// +optional
	Config *Config `json:"config,omitempty"`

	// Configures PVC for persisting data. Automated data snapshots can also be configured in
	// this section.
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

	// Indicates this node is going to be a validator and allows configuring it.
	// +optional
	Validator *ValidatorConfig `json:"validator,omitempty"`

	// Ensures peers with same chain ID are connected with each other. Enabled by default.
	// +optional
	AutoDiscoverPeers *bool `json:"autoDiscoverPeers,omitempty"`

	// Configures this node to find a state-sync snapshot on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`

	// Additional persistent peers that should be added to this node.
	// +optional
	Peers []Peer `json:"peers,omitempty"`

	// Allows exposing P2P traffic to public.
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// Compute Resources required by the app container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's scheduling constraints.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// ChainNodeStatus defines the observed state of ChainNode
type ChainNodeStatus struct {
	// Indicates the current phase for this ChainNode.
	// +optional
	Phase ChainNodePhase `json:"phase,omitempty"`

	// Conditions to track state of the ChainNode.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Indicates this node's ID.
	// +optional
	NodeID string `json:"nodeID,omitempty"`

	// Internal IP address of this node.
	// +optional
	IP string `json:"ip,omitempty"`

	// Public address for P2P when enabled.
	// +optional
	PublicAddress string `json:"publicAddress,omitempty"`

	// Indicates the chain ID.
	// +optional
	ChainID string `json:"chainID,omitempty"`

	// Current size of the data PVC for this node.
	// +optional
	PvcSize string `json:"pvcSize,omitempty"`

	// Usage percentage of data volume.
	// +optional
	DataUsage string `json:"dataUsage,omitempty"`

	// Indicates if this node is a validator.
	Validator bool `json:"validator"`

	// Account address of this validator. Omitted when not a validator.
	// +optional
	AccountAddress string `json:"accountAddress,omitempty"`

	// Validator address is the valoper address of this validator. Omitted when not a validator.
	// +optional
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// Indicates if this validator is jailed. Always false if not a validator node.
	// +optional
	Jailed bool `json:"jailed,omitempty"`

	// Application version currently deployed.
	// +optional
	AppVersion string `json:"appVersion,omitempty"`

	// Last height read on the node by cosmopilot.
	// +optional
	LatestHeight int64 `json:"latestHeight,omitempty"`

	// Indicates if this node is running with seed mode enabled.
	// +optional
	SeedMode bool `json:"seedMode,omitempty"`

	// All scheduled/completed upgrades performed by cosmopilot on this ChainNode.
	// +optional
	Upgrades []Upgrade `json:"upgrades,omitempty"`

	// Public key of the validator.
	// +optional
	PubKey string `json:"pubKey,omitempty"`

	// Indicates the current status of validator if this node is one.
	// +optional
	ValidatorStatus ValidatorStatus `json:"validatorStatus,omitempty"`
}

// ValidatorConfig contains the configuration for running a node as validator.
type ValidatorConfig struct {
	// Indicates the secret containing the private key to be used by this validator.
	// Defaults to `<chainnode>-priv-key`. Will be created if it does not exist.
	// +optional
	PrivateKeySecret *string `json:"privateKeySecret,omitempty"`

	// Contains information details about this validator.
	// +optional
	Info *ValidatorInfo `json:"info,omitempty"`

	// Specifies configs and initialization commands for creating a new genesis.
	// +optional
	Init *GenesisInitConfig `json:"init,omitempty"`

	// TmKMS configuration for signing commits for this validator.
	// When configured, .spec.validator.privateKeySecret will not be mounted on the validator node.
	// +optional
	TmKMS *TmKMS `json:"tmKMS,omitempty"`

	// Indicates that cosmopilot should run create-validator tx to make this node a validator.
	// +optional
	CreateValidator *CreateValidatorConfig `json:"createValidator,omitempty"`
}
