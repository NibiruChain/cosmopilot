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

// These are the valid phases for nodes.
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
//+kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.appVersion`
//+kubebuilder:printcolumn:name="ChainID",type=string,JSONPath=`.status.chainID`
//+kubebuilder:printcolumn:name="Validator",type=boolean,JSONPath=`.status.validator`
//+kubebuilder:printcolumn:name="BondStatus",type=string,JSONPath=`.status.validatorStatus`
//+kubebuilder:printcolumn:name="Jailed",type=boolean,JSONPath=`.status.jailed`
//+kubebuilder:printcolumn:name="DataUsage",type=string,JSONPath=`.status.dataUsage`
//+kubebuilder:printcolumn:name="LatestHeight",type=integer,JSONPath=`.status.latestHeight`
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

	// StateSyncRestore configures this node to find a state-sync snapshot on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`

	// Peers are additional persistent peers that should be added to this node.
	// +optional
	Peers []Peer `json:"peers,omitempty"`

	// Expose specifies which node endpoints are exposed and how they are exposed
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// Compute Resources required by the app container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// ChainNodeStatus defines the observed state of ChainNode
type ChainNodeStatus struct {
	// Phase indicates the current phase for this ChainNode.
	// +optional
	Phase ChainNodePhase `json:"phase,omitempty"`

	// NodeID show this node's ID
	// +optional
	NodeID string `json:"nodeID,omitempty"`

	// IP of this node.
	// +optional
	IP string `json:"ip,omitempty"`

	// PublicAddress for p2p when enabled.
	// +optional
	PublicAddress string `json:"publicAddress,omitempty"`

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

	// AppVersion is the application version currently deployed
	AppVersion string `json:"appVersion,omitempty"`

	// LatestHeight is the last height read on the node by the operator.
	LatestHeight int64 `json:"latestHeight,omitempty"`

	// SeedMode indicates if this node is running with seed mode enabled.
	SeedMode bool `json:"seedMode,omitempty"`

	// Upgrades contains all scheduled/completed upgrades performed by the operator on this ChainNode.
	// +optional
	Upgrades []Upgrade `json:"upgrades,omitempty"`

	// PubKey of the validator.
	// +optional
	PubKey string `json:"pubKey,omitempty"`

	// ValidatorStatus indicates the current status of validator if this node is one.
	ValidatorStatus ValidatorStatus `json:"validatorStatus,omitempty"`
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

	// TmKMS configuration for signing commits for this validator.
	// When configured, .spec.validator.privateKeySecret will not be mounted on the validator node.
	// +optional
	TmKMS *TmKMS `json:"tmKMS,omitempty"`

	// CreateValidator indicates that operator should run create-validator tx to make this node a validator.
	// +optional
	CreateValidator *CreateValidatorConfig `json:"createValidator,omitempty"`
}
