
### Custom Resources

* [ChainNode](#chainnode)

### Sub Resources

* [ChainNodeList](#chainnodelist)
* [ChainNodeSpec](#chainnodespec)
* [ChainNodeStatus](#chainnodestatus)
* [Config](#config)
* [ExposeConfig](#exposeconfig)
* [Persistence](#persistence)
* [SidecarSpec](#sidecarspec)
* [ValidatorConfig](#validatorconfig)

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
| genesis | Genesis indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified. | *GenesisConfig | true |
| app | App specifies image and binary name of the chain application to run | AppSpec | true |
| config | Config allows setting specific configurations for this node | *[Config](#config) | false |
| persistence | Persistence configures pvc for persisting data on nodes | *[Persistence](#persistence) | false |
| validator | Validator configures this node as a validator and configures it. | *[ValidatorConfig](#validatorconfig) | false |
| autoDiscoverPeers | AutoDiscoverPeers ensures peers with same chain ID are connected with each other. By default, it is enabled. | *bool | false |
| peers | Peers are additional persistent peers that should be added to this node. | []Peer | false |
| expose | Expose specifies which node endpoints are exposed and how they are exposed | *[ExposeConfig](#exposeconfig) | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints | *corev1.Affinity | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeStatus

ChainNodeStatus defines the observed state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Phase indicates the current phase for this ChainNode. | ChainNodePhase | false |
| nodeID | NodeID show this node's ID | string | false |
| ip | IP of this node. | string | false |
| publicAddress | PublicAddress for p2p when enabled. | string | false |
| chainID | ChainID shows the chain ID | string | false |
| pvcSize | PvcSize shows the current size of the pvc of this node | string | false |
| dataUsage | DataUsage shows the percentage of data usage. | string | false |
| validator | Validator indicates if this node is a validator. | bool | true |
| accountAddress | AccountAddress is the account address of this validator. Omitted when not a validator | string | false |
| validatorAddress | ValidatorAddress is the valoper address of this validator. Omitted when not a validator | string | false |
| jailed | Jailed indicates if this validator is jailed. Always false if not a validator node. | bool | true |
| appVersion | AppVersion is the application version currently deployed | string | false |

[Back to Custom Resources](#custom-resources)

#### Config

Config allows setting specific configurations for a chainnode such as overrides to app.toml and config.toml

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| override | Override allows overriding configs on toml configuration files | *map[string]runtime.RawExtension | false |
| sidecars | Sidecars allow configuring additional containers to run alongside the node | [][SidecarSpec](#sidecarspec) | false |
| imagePullSecrets | ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this node. | []corev1.LocalObjectReference | false |
| blockThreshold | BlockThreshold specifies the time to wait for a block before considering node unhealthy | *string | false |
| reconcilePeriod | ReconcilePeriod is the period at which a reconcile loop will happen for this ChainNode. Defaults to `1m`. | *string | false |

[Back to Custom Resources](#custom-resources)

#### ExposeConfig



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| p2p | P2P indicates whether to expose p2p endpoint for this node. Defaults to `false`. | *bool | false |
| p2pServiceType | P2pServiceType indicates how p2p port will be exposed. Either `LoadBalancer` or `NodePort`. Defaults to `NodePort`. | *corev1.ServiceType | false |

[Back to Custom Resources](#custom-resources)

#### Persistence

Persistence configuration for this node

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| size | Size of the persistent volume for storing data. Can't be updated when autoResize is enabled. Defaults to `50Gi`. | *string | false |
| storageClass | StorageClassName specifies the name of the storage class to use to create persistent volumes. | *string | false |
| autoResize | AutoResize specifies configurations to automatically resize PVC. Defaults to `true`. | *bool | false |
| autoResizeThreshold | AutoResizeThreshold is the percentage of data usage at which an auto-resize event should occur. Defaults to `80`. | *int | false |
| autoResizeIncrement | AutoResizeIncrement specifies the size increment on each auto-resize event. Defaults to `50Gi`. | *string | false |
| autoResizeMaxSize | AutoResizeMaxSize specifies the maximum size the PVC can have. Defaults to `2Ti`. | *string | false |
| additionalInitCommands | AdditionalInitCommands are additional commands to run on data initialization. Useful for downloading and extracting snapshots. App home is at `/home/app` and data dir is at `/home/app/data`. There is also `/temp`, a temporary volume shared by all init containers. | []InitCommand | false |

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

#### ValidatorConfig

ValidatorConfig turns this node into a validator and specifies how it will do it.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | PrivateKeySecret indicates the secret containing the private key to be use by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Info contains information details about this validator. | *ValidatorInfo | false |
| init | Init specifies configs and initialization commands for creating a new chain and its genesis. | *GenesisInitConfig | false |
| tmKMS | TmKMS configuration for signing commits for this validator. When configured, .spec.validator.privateKeySecret will not be mounted on the validator node. | *TmKMS | false |

[Back to Custom Resources](#custom-resources)
