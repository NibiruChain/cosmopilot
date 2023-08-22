
### Custom Resources


### Sub Resources

* [AccountAssets](#accountassets)
* [AppSpec](#appspec)
* [Config](#config)
* [ExposeConfig](#exposeconfig)
* [FromNodeRPCConfig](#fromnoderpcconfig)
* [GenesisConfig](#genesisconfig)
* [GenesisInitConfig](#genesisinitconfig)
* [InitCommand](#initcommand)
* [Peer](#peer)
* [ServiceMonitorSpec](#servicemonitorspec)
* [SidecarSpec](#sidecarspec)
* [StateSyncConfig](#statesyncconfig)
* [TmKMS](#tmkms)
* [TmKmsKeyFormat](#tmkmskeyformat)
* [TmKmsProvider](#tmkmsprovider)
* [TmKmsVaultProvider](#tmkmsvaultprovider)
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
| sdkVersion | SdkVersion specifies the version of cosmos-sdk used by this app. Defaults to `v0.47`. | *SdkVersion | false |

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
| stateSync | StateSync configures statesync snapshots for this node. | *[StateSyncConfig](#statesyncconfig) | false |
| seedMode | SeedMode configures this node to run on seed mode. Defaults to `false`. | *bool | false |
| env | Env refers to the list of environment variables to set in the app container. | []corev1.EnvVar | false |
| safeToEvict | SafeToEvict sets cluster-autoscaler.kubernetes.io/safe-to-evict annotation to the given value. It allows/disallows cluster-autoscaler to evict this node's pod. | *bool | false |
| serviceMonitor | ServiceMonitor allows deploying prometheus service monitor for this node. | *[ServiceMonitorSpec](#servicemonitorspec) | false |

[Back to Custom Resources](#custom-resources)

#### ExposeConfig



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| p2p | P2P indicates whether to expose p2p endpoint for this node. Defaults to `false`. | *bool | false |
| p2pServiceType | P2pServiceType indicates how p2p port will be exposed. Either `LoadBalancer` or `NodePort`. Defaults to `NodePort`. | *corev1.ServiceType | false |

[Back to Custom Resources](#custom-resources)

#### FromNodeRPCConfig



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| secure | Defines protocol to use. Defaults to false. | bool | false |
| hostname | Hostname or IP address of the RPC server | string | true |
| port | TCP port used for RPC queries on the RPC server. Defaults to `26657`. | *int | false |

[Back to Custom Resources](#custom-resources)

#### GenesisConfig

GenesisConfig specifies how genesis will be retrieved

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| url | URL to download the genesis from. | *string | false |
| fromNodeRPC | Get the genesis from the existing node RPC endpoint. | *[FromNodeRPCConfig](#fromnoderpcconfig) | false |
| genesisSHA | GenesisSHA is the 256 SHA to validate the genesis. | *string | false |
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

#### ServiceMonitorSpec



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| enable | Enable indicates a service monitor should be deployed for this node. | bool | true |
| selector | Selector indicates the prometheus installation that will be using this service monitor. | map[string]string | false |

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
| securityContext | SecurityContext defines the security options the container should be run with. If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext, which defaults to user ID 1000. | *corev1.SecurityContext | false |
| resources | Compute Resources required by the sidecar container. | corev1.ResourceRequirements | false |

[Back to Custom Resources](#custom-resources)

#### StateSyncConfig



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| snapshotInterval | SnapshotInterval specifies the block interval at which local state sync snapshots are taken (0 to disable). | int | true |
| snapshotKeepRecent | SnapshotKeepRecent specifies the number of recent snapshots to keep and serve (0 to keep all). Defaults to 2. | *int | false |

[Back to Custom Resources](#custom-resources)

#### TmKMS



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| provider | Provider specifies the signing provider to be used by tmkms | [TmKmsProvider](#tmkmsprovider) | true |
| keyFormat | KeyFormat specifies the format and type of key for chain. Defaults to `{\"type\": \"bech32\", \"account_key_prefix\": \"nibipub\", \"consensus_key_prefix\": \"nibivalconspub\"}`. | *[TmKmsKeyFormat](#tmkmskeyformat) | false |
| validatorProtocol | ValidatorProtocol specifies the tendermint protocol version to be used. One of `legacy`, `v0.33` or `v0.34`. Defaults to `v0.34`. | *tmkms.ProtocolVersion | false |

[Back to Custom Resources](#custom-resources)

#### TmKmsKeyFormat



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type |  | string | true |
| account_key_prefix |  | string | true |
| consensus_key_prefix |  | string | true |

[Back to Custom Resources](#custom-resources)

#### TmKmsProvider



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| vault | Vault provider | *[TmKmsVaultProvider](#tmkmsvaultprovider) | false |

[Back to Custom Resources](#custom-resources)

#### TmKmsVaultProvider



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| address | Address of the Vault cluster | string | true |
| key | Key to be used by this validator. | string | true |
| certificateSecret | Secret containing the CA certificate of the Vault cluster. | *corev1.SecretKeySelector | false |
| tokenSecret | Secret containing the token to be used. | *corev1.SecretKeySelector | true |
| uploadGenerated | UploadGenerated indicates if the controller should upload the generated private key to vault. Defaults to `false`. Will be set to `true` if this validator is initializing a new genesis. This should not be used in production. | bool | false |

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
