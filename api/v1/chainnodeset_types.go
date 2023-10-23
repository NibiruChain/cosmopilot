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

// These are the valid phases for ChainNodeSets.
const (
	PhaseChainNodeSetRunning    ChainNodeSetPhase = "Running"
	PhaseChainNodeSetInitialing ChainNodeSetPhase = "Initializing"
)

//+kubebuilder:object:root=true

// ChainNodeSetList contains a list of ChainNodeSet.
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
//+kubebuilder:printcolumn:name="LatestHeight",type=integer,JSONPath=`.status.latestHeight`
//+kubebuilder:printcolumn:name="Instances",type=integer,JSONPath=`.status.instances`

// ChainNodeSet is the Schema for the chainnodesets API.
type ChainNodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainNodeSetSpec   `json:"spec,omitempty"`
	Status ChainNodeSetStatus `json:"status,omitempty"`
}

// ChainNodeSetSpec defines the desired state of ChainNode.
type ChainNodeSetSpec struct {
	// Specifies image, version and binary name of the chain application to run. It also allows to schedule upgrades,
	// or setting/updating the image for an on-chain upgrade.
	App AppSpec `json:"app"`

	// Indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified.
	// +optional
	Genesis *GenesisConfig `json:"genesis"`

	// Indicates this node set will run a validator and allows configuring it.
	// +optional
	Validator *NodeSetValidatorConfig `json:"validator,omitempty"`

	// List of groups of ChainNodes to be run.
	Nodes []NodeGroupSpec `json:"nodes"`

	// Allows deploying prometheus service monitor for all ChainNodes in this ChainNodeSet.
	// ServiceMonitor config on ChainNode overrides this one.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// ChainNodeSetStatus defines the observed state of ChainNodeSet.
type ChainNodeSetStatus struct {
	// Indicates the current phase for this ChainNodeSet.
	// +optional
	Phase ChainNodeSetPhase `json:"phase,omitempty"`

	// Indicates the chain ID.
	// +optional
	ChainID string `json:"chainID,omitempty"`

	// Indicates the total number of ChainNode instances on this ChainNodeSet.
	// +optional
	Instances int `json:"instances,omitempty"`

	// The application version currently deployed.
	// +optional
	AppVersion string `json:"appVersion,omitempty"`

	// Nodes available on this nodeset. Excludes validator node.
	// +optional
	Nodes []ChainNodeSetNodeStatus `json:"nodes,omitempty"`

	// Validator address of the validator in this ChainNodeSet if one is available.
	// Omitted when no validator is present in the ChainNodeSet.
	// +optional
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// Current status of validator if this ChainNodeSet has one.
	// +optional
	ValidatorStatus ValidatorStatus `json:"validatorStatus,omitempty"`

	// Public key of the validator if this ChainNodeSet has one.
	// +optional
	PubKey string `json:"pubKey,omitempty"`

	// All scheduled/completed upgrades performed by the operator on ChainNodes of this CHainNodeSet.
	// +optional
	Upgrades []Upgrade `json:"upgrades,omitempty"`

	// Last height read on the nodes by the operator.
	// +optional
	LatestHeight int64 `json:"latestHeight,omitempty"`
}

// ChainNodeSetNodeStatus contains information about a node running on this ChainNodeSet.
type ChainNodeSetNodeStatus struct {
	// Name of the node.
	Name string `json:"name"`

	// Whether this node can be accessed publicly.
	Public bool `json:"public"`

	// Indicates if this node is running in seed mode.
	Seed bool `json:"seed"`

	// ID of this node.
	ID string `json:"id"`

	// Hostname or IP address to reach this node internally.
	Address string `json:"address"`

	// Hostname or IP address to reach this node publicly.
	// +optional
	PublicAddress string `json:"publicAddress,omitempty"`

	// P2P port for connecting to this node.
	Port int `json:"port"`

	// Group to which this ChainNode belongs.
	// +optional
	Group string `json:"group,omitempty"`
}

// NodeSetValidatorConfig contains validator configurations.
type NodeSetValidatorConfig struct {
	// Secret containing the private key to be used by this validator.
	// Defaults to `<chainnode>-priv-key`. Will be created if it does not exist.
	// +optional
	PrivateKeySecret *string `json:"privateKeySecret,omitempty"`

	// Contains information details about the validator.
	// +optional
	Info *ValidatorInfo `json:"info,omitempty"`

	// Specifies configs and initialization commands for creating a new genesis.
	// +optional
	Init *GenesisInitConfig `json:"init,omitempty"`

	// Allows setting specific configurations for the validator.
	// +optional
	Config *Config `json:"config,omitempty"`

	// Configures PVC for persisting data. Automated data snapshots can also be configured in
	// this section.
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

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

	// TmKMS configuration for signing commits for this validator.
	// When configured, .spec.validator.privateKeySecret will not be mounted on the validator node.
	// +optional
	TmKMS *TmKMS `json:"tmKMS,omitempty"`

	// Configures this node to find a state-sync snapshot on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`

	// Indicates that operator should run create-validator tx to make this node a validator.
	// +optional
	CreateValidator *CreateValidatorConfig `json:"createValidator,omitempty"`
}

// NodeGroupSpec sets chainnode configurations for a group.
type NodeGroupSpec struct {
	// Name of this group.
	Name string `json:"name"`

	// Number of ChainNode instances to run on this group.
	// +optional
	// +default=1
	Instances *int `json:"instances,omitempty"`

	// Specific configurations for these nodes.
	// +optional
	Config *Config `json:"config,omitempty"`

	// Configures PVC for persisting data. Automated data snapshots can also be configured in
	// this section.
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`

	// Additional persistent peers that should be added to these nodes.
	// +optional
	Peers []Peer `json:"peers,omitempty"`

	// Allows exposing P2P traffic to public.
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// Indicates if an ingress should be created to access API endpoints of these nodes and configures it.
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

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

	// Configures these nodes to find state-sync snapshots on the network and restore from it.
	// This is disabled by default.
	// +optional
	StateSyncRestore *bool `json:"stateSyncRestore,omitempty"`
}

// IngressConfig specifies configurations for ingress to expose API endpoints.
type IngressConfig struct {
	// Enable RPC endpoint.
	// +optional
	EnableRPC bool `json:"enableRPC,omitempty"`

	// Enable gRPC endpoint.
	// +optional
	EnableGRPC bool `json:"enableGRPC,omitempty"`

	// Enable LCD endpoint.
	// +optional
	EnableLCD bool `json:"enableLCD,omitempty"`

	// Host in which endpoints will be exposed. Endpoints are exposed on corresponding
	// subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at
	// `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`.
	Host string `json:"host"`

	// Annotations to be appended to the ingress.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
