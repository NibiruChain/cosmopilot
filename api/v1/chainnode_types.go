package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(&ChainNode{}, &ChainNodeList{})
}

//+kubebuilder:object:root=true

// ChainNodeList contains a list of ChainNode
type ChainNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChainNode `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ChainID",type=string,JSONPath=`.status.chainID`

// ChainNode is the Schema for the chainnodes API
type ChainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChainNodeSpec   `json:"spec,omitempty"`
	Status ChainNodeStatus `json:"status,omitempty"`
}

// ChainNodeSpec defines the desired state of ChainNode
type ChainNodeSpec struct {
	// Genesis indicates where this node will get the genesis from
	Genesis GenesisConfig `json:"genesis"`

	// App specifies image and binary name of the chain application to run
	App AppSpec `json:"app"`

	// Config allows setting specific configurations for this node
	// +optional
	Config *Config `json:"config,omitempty"`

	// Persistence configures pvc for persisting data on nodes
	// +optional
	Persistence *Persistence `json:"persistence,omitempty"`
}

// ChainNodeStatus defines the observed state of ChainNode
type ChainNodeStatus struct {
	// NodeID show this node's ID
	// +optional
	NodeID string `json:"nodeID,omitempty"`

	// ChainID shows the chain ID
	// +optional
	ChainID string `json:"chainID,omitempty"`

	// PvcSize shows the current size of the pvc of this node
	// +optional
	PvcSize string `json:"pvcSize,omitempty"`
}

// GenesisConfig specifies how genesis will be retrieved
type GenesisConfig struct {
	// URL to download the genesis from.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Url *string `json:"url,omitempty"`
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
}

// Persistence configuration for this node
type Persistence struct {
	// Size of the persistent volume for storing data. Defaults to `50Gi`.
	// +optional
	// +default="50Gi"
	// +kubebuilder:validation:MinLength=1
	Size *string `json:"size,omitempty"`

	// StorageClassName specifies the name of the storage class to use
	// to create persistent volumes.
	// +optional
	StorageClassName *string `json:"storageClass,omitempty"`
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
