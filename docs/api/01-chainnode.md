
### Custom Resources

* [ChainNode](#chainnode)

### Sub Resources

* [AppSpec](#appspec)
* [ChainNodeList](#chainnodelist)
* [ChainNodeSpec](#chainnodespec)
* [ChainNodeStatus](#chainnodestatus)
* [Config](#config)
* [GenesisConfig](#genesisconfig)
* [Persistence](#persistence)
* [SidecarSpec](#sidecarspec)

#### AppSpec

AppSpec specifies the source image and binary name of the app to run

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| image | Image indicates the docker image to be used | string | true |
| version | Version is the image tag to be used. Defaults to `latest`. | *string | false |
| imagePullPolicy | ImagePullPolicy indicates the desired pull policy when creating nodes. Defaults to `Always` if `version` is `latest` and `IfNotPresent` otherwise. | corev1.PullPolicy | false |
| app | App is the name of the binary of the application to be run | string | true |

[Back to Custom Resources](#custom-resources)

#### ChainNode

ChainNode is the Schema for the chainnodes API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [ChainNodeSpec](#chainnodespec) | false |
| status |  | [ChainNodeStatus](#chainnodestatus) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeList

ChainNodeList contains a list of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][ChainNode](#chainnode) | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSpec

ChainNodeSpec defines the desired state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| genesis | Genesis indicates where this node will get the genesis from | [GenesisConfig](#genesisconfig) | true |
| app | App specifies image and binary name of the chain application to run | [AppSpec](#appspec) | true |
| config | Config allows setting specific configurations for this node | *[Config](#config) | false |
| persistence | Persistence configures pvc for persisting data on nodes | *[Persistence](#persistence) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeStatus

ChainNodeStatus defines the observed state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| nodeID | NodeID show this node's ID | string | false |
| chainID | ChainID shows the chain ID | string | false |
| pvcSize | PvcSize shows the current size of the pvc of this node | string | false |

[Back to Custom Resources](#custom-resources)

#### Config

Config allows setting specific configurations for this node such has overrides to app.toml and config.toml

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| override | Override allows overriding configs on toml configuration files | *map[string]runtime.RawExtension | false |
| sidecars | Sidecars allow configuring additional containers to run alongside the node | [][SidecarSpec](#sidecarspec) | false |
| imagePullSecrets | ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this node. | []corev1.LocalObjectReference | false |

[Back to Custom Resources](#custom-resources)

#### GenesisConfig

GenesisConfig specifies how genesis will be retrieved

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| url | URL to download the genesis from. | *string | false |

[Back to Custom Resources](#custom-resources)

#### Persistence

Persistence configuration for this node

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| size | Size of the persistent volume for storing data. Defaults to `50Gi`. | *string | false |
| storageClass | StorageClassName specifies the name of the storage class to use to create persistent volumes. | *string | false |

[Back to Custom Resources](#custom-resources)

#### SidecarSpec

SidecarSpec allow configuring additional containers to run alongside the node

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name refers to the name to be assigned to the container | string | true |
| image | Image refers to the docker image to be used by the container | string | true |
| imagePullPolicy | ImagePullPolicy indicates the desired pull policy when creating nodes. Defaults to `Always` if `version` is `latest` and `IfNotPresent` otherwise. | corev1.PullPolicy | false |
| mountDataVolume | MountDataVolume indicates where data volume will be mounted on this container. It is not mounted if not specified. | *string | false |
| command | Command to be run by this container. Defaults to entrypoint defined in image. | []string | false |
| args | Args to be passed to this container. Defaults to cmd defined in image. | []string | false |
| env | Env sets environment variables to be passed to this container. | []corev1.EnvVar | false |

[Back to Custom Resources](#custom-resources)
