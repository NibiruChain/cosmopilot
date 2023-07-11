package v1

import (
	"reflect"

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
	PhaseChainNodeInitData    ChainNodePhase = "InitializingData"
	PhaseChainNodeInitGenesis ChainNodePhase = "InitGenesis"
	PhaseChainNodeStarting    ChainNodePhase = "Starting"
	PhaseChainNodeRunning     ChainNodePhase = "Running"
	PhaseChainNodeSyncing     ChainNodePhase = "Syncing"
	PhaseChainNodeRestarting  ChainNodePhase = "Restarting"
	PhaseChainNodeError       ChainNodePhase = "Error"
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

func (chainNode *ChainNode) Equal(n *ChainNode) bool {
	if !reflect.DeepEqual(chainNode.Labels, n.Labels) {
		return false
	}
	if !reflect.DeepEqual(chainNode.Annotations, n.Annotations) {
		return false
	}

	if !reflect.DeepEqual(chainNode.Spec, n.Spec) {
		return false
	}

	return true
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
