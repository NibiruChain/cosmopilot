
### Custom Resources

* [ChainNode](#chainnode)

### Sub Resources

* [ChainNodeList](#chainnodelist)
* [ChainNodeSpec](#chainnodespec)
* [ChainNodeStatus](#chainnodestatus)
* [ValidatorConfig](#validatorconfig)

#### ChainNode

ChainNode is the Schema for the chainnodes API.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [ChainNodeSpec](#chainnodespec) | false |
| status |  | [ChainNodeStatus](#chainnodestatus) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeList

ChainNodeList contains a list of ChainNode.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][ChainNode](#chainnode) | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSpec

ChainNodeSpec defines the desired state of ChainNode.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| genesis | Indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified. | *GenesisConfig | true |
| app | Specifies image, version and binary name of the chain application to run. It also allows to schedule upgrades, or setting/updating the image for an on-chain upgrade. | AppSpec | true |
| config | Allows setting specific configurations for this node. | *Config | false |
| persistence | Configures PVC for persisting data. Automated data snapshots can also be configured in this section. | *Persistence | false |
| validator | Indicates this node is going to be a validator and allows configuring it. | *[ValidatorConfig](#validatorconfig) | false |
| autoDiscoverPeers | Ensures peers with same chain ID are connected with each other. Enabled by default. | *bool | false |
| stateSyncRestore | Configures this node to find a state-sync snapshot on the network and restore from it. This is disabled by default. | *bool | false |
| peers | Additional persistent peers that should be added to this node. | []Peer | false |
| expose | Allows exposing P2P traffic to public. | *ExposeConfig | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | Selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints. | *corev1.Affinity | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeStatus

ChainNodeStatus defines the observed state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Indicates the current phase for this ChainNode. | ChainNodePhase | false |
| nodeID | Indicates this node's ID. | string | false |
| ip | Internal IP address of this node. | string | false |
| publicAddress | Public address for P2P when enabled. | string | false |
| chainID | Indicates the chain ID. | string | false |
| pvcSize | Current size of the data PVC for this node. | string | false |
| dataUsage | Usage percentage of data volume. | string | false |
| validator | Indicates if this node is a validator. | bool | true |
| accountAddress | Account address of this validator. Omitted when not a validator. | string | false |
| validatorAddress | Validator address is the valoper address of this validator. Omitted when not a validator. | string | false |
| jailed | Indicates if this validator is jailed. Always false if not a validator node. | bool | false |
| appVersion | Application version currently deployed. | string | false |
| latestHeight | Last height read on the node by the operator. | int64 | false |
| seedMode | Indicates if this node is running with seed mode enabled. | bool | false |
| upgrades | All scheduled/completed upgrades performed by the operator on this ChainNode. | []Upgrade | false |
| pubKey | Public key of the validator. | string | false |
| validatorStatus | Indicates the current status of validator if this node is one. | ValidatorStatus | false |

[Back to Custom Resources](#custom-resources)

#### ValidatorConfig

ValidatorConfig contains the configuration for running a node as validator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | Indicates the secret containing the private key to be used by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Contains information details about this validator. | *ValidatorInfo | false |
| init | Specifies configs and initialization commands for creating a new genesis. | *GenesisInitConfig | false |
| tmKMS | TmKMS configuration for signing commits for this validator. When configured, .spec.validator.privateKeySecret will not be mounted on the validator node. | *TmKMS | false |
| createValidator | Indicates that operator should run create-validator tx to make this node a validator. | *CreateValidatorConfig | false |

[Back to Custom Resources](#custom-resources)
