
### Custom Resources

* [ChainNodeSet](#chainnodeset)

### Sub Resources

* [ChainNodeSetList](#chainnodesetlist)
* [ChainNodeSetNodeStatus](#chainnodesetnodestatus)
* [ChainNodeSetSpec](#chainnodesetspec)
* [ChainNodeSetStatus](#chainnodesetstatus)
* [IngressConfig](#ingressconfig)
* [NodeGroupSpec](#nodegroupspec)
* [NodeSetValidatorConfig](#nodesetvalidatorconfig)

#### ChainNodeSet

ChainNodeSet is the Schema for the chainnodesets API.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [ChainNodeSetSpec](#chainnodesetspec) | false |
| status |  | [ChainNodeSetStatus](#chainnodesetstatus) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetList

ChainNodeSetList contains a list of ChainNodeSet.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][ChainNodeSet](#chainnodeset) | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetNodeStatus

ChainNodeSetNodeStatus contains information about a node running on this ChainNodeSet.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of the node. | string | true |
| public | Whether this node can be accessed publicly. | bool | true |
| seed | Indicates if this node is running in seed mode. | bool | true |
| id | ID of this node. | string | true |
| address | Hostname or IP address to reach this node internally. | string | true |
| publicAddress | Hostname or IP address to reach this node publicly. | string | false |
| port | P2P port for connecting to this node. | int | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetSpec

ChainNodeSetSpec defines the desired state of ChainNode.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| app | Specifies image, version and binary name of the chain application to run. It also allows to schedule upgrades, or setting/updating the image for an on-chain upgrade. | AppSpec | true |
| genesis | Indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified. | *GenesisConfig | true |
| validator | Indicates this node set will run a validator and allows configuring it. | *[NodeSetValidatorConfig](#nodesetvalidatorconfig) | false |
| nodes | List of groups of ChainNodes to be run. | [][NodeGroupSpec](#nodegroupspec) | true |
| serviceMonitor | Allows deploying prometheus service monitor for all ChainNodes in this ChainNodeSet. ServiceMonitor config on ChainNode overrides this one. | *ServiceMonitorSpec | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetStatus

ChainNodeSetStatus defines the observed state of ChainNodeSet.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Indicates the current phase for this ChainNodeSet. | ChainNodeSetPhase | false |
| chainID | Indicates the chain ID. | string | false |
| instances | Indicates the total number of ChainNode instances on this ChainNodeSet. | int | false |
| appVersion | The application version currently deployed. | string | false |
| nodes | Nodes available on this nodeset. Excludes validator node. | [][ChainNodeSetNodeStatus](#chainnodesetnodestatus) | false |
| validatorAddress | Validator address of the validator in this ChainNodeSet if one is available. Omitted when no validator is present in the ChainNodeSet. | string | false |
| validatorStatus | Current status of validator if this ChainNodeSet has one. | ValidatorStatus | false |
| pubKey | Public key of the validator if this ChainNodeSet has one. | string | false |
| upgrades | All scheduled/completed upgrades performed by the operator on ChainNodes of this CHainNodeSet. | []Upgrade | false |
| latestHeight | Last height read on the nodes by the operator. | int64 | false |

[Back to Custom Resources](#custom-resources)

#### IngressConfig

IngressConfig specifies configurations for ingress to expose API endpoints.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| enableRPC | Enable RPC endpoint. | bool | false |
| enableGRPC | Enable gRPC endpoint. | bool | false |
| enableLCD | Enable LCD endpoint. | bool | false |
| host | Host in which endpoints will be exposed. Endpoints are exposed on corresponding subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`. | string | true |
| annotations | Annotations to be appended to the ingress. | map[string]string | false |

[Back to Custom Resources](#custom-resources)

#### NodeGroupSpec

NodeGroupSpec sets chainnode configurations for a group.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of this group. | string | true |
| instances | Number of ChainNode instances to run on this group. | *int | false |
| config | Specific configurations for these nodes. | *Config | false |
| persistence | Configures PVC for persisting data. Automated data snapshots can also be configured in this section. | *Persistence | false |
| peers | Additional persistent peers that should be added to these nodes. | []Peer | false |
| expose | Allows exposing P2P traffic to public. | *ExposeConfig | false |
| ingress | Indicates if an ingress should be created to access API endpoints of these nodes and configures it. | *[IngressConfig](#ingressconfig) | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | Selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints. | *corev1.Affinity | false |
| stateSyncRestore | Configures these nodes to find state-sync snapshots on the network and restore from it. This is disabled by default. | *bool | false |

[Back to Custom Resources](#custom-resources)

#### NodeSetValidatorConfig

NodeSetValidatorConfig contains validator configurations.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | Secret containing the private key to be used by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Contains information details about the validator. | *ValidatorInfo | false |
| init | Specifies configs and initialization commands for creating a new genesis. | *GenesisInitConfig | false |
| config | Allows setting specific configurations for the validator. | *Config | false |
| persistence | Configures PVC for persisting data. Automated data snapshots can also be configured in this section. | *Persistence | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | Selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints. | *corev1.Affinity | false |
| tmKMS | TmKMS configuration for signing commits for this validator. When configured, .spec.validator.privateKeySecret will not be mounted on the validator node. | *TmKMS | false |
| stateSyncRestore | Configures this node to find a state-sync snapshot on the network and restore from it. This is disabled by default. | *bool | false |
| createValidator | Indicates that operator should run create-validator tx to make this node a validator. | *CreateValidatorConfig | false |

[Back to Custom Resources](#custom-resources)
