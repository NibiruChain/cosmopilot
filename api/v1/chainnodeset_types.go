package v1

import (
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
	Validator *ValidatorConfig `json:"validator,omitempty"`

	// Nodes indicates the list of groups of chainnodes to be run
	Nodes []NodeGroupSpec `json:"nodes"`
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
