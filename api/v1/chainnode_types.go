package v1

import (
	"reflect"

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
