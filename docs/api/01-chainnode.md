
### Custom Resources

* [ChainNode](#chainnode)

### Sub Resources

* [AccountAssets](#accountassets)
* [AppSpec](#appspec)
* [ChainNodeList](#chainnodelist)
* [ChainNodeSpec](#chainnodespec)
* [ChainNodeStatus](#chainnodestatus)
* [Config](#config)
* [GenesisConfig](#genesisconfig)
* [GenesisInitConfig](#genesisinitconfig)
* [InitCommand](#initcommand)
* [Peer](#peer)
* [Persistence](#persistence)
* [SidecarSpec](#sidecarspec)
* [ValidatorConfig](#validatorconfig)
* [ValidatorInfo](#validatorinfo)

#### AccountAssets



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| address | Address of the account. | string | true |
| assets | Assets to be assigned to this account. | []string | true |

[Back to Custom Resources](#custom-resources)

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
| genesis | Genesis indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified. | *[GenesisConfig](#genesisconfig) | true |
| app | App specifies image and binary name of the chain application to run | [AppSpec](#appspec) | true |
| config | Config allows setting specific configurations for this node | *[Config](#config) | false |
| persistence | Persistence configures pvc for persisting data on nodes | *[Persistence](#persistence) | false |
| validator | Validator configures this node as a validator and configures it. | *[ValidatorConfig](#validatorconfig) | false |
| autoDiscoverPeers | AutoDiscoverPeers ensures peers with same chain ID are connected with each other. By default, it is enabled. | *bool | false |
| peers | Peers are additional persistent peers that should be added to this node. | [][Peer](#peer) | false |

[Back to Custom Resources](#custom-resources)

#### ChainNodeStatus

ChainNodeStatus defines the observed state of ChainNode

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| nodeID | NodeID show this node's ID | string | false |
| chainID | ChainID shows the chain ID | string | false |
| pvcSize | PvcSize shows the current size of the pvc of this node | string | false |
| validator | Validator indicates if this node is a validator. | bool | true |
| accountAddress | AccountAddress is the account address of this validator. Omitted when not a validator | string | false |
| validatorAddress | ValidatorAddress is the valoper address of this validator. Omitted when not a validator | string | false |

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
| configMap | ConfigMap specifies a configmap to load the genesis from | *string | false |

[Back to Custom Resources](#custom-resources)

#### GenesisInitConfig

GenesisInitConfig specifies configs and initialization commands for creating a new chain and its genesis

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| chainID | ChainID of the chain to initialize. | string | true |
| accountMnemonicSecret | AccountMnemonicSecret is the name of the secret containing the mnemonic of the account to be used by this validator. Defaults to `<chainnode>-account`. Will be created if does not exist. | *string | false |
| accountHDPath | AccountHDPath is the HD path for the validator account. Defaults to `m/44'/118'/0'/0/0`. | *string | false |
| accountPrefix | AccountPrefix is the prefix for accounts. Defaults to `nibi`. | *string | false |
| valPrefix | ValPrefix is the prefix for validator accounts. Defaults to `nibivaloper`. | *string | false |
| assets | Assets is the list of tokens and their amounts to be assigned to this validators account. | []string | true |
| stakeAmount | StakeAmount represents the amount to be staked by this validator. | string | true |
| accounts | Accounts specify additional accounts and respective assets to be added to this chain. | [][AccountAssets](#accountassets) | false |
| unbondingTime | UnbondingTime is the time that takes to unbond delegations. Defaults to `1814400s`. | *string | false |
| votingPeriod | VotingPeriod indicates the voting period for this chain. Defaults to `120h`. | *string | false |
| additionalInitCommands | AdditionalInitCommands are additional commands to run on genesis initialization. App home is at `/home/app` and `/temp` is a temporary volume shared by all init containers. | [][InitCommand](#initcommand) | false |

[Back to Custom Resources](#custom-resources)

#### InitCommand



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| image | Image to be used to run this command. Defaults to app image. | *string | false |
| command | Command to be used. Defaults to image entrypoint. | []string | false |
| args | Args to be passed to this command. | []string | true |

[Back to Custom Resources](#custom-resources)

#### Peer



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| id | ID refers to tendermint node ID for this node | string | true |
| address | Address is the hostname or IP address of this peer | string | true |
| port | Port is the P2P port to be used. Defaults to `26656`. | *int | false |
| unconditional | Unconditional marks this peer as unconditional. | *bool | false |
| private | Private marks this peer as private. | *bool | false |

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

#### ValidatorConfig

ValidatorConfig turns this node into a validator and specifies how it will do it.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | PrivateKeySecret indicates the secret containing the private key to be use by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Info contains information details about this validator. | *[ValidatorInfo](#validatorinfo) | false |
| init | Init specifies configs and initialization commands for creating a new chain and its genesis. | *[GenesisInitConfig](#genesisinitconfig) | false |

[Back to Custom Resources](#custom-resources)

#### ValidatorInfo

ValidatorInfo contains information about this validator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| moniker | Moniker to be used by this validator. Defaults to the ChainNode name. | *string | false |
| details | Details of this validator. | *string | false |
| website | Website indicates this validator's website. | *string | false |
| identity | Identity signature of this validator. | *string | false |

[Back to Custom Resources](#custom-resources)
