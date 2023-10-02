
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

ChainNodeSet is the Schema for the chainnodesets API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [ChainNodeSetSpec](#chainnodesetspec) | false |
| status |  | [ChainNodeSetStatus](#chainnodesetstatus) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetList

ChainNodeSetList contains a list of ChainNodeSet

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][ChainNodeSet](#chainnodeset) | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetNodeStatus



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name is the name of the node. | string | true |
| public | Public indicates whether this node can be accessed publicly. | bool | true |
| seed | Seed indicates if this node is running in seed mode. | bool | true |
| id | ID is the node ID of this node. | string | true |
| address | Address is the hostname or IP address to reach this node internally. | string | true |
| publicAddress | PublicAddress is the hostname or IP address to reach this node publicly. | string | false |
| port | Port is the P2P port for connecting to this node. | int | true |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetSpec

ChainNodeSetSpec defines the desired state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| app | App specifies image and binary name of the chain application to run | AppSpec | true |
| genesis | Genesis indicates where nodes from this set will get the genesis from. Can be omitted when .spec.validator.init is specified. | *GenesisConfig | true |
| validator | Validator configures a validator node and configures it. | *[NodeSetValidatorConfig](#nodesetvalidatorconfig) | false |
| nodes | Nodes indicates the list of groups of chainnodes to be run | [][NodeGroupSpec](#nodegroupspec) | true |
| serviceMonitor | ServiceMonitor allows deploying prometheus service monitor for all ChainNodes in this ChainNodeSet. ServiceMonitor config on ChainNode overrides this one. | *ServiceMonitorSpec | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeSetStatus

ChainNodeSetStatus defines the observed state of ChainNodeSet

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| phase | Phase indicates the current phase for this ChainNodeSet. | ChainNodeSetPhase | false |
| chainID | ChainID shows the chain ID | string | false |
| instances | Instances indicates the total number of chainnode instances on this set | int | false |
| appVersion | AppVersion is the application version currently deployed | string | false |
| nodes | Nodes indicates which nodes are available on this nodeset. Excludes validator node. | [][ChainNodeSetNodeStatus](#chainnodesetnodestatus) | false |
| validatorAddress | ValidatorAddress is the valoper address of the validator in this ChainNodeSet if one is available. Omitted when no validator is present in the ChainNodeSet. | string | false |

[Back to Custom Resources](#custom-resources)

#### IngressConfig

IngressConfig specifies configurations for ingress to expose API endpoints

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| enableRPC | EnableRPC enable RPC endpoint. | bool | false |
| enableGRPC | EnableGRPC enable gRPC endpoint. | bool | false |
| enableLCD | EnableLCD enable LCD endpoint. | bool | false |
| host | Host specifies the host in which endpoints will be exposed. Endpoints are exposed on corresponding subdomain of this host. An example host `nodes.example.com` will have endpoints exposed at `rpc.nodes.example.com`, `grpc.nodes.example.com` and `lcd.nodes.example.com`. | string | true |
| annotations | Annotations to be appended to the ingress. | map[string]string | false |

[Back to Custom Resources](#custom-resources)

#### NodeGroupSpec

NodeGroupSpec sets chainnode configurations for a group

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name refers the name of this group | string | true |
| instances | Instances indicates the number of chainnode instances to run on this group | *int | false |
| config | Config allows setting specific configurations for this node | *Config | false |
| persistence | Persistence configures pvc for persisting data on nodes | *Persistence | false |
| peers | Peers are additional persistent peers that should be added to this node. | []Peer | false |
| expose | Expose specifies which node endpoints are exposed and how they are exposed | *ExposeConfig | false |
| ingress | Ingress indicates if an ingress should be created to access API endpoints of these nodes and configures it. | *[IngressConfig](#ingressconfig) | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints | *corev1.Affinity | false |
| stateSyncRestore | StateSyncRestore configures this node to find a state-sync snapshot on the network and restore from it. This is disabled by default. | *bool | false |

[Back to Custom Resources](#custom-resources)

#### NodeSetValidatorConfig

NodeSetValidatorConfig turns this node into a validator and specifies how it will do it.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | PrivateKeySecret indicates the secret containing the private key to be use by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Info contains information details about this validator. | *ValidatorInfo | false |
| init | Init specifies configs and initialization commands for creating a new chain and its genesis. | *GenesisInitConfig | false |
| config | Config allows setting specific configurations for this node. | *Config | false |
| persistence | Persistence configures pvc for persisting data for this node. | *Persistence | false |
| resources | Compute Resources required by the app container. | corev1.ResourceRequirements | false |
| nodeSelector | NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. | map[string]string | false |
| affinity | If specified, the pod's scheduling constraints | *corev1.Affinity | false |
| tmKMS | TmKMS configuration for signing commits for this validator. When configured, .spec.validator.privateKeySecret will not be mounted on the validator node. | *TmKMS | false |
| stateSyncRestore | StateSyncRestore configures this node to find a state-sync snapshot on the network and restore from it. This is disabled by default. | *bool | false |

[Back to Custom Resources](#custom-resources)
