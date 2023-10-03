package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ChainNodeSet{}, &ChainNodeSetList{})
}

// ChainNodeSetPhase is a label for the condition of a nodeset at the current time.
type ChainNodeSetPhase string

// These are the valid phases for nodesets.
const (
	PhaseChainNodeSetRunning    ChainNodeSetPhase = "Running"
	PhaseChainNodeSetInitialing ChainNodeSetPhase = "Initializing"
)

//+kubebuilder:object:root=true

// ChainNodeSetList contains a list of ChainNodeSet
type ChainNodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChainNodeSet `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.appVersion`
//+kubebuilder:printcolumn:name="ChainID",type=string,JSONPath=`.status.chainID`
//+kubebuilder:printcolumn:name="Instances",type=integer,JSONPath=`.status.instances`

// ChainNodeSet is the Schema for the chainnodesets API
type ChainNodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainNodeSetSpec   `json:"spec,omitempty"`
	Status ChainNodeSetStatus `json:"status,omitempty"`
}

// ChainNodeSetSpec defines the desired state of ChainNode
type ChainNodeSetSpec struct {
	// App specifies image and binary name of the chain application to run
	App AppSpec `json:"app"`

	// Genesis indicates where nodes from this set will get the genesis from. Can be omitted when .spec.validator.init is specified.
	// +optional
	Genesis *GenesisConfig `json:"genesis"`

	// Validator configures a validator node and configures it.
	// +optional
	Validator *NodeSetValidatorConfig `json:"validator,omitempty"`

	// Nodes indicates the list of groups of chainnodes to be run
	Nodes []NodeGroupSpec `json:"nodes"`

	// ServiceMonitor allows deploying prometheus service monitor for all ChainNodes in this ChainNodeSet.
	// ServiceMonitor config on ChainNode overrides this one.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// ChainNodeSetStatus defines the observed state of ChainNodeSet
type ChainNodeSetStatus struct {
	// Phase indicates the current phase for this ChainNodeSet.
	// +optional
	Phase ChainNodeSetPhase `json:"phase,omitempty"`

	// ChainID shows the chain ID
	// +optional
	ChainID string `json:"chainID,omitempty"`

	// Instances indicates the total number of chainnode instances on this set
	// +optional
	Instances int `json:"instances,omitempty"`

	// AppVersion is the application version currently deployed
	AppVersion string `json:"appVersion,omitempty"`

	// Nodes indicates which nodes are available on this nodeset. Excludes validator node.
	Nodes []ChainNodeSetNodeStatus `json:"nodes,omitempty"`

	// ValidatorAddress is the valoper address of the validator in this ChainNodeSet if one is available.
	// Omitted when no validator is present in the ChainNodeSet.
	// +optional
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// ValidatorStatus indicates the current status of validator if this node is one.
	ValidatorStatus ValidatorStatus `json:"validatorStatus,omitempty"`

	// PubKey of the validator.
	// +optional
	PubKey string `json:"pubKey,omitempty"`
}

type ChainNodeSetNodeStatus struct {
	// Name is the name of the node.
	Name string `json:"name"`

	// Public indicates whether this node can be accessed publicly.
	Public bool `json:"public"`

	// Seed indicates if this node is running in seed mode.
	Seed bool `json:"seed"`

	// ID is the node ID of this node.
	ID string `json:"id"`

	// Address is the hostname or IP address to reach this node internally.
	Address string `json:"address"`

	// PublicAddress is the hostname or IP address to reach this node publicly.
	// +optional
	PublicAddress string `json:"publicAddress,omitempty"`

	// Port is the P2P port for connecting to this node.
	Port int `json:"port"`
}

// NodeSetValidatorConfig turns this node into a validator and specifies how it will do it.
type NodeSetValidatorConfig struct {
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

	// Config allows setting specific configurations for this node.
	// +optional
	Config *Config `json:"config,omitempty"`

	// Persistence configures pvc for persisting data for this node.
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

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

	// TmKMS configuration for signing commits for this validator.
	// When configured, .spec.validator.privateKeySecret will not be mounted on the validator node.
	// +optional
	TmKMS *TmKMS `json:"tmKMS,omitempty"`

	// StateSyncRestore configures this node to find a state-sync snapshot on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`

	// CreateValidator indicates that operator should run create-validator tx to make this node a validator.
	// +optional
	CreateValidator *CreateValidatorConfig `json:"createValidator,omitempty"`
}

// NodeGroupSpec sets chainnode configurations for a group
type NodeGroupSpec struct {
	// Name refers the name of this group
	Name string `json:"name"`

	// Instances indicates the number of chainnode instances to run on this group
	// +optional
	// +default=1
	Instances *int `json:"instances,omitempty"`

	// Config allows setting specific configurations for this node
	// +optional
	Config *Config `json:"config,omitempty"`

	// Persistence configures pvc for persisting data on nodes
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

	// Peers are additional persistent peers that should be added to this node.
	// +optional
	Peers []Peer `json:"peers,omitempty"`

	// Expose specifies which node endpoints are exposed and how they are exposed
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// Ingress indicates if an ingress should be created to access API endpoints of these nodes and configures it.
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

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

	// StateSyncRestore configures this node to find a state-sync snapshot on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`
}

// IngressConfig specifies configurations for ingress to expose API endpoints
type IngressConfig struct {
	// EnableRPC enable RPC endpoint.
	// +optional
	EnableRPC bool `json:"enableRPC,omitempty"`

	// EnableGRPC enable gRPC endpoint.
	// +optional
	EnableGRPC bool `json:"enableGRPC,omitempty"`

	// EnableLCD enable LCD endpoint.
	// +optional
	EnableLCD bool `json:"enableLCD,omitempty"`

	// Host specifies the host in which endpoints will be exposed. Endpoints are exposed on corresponding
	// subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at
	// `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`.
	Host string `json:"host"`

	// Annotations to be appended to the ingress.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
