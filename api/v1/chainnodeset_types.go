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
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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

	// List of ingresses to create for this ChainNodeSet. This allows to create ingresses targeting
	// multiple groups of nodes.
	// +optional
	Ingresses []GlobalIngressConfig `json:"ingresses,omitempty"`

	// Allows deploying seed nodes using Cosmoseed.
	// +optional
	Cosmoseed *CosmoseedConfig `json:"cosmoseed,omitempty"`
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

	// All scheduled/completed upgrades performed by cosmopilot on ChainNodes of this CHainNodeSet.
	// +optional
	Upgrades []Upgrade `json:"upgrades,omitempty"`

	// Last height read on the nodes by cosmopilot.
	// +optional
	LatestHeight int64 `json:"latestHeight,omitempty"`

	// Status of seed nodes (cosmoseed)
	Seeds []SeedStatus `json:"seeds,omitempty"`
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

	// Port to reach this node publicly.
	// +optional
	PublicPort int `json:"publicPort,omitempty"`

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

	// Indicates cosmopilot should run create-validator tx to make this node a validator.
	// +optional
	CreateValidator *CreateValidatorConfig `json:"createValidator,omitempty"`

	// Vertical Pod Autoscaling configuration for this node.
	// +optional
	VPA *VerticalAutoscalingConfig `json:"vpa,omitempty"`

	// Pod Disruption Budget configuration for the validator pod.
	// This is mainly useful in testnets where multiple validators might run in the same namespace.
	// In production mainnet environments, where typically only one validator runs per namespace,
	// this is rarely needed.
	// +optional
	PDB *PdbConfig `json:"pdb,omitempty"`

	// OverrideVersion will force validator to use the specified version.
	// NOTE: when this is set, cosmopilot will not upgrade the node, nor will set the version
	// based on upgrade history. For unsetting this, you will have to do it here and on
	// the ChainNode itself.
	// +optional
	OverrideVersion *string `json:"overrideVersion,omitempty"`

	// Indicates if an ingress should be created to access API endpoints of validator node
	// and configures it.
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`
}

// NodeGroupSpec sets chainnode configurations for a group.
type NodeGroupSpec struct {
	// Name of this group.
	Name string `json:"name"`

	// Number of ChainNode instances to run on this group.
	// +optional
	// +default=1
	// +kubebuilder:validation:Minimum=0
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

	// Ingress defines configuration for exposing API endpoints through a single shared Ingress,
	// which routes traffic to the group Service backing all nodes in this set. This results in
	// load-balanced access across all nodes (e.g., round-robin).
	// See IngressConfig for detailed endpoint and TLS settings.
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// IndividualIngresses defines configuration for exposing API endpoints through separate
	// Ingress resources per node in the set. Each Ingress routes traffic directly to its
	// corresponding node's Service (i.e., no load balancing across nodes).
	//
	// The same IngressConfig is reused for all nodes, but the `host` field will be prefixed
	// with the node index to generate unique subdomains. For example, if
	// `host = "fullnodes.cosmopilot.local"`, then node ingress domains will be:
	//   - 0.fullnodes.cosmopilot.local
	//   - 1.fullnodes.cosmopilot.local
	//   - etc.
	//
	// This mode allows targeting specific nodes individually.
	// +optional
	IndividualIngresses *IngressConfig `json:"individualIngresses,omitempty"`

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

	// Whether these nodes should inherit gas price from validator (if there is not configured on this ChainNodeSet)
	// Defaults to `true`.
	// +optional
	InheritValidatorGasPrice *bool `json:"inheritValidatorGasPrice,omitempty"`

	// Whether ChainNodeSet group label should be ignored on pod disruption checks.
	// This is useful to ensure no downtime globally or per global ingress, instead of just per group.
	// Defaults to `false`.
	// +optional
	IgnoreGroupOnDisruptionChecks *bool `json:"ignoreGroupOnDisruptionChecks,omitempty"`

	// Vertical Pod Autoscaling configuration for this node.
	// +optional
	VPA *VerticalAutoscalingConfig `json:"vpa,omitempty"`

	// Pod Disruption Budget configuration for this group.
	// +optional
	PDB *PdbConfig `json:"pdb,omitempty"`

	// Index of the node in the group to take volume snapshots from (if enabled).
	// Defaults to `0`.
	// +optional
	// +default=0
	// +kubebuilder:validation:Minimum=0
	SnapshotNodeIndex *int `json:"snapshotNodeIndex,omitempty"`

	// OverrideVersion will force this group to use the specified version.
	// NOTE: when this is set, cosmopilot will not upgrade the nodes, nor will set the version
	// based on upgrade history. For unsetting this, you will have to do it here and individually
	// per ChainNode
	// +optional
	OverrideVersion *string `json:"overrideVersion,omitempty"`
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

	// Enable EVM RPC endpoint.
	// +optional
	EnableEvmRPC bool `json:"enableEvmRPC,omitempty"`

	// Enable EVM RPC Websocket endpoint.
	// +optional
	EnableEvmRpcWs bool `json:"enableEvmRpcWS,omitempty"`

	// Host in which endpoints will be exposed. Endpoints are exposed on corresponding
	// subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at
	// `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`.
	Host string `json:"host"`

	// Annotations to be appended to the ingress.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Whether to disable TLS on ingress resource.
	// +optional
	DisableTLS bool `json:"disableTLS,omitempty"`

	// Name of the secret containing TLS certificate.
	// +optional
	TlsSecretName *string `json:"tlsSecretName,omitempty"`

	// GrpcAnnotations to be set on grpc ingress resource.
	// Defaults to nginx annotation `nginx.ingress.kubernetes.io/backend-protocol: GRPC`
	// if nginx ingress class is used.
	// +optional
	GrpcAnnotations map[string]string `json:"grpcAnnotations,omitempty"`

	// IngressClass specifies the ingress class to be used on ingresses
	// +optional
	// +default="nginx"
	IngressClass *string `json:"ingressClass,omitempty"`

	// UseInternalServices configures Ingress to route traffic directly to the node services,
	// bypassing Cosmoguard and any readiness checks. This is only recommended for debugging
	// or for private/internal traffic (e.g., when accessing the cluster over a VPN).
	// +optional
	// +default=false
	UseInternalServices *bool `json:"useInternalServices,omitempty"`
}

// GlobalIngressConfig specifies configurations for ingress to expose API endpoints of several groups of nodes.
type GlobalIngressConfig struct {
	// The name of this ingress
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Groups of nodes to which this ingress will point to.
	// +kubebuilder:validation:MinItems=1
	Groups []string `json:"groups"`

	// Enable RPC endpoint.
	// +optional
	EnableRPC bool `json:"enableRPC,omitempty"`

	// Enable gRPC endpoint.
	// +optional
	EnableGRPC bool `json:"enableGRPC,omitempty"`

	// Enable LCD endpoint.
	// +optional
	EnableLCD bool `json:"enableLCD,omitempty"`

	// Enable EVM RPC endpoint.
	// +optional
	EnableEvmRPC bool `json:"enableEvmRPC,omitempty"`

	// Enable EVM RPC Websocket endpoint.
	// +optional
	EnableEvmRpcWs bool `json:"enableEvmRpcWS,omitempty"`

	// Host in which endpoints will be exposed. Endpoints are exposed on corresponding
	// subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at
	// `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`.
	Host string `json:"host"`

	// Annotations to be set on ingress resource.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Whether to disable TLS on ingress resource.
	// +optional
	DisableTLS bool `json:"disableTLS,omitempty"`

	// Name of the secret containing TLS certificate.
	// +optional
	TlsSecretName *string `json:"tlsSecretName,omitempty"`

	// GrpcAnnotations to be set on grpc ingress resource.
	// Defaults to nginx annotation `nginx.ingress.kubernetes.io/backend-protocol: GRPC`
	// if nginx ingress class is used.
	// +optional
	GrpcAnnotations map[string]string `json:"grpcAnnotations,omitempty"`

	// IngressClass specifies the ingress class to be used on ingresses
	// +optional
	// +default="nginx"
	IngressClass *string `json:"ingressClass,omitempty"`

	// UseInternalServices configures Ingress to route traffic directly to the node services,
	// bypassing Cosmoguard and any readiness checks. This is only recommended for debugging
	// or for private/internal traffic (e.g., when accessing the cluster over a VPN).
	// +optional
	// +default=false
	UseInternalServices *bool `json:"useInternalServices,omitempty"`

	// ServicesOnly indicates that only global services should be created. No ingress resources will be created.
	// Useful for usage with custom controllers that have their own CRDs.
	// +optional
	ServicesOnly *bool `json:"servicesOnly,omitempty"`
}

type PdbConfig struct {
	// Whether to deploy a Pod Disruption Budget
	Enabled bool `json:"enabled"`

	// MinAvailable indicates minAvailable field set in PDB.
	// Defaults to the number of instances in the group minus 1,
	// i.e. it allows only a single disruption.
	// +optional
	MinAvailable *int `json:"minAvailable,omitempty"`
}

type CosmoseedConfig struct {
	// Whether to enable deployment of Cosmoseed.
	// If false or unset, no seed node instances will be created.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Number of seed node instances to deploy.
	// Defaults to 1.
	// +optional
	// +default=1
	// +kubebuilder:validation:Minimum=0
	Instances *int `json:"instances,omitempty"`

	// Configuration for exposing the P2P endpoint (e.g., via LoadBalancer or NodePort).
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// Compute Resources to be applied on the cosmoseed container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Used to enforce strict routability rules for peer addresses.
	// Set to false to only accept publicly routable IPs (recommended for public networks).
	// Set to true to allow local/private IPs (e.g., in testnets or dev environments).
	// Defaults to `false`.
	// +default=false
	// +optional
	AllowNonRoutable *bool `json:"allowNonRoutable,omitempty"`

	// Maximum number of inbound P2P connections.
	// Defaults to `2000`.
	// +default=2000
	// +optional
	MaxInboundPeers *int `json:"maxInboundPeers,omitempty"`

	// Maximum number of outbound P2P connections.
	// Defaults to `20`.
	// +default=20
	// +optional
	MaxOutboundPeers *int `json:"maxOutboundPeers,omitempty"`

	// Size of the internal peer queue used by dial workers in the PEX reactor.
	// This queue holds peers to be dialed; dial workers consume from it.
	// If the queue is full, new discovered peers may be discarded.
	// Use together with `DialWorkers` to control peer discovery throughput.
	// Defaults to `1000`.
	// +default=1000
	// +optional
	PeerQueueSize *int `json:"peerQueueSize,omitempty"`

	// Number of concurrent dialer workers used for outbound peer discovery.
	// Each worker fetches peers from the queue (`PeerQueueSize`) and attempts to dial them.
	// Higher values increase parallelism, but may increase CPU/network load.
	// Defaults to `20`.
	// +default=20
	// +optional
	DialWorkers *int `json:"dialWorkers,omitempty"`

	// Maximum size (in bytes) of packet message payloads over P2P.
	// Defaults to `1024`.
	// +default=1024
	// +optional
	MaxPacketMsgPayloadSize *int `json:"maxPacketMsgPayloadSize,omitempty"`

	// Additional seed nodes to append to the node’s default seed list.
	// Comma-separated list in the format `nodeID@ip:port`.
	// +optional
	AdditionalSeeds *string `json:"additionalSeeds,omitempty"`

	// Log level of cosmoseed.
	// Defaults to `info`.
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`

	// Ingress configuration for cosmoseed nodes.
	// +optional
	Ingress *CosmoseedIngressConfig `json:"ingress,omitempty"`
}

type SeedStatus struct {
	Name          string `json:"name"`
	ID            string `json:"id"`
	PublicAddress string `json:"publicAddress,omitempty"`
}

type CosmoseedIngressConfig struct {
	// Host in which cosmoseed nodes will be exposed.
	Host string `json:"host"`

	// Annotations to be appended to the ingress.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Whether to disable TLS on ingress resource.
	// +optional
	DisableTLS bool `json:"disableTLS,omitempty"`

	// Name of the secret containing TLS certificate.
	// +optional
	TlsSecretName *string `json:"tlsSecretName,omitempty"`

	// IngressClass specifies the ingress class to be used on ingresses
	// +optional
	// +default="nginx"
	IngressClass *string `json:"ingressClass,omitempty"`
}

type IndividualIngressConfig struct {
	Host string `json:"host"`
}
