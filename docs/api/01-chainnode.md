
### Custom Resources

* [ChainNode](#chainnode)

### Sub Resources

* [ChainNodeList](#chainnodelist)
* [ChainNodeSpec](#chainnodespec)
* [ChainNodeStatus](#chainnodestatus)
* [ValidatorConfig](#validatorconfig)
* [AccountAssets](#accountassets)
* [AppSpec](#appspec)
* [Config](#config)
* [CreateValidatorConfig](#createvalidatorconfig)
* [ExportTarballConfig](#exporttarballconfig)
* [ExposeConfig](#exposeconfig)
* [FirewallConfig](#firewallconfig)
* [FromNodeRPCConfig](#fromnoderpcconfig)
* [GcsExportConfig](#gcsexportconfig)
* [GenesisConfig](#genesisconfig)
* [GenesisInitConfig](#genesisinitconfig)
* [InitCommand](#initcommand)
* [Peer](#peer)
* [Persistence](#persistence)
* [PvcSnapshot](#pvcsnapshot)
* [ServiceMonitorSpec](#servicemonitorspec)
* [SidecarSpec](#sidecarspec)
* [StateSyncConfig](#statesyncconfig)
* [TmKMS](#tmkms)
* [TmKmsKeyFormat](#tmkmskeyformat)
* [TmKmsProvider](#tmkmsprovider)
* [TmKmsVaultProvider](#tmkmsvaultprovider)
* [Upgrade](#upgrade)
* [UpgradeSpec](#upgradespec)
* [ValidatorInfo](#validatorinfo)
* [VolumeSnapshotsConfig](#volumesnapshotsconfig)

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
| genesis | Indicates where this node will get the genesis from. Can be omitted when .spec.validator.init is specified. | *[GenesisConfig](#genesisconfig) | true |
| app | Specifies image, version and binary name of the chain application to run. It also allows to schedule upgrades, or setting/updating the image for an on-chain upgrade. | [AppSpec](#appspec) | true |
| config | Allows setting specific configurations for this node. | *[Config](#config) | false |
| persistence | Configures PVC for persisting data. Automated data snapshots can also be configured in this section. | *[Persistence](#persistence) | false |
| validator | Indicates this node is going to be a validator and allows configuring it. | *[ValidatorConfig](#validatorconfig) | false |
| autoDiscoverPeers | Ensures peers with same chain ID are connected with each other. Enabled by default. | *bool | false |
| stateSyncRestore | Configures this node to find a state-sync snapshot on the network and restore from it. This is disabled by default. | *bool | false |
| peers | Additional persistent peers that should be added to this node. | [][Peer](#peer) | false |
| expose | Allows exposing P2P traffic to public. | *[ExposeConfig](#exposeconfig) | false |
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
| upgrades | All scheduled/completed upgrades performed by the operator on this ChainNode. | [][Upgrade](#upgrade) | false |
| pubKey | Public key of the validator. | string | false |
| validatorStatus | Indicates the current status of validator if this node is one. | ValidatorStatus | false |

[Back to Custom Resources](#custom-resources)

#### ValidatorConfig

ValidatorConfig contains the configuration for running a node as validator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| privateKeySecret | Indicates the secret containing the private key to be used by this validator. Defaults to `<chainnode>-priv-key`. Will be created if it does not exist. | *string | false |
| info | Contains information details about this validator. | *[ValidatorInfo](#validatorinfo) | false |
| init | Specifies configs and initialization commands for creating a new genesis. | *[GenesisInitConfig](#genesisinitconfig) | false |
| tmKMS | TmKMS configuration for signing commits for this validator. When configured, .spec.validator.privateKeySecret will not be mounted on the validator node. | *[TmKMS](#tmkms) | false |
| createValidator | Indicates that operator should run create-validator tx to make this node a validator. | *[CreateValidatorConfig](#createvalidatorconfig) | false |

[Back to Custom Resources](#custom-resources)

#### AccountAssets

AccountAssets represents the assets associated with an account.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| address | Address of the account. | string | true |
| assets | Assets assigned to this account. | []string | true |

[Back to Custom Resources](#custom-resources)

#### AppSpec

AppSpec specifies the source image, version and binary name of the app to run. Also allows specifying upgrades for the app and enabling automatic check of upgrade proposals on chain.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| image | Container image to be used. | string | true |
| version | Image tag to be used. Once there are completed or skipped upgrades this will be ignored. For a new node that will be state-synced, this will be the version used during state-sync. Only after that, the operator will switch to the version of last upgrade. Defaults to `latest`. | *string | false |
| imagePullPolicy | Indicates the desired pull policy when creating nodes. Defaults to `Always` if `version` is `latest` and `IfNotPresent` otherwise. | corev1.PullPolicy | false |
| app | Binary name of the application to be run. | string | true |
| sdkVersion | SdkVersion specifies the version of cosmos-sdk used by this app. Valid options are: - \"v0.47\" (default) - \"v0.45\" | *SdkVersion | false |
| checkGovUpgrades | Whether the operator should query gov proposals to find and schedule upgrades. Defaults to `true`. | *bool | false |
| upgrades | List of upgrades to schedule for this node. | [][UpgradeSpec](#upgradespec) | false |

[Back to Custom Resources](#custom-resources)

#### Config

Config allows setting specific configurations for a node, including overriding configs in app.toml and config.toml.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| override | Allows overriding configs on `.toml` configuration files. | *map[string]runtime.RawExtension | false |
| sidecars | Allows configuring additional containers to run alongside the node. | [][SidecarSpec](#sidecarspec) | false |
| imagePullSecrets | Optional list of references to secrets in the same namespace to use for pulling any of the images used by this node. | []corev1.LocalObjectReference | false |
| blockThreshold | The time to wait for a block before considering node unhealthy. Defaults to `15s`. | *string | false |
| reconcilePeriod | Period at which a reconcile loop will happen for this ChainNode. Defaults to `15s`. | *string | false |
| stateSync | Allows configuring this node to perform state-sync snapshots. | *[StateSyncConfig](#statesyncconfig) | false |
| seedMode | Configures this node to run on seed mode. Defaults to `false`. | *bool | false |
| env | List of environment variables to set in the app container. | []corev1.EnvVar | false |
| safeToEvict | SafeToEvict sets cluster-autoscaler.kubernetes.io/safe-to-evict annotation to the given value. It allows/disallows cluster-autoscaler to evict this node's pod. | *bool | false |
| serviceMonitor | ServiceMonitor allows deploying prometheus service monitor for this node. | *[ServiceMonitorSpec](#servicemonitorspec) | false |
| firewall | Deploys cosmos-firewall to protect API endpoints to the node. | *[FirewallConfig](#firewallconfig) | false |

[Back to Custom Resources](#custom-resources)

#### CreateValidatorConfig

CreateValidatorConfig holds configuration for the operator to submit a create-validator transaction.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| accountMnemonicSecret | Name of the secret containing the mnemonic of the account to be used by this validator. Defaults to `<chainnode>-account`. Will be created if it does not exist. | *string | false |
| accountHDPath | HD path of accounts. Defaults to `m/44'/118'/0'/0/0`. | *string | false |
| accountPrefix | Prefix for accounts. Defaults to `nibi`. | *string | false |
| valPrefix | Prefix for validator operator accounts. Defaults to `nibivaloper`. | *string | false |
| commissionMaxChangeRate | Maximum commission change rate percentage (per day). Defaults to `0.1`. | *string | false |
| commissionMaxRate | Maximum commission rate percentage. Defaults to `0.1`. | *string | false |
| commissionRate | Initial commission rate percentage. Defaults to `0.1`. | *string | false |
| minSelfDelegation | Minimum self delegation required on the validator. Defaults to `1`. | *string | false |
| stakeAmount | Amount to be staked by this validator. | string | true |
| gasPrices | Gas prices in decimal format to determine the transaction fee. | string | true |

[Back to Custom Resources](#custom-resources)

#### ExportTarballConfig

ExportTarballConfig holds config options for tarball upload.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| suffix | Suffix to add to archive name. The name of the tarball will be `<chain-id>-<timestamp>-<suffix>`. | *string | false |
| deleteOnExpire | Whether to delete the tarball when the snapshot expires. Default is `false`. | *bool | false |
| gcs | Configuration to upload tarballs to a GCS bucket. | *[GcsExportConfig](#gcsexportconfig) | false |

[Back to Custom Resources](#custom-resources)

#### ExposeConfig

ExposeConfig allows configuring how P2P endpoint is exposed to public.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| p2p | Whether to expose p2p endpoint for this node. Defaults to `false`. | *bool | false |
| p2pServiceType | P2pServiceType indicates how P2P port will be exposed. Valid values are: - `LoadBalancer` - `NodePort` (default) | *corev1.ServiceType | false |

[Back to Custom Resources](#custom-resources)

#### FirewallConfig

FirewallConfig allows configuring cosmos-firewall rules.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| enable | Whether to enable cosmos-firewall on this node. | bool | true |
| config | ConfigMap which cosmos-firewall configuration for this node. | *corev1.ConfigMapKeySelector | true |

[Back to Custom Resources](#custom-resources)

#### FromNodeRPCConfig

FromNodeRPCConfig holds configuration to retrieve genesis from an existing node using RPC endpoint.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| secure | Defines protocol to use. Defaults to `false`. | bool | false |
| hostname | Hostname or IP address of the RPC server | string | true |
| port | TCP port used for RPC queries on the RPC server. Defaults to `26657`. | *int | false |

[Back to Custom Resources](#custom-resources)

#### GcsExportConfig

GcsExportConfig holds required settings to upload to GCS.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| bucket | Name of the bucket to upload tarballs to. | string | true |
| credentialsSecret | Secret with the JSON credentials to upload to bucket. | *corev1.SecretKeySelector | true |

[Back to Custom Resources](#custom-resources)

#### GenesisConfig

GenesisConfig specifies how genesis will be retrieved.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| url | URL to download the genesis from. | *string | false |
| fromNodeRPC | Get the genesis from an existing node using its RPC endpoint. | *[FromNodeRPCConfig](#fromnoderpcconfig) | false |
| genesisSHA | SHA256 to validate the genesis. | *string | false |
| configMap | ConfigMap specifies a configmap to load the genesis from. | *string | false |
| useDataVolume | UseDataVolume indicates that the operator should save the genesis in the same volume as node data instead of a ConfigMap. This is useful for genesis whose size is bigger than ConfigMap limit of 1MiB. | *bool | false |

[Back to Custom Resources](#custom-resources)

#### GenesisInitConfig

GenesisInitConfig specifies configs and initialization commands for creating a new genesis.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| chainID | ChainID of the chain to initialize. | string | true |
| accountMnemonicSecret | Name of the secret containing the mnemonic of the account to be used by this validator. Defaults to `<chainnode>-account`. Will be created if it does not exist. | *string | false |
| accountHDPath | HD path of accounts. Defaults to `m/44'/118'/0'/0/0`. | *string | false |
| accountPrefix | Prefix for accounts. Defaults to `nibi`. | *string | false |
| valPrefix | Prefix for validator operator accounts. Defaults to `nibivaloper`. | *string | false |
| commissionMaxChangeRate | Maximum commission change rate percentage (per day). Defaults to `0.1`. | *string | false |
| commissionMaxRate | Maximum commission rate percentage. Defaults to `0.1`. | *string | false |
| commissionRate | Initial commission rate percentage. Defaults to `0.1`. | *string | false |
| minSelfDelegation | Minimum self delegation required on the validator. Defaults to `1`. | *string | false |
| assets | Assets is the list of tokens and their amounts to be assigned to this validators account. | []string | true |
| stakeAmount | Amount to be staked by this validator. | string | true |
| accounts | Accounts specify additional accounts and respective assets to be added to this chain. | [][AccountAssets](#accountassets) | false |
| unbondingTime | Time required to totally unbond delegations. Defaults to `1814400s` (21 days). | *string | false |
| votingPeriod | Voting period for this chain. Defaults to `120h`. | *string | false |
| additionalInitCommands | Additional commands to run on genesis initialization. Note: App home is at `/home/app` and `/temp` is a temporary volume shared by all init containers. | [][InitCommand](#initcommand) | false |

[Back to Custom Resources](#custom-resources)

#### InitCommand

InitCommand represents an initialization command. It may be used for running addtional operators on genesis or volume initialization.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| image | Image to be used to run this command. Defaults to app image. | *string | false |
| command | Command to be used. Defaults to image entrypoint. | []string | false |
| args | Args to be passed to this command. | []string | true |

[Back to Custom Resources](#custom-resources)

#### Peer

Peer represents a persistent peer.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| id | Tendermint node ID for this node. | string | true |
| address | Hostname or IP address of this peer. | string | true |
| port | P2P port to be used. Defaults to `26656`. | *int | false |
| unconditional | Indicates this peer is unconditional. | *bool | false |
| private | Indicates this peer is private. | *bool | false |

[Back to Custom Resources](#custom-resources)

#### Persistence

Persistence configuration for a node.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| size | Size of the persistent volume for storing data. Can't be updated when autoResize is enabled. Defaults to `50Gi`. | *string | false |
| storageClass | Name of the storage class to use for the PVC. Uses the default class if not specified. to create persistent volumes. | *string | false |
| autoResize | Automatically resize PVC. Defaults to `true`. | *bool | false |
| autoResizeThreshold | Percentage of data usage at which an auto-resize event should occur. Defaults to `80`. | *int | false |
| autoResizeIncrement | Increment size on each auto-resize event. Defaults to `50Gi`. | *string | false |
| autoResizeMaxSize | Size at which auto-resize will stop incrementing PVC size. Defaults to `2Ti`. | *string | false |
| additionalInitCommands | Additional commands to run on data initialization. Useful for downloading and extracting snapshots. App home is at `/home/app` and data dir is at `/home/app/data`. There is also `/temp`, a temporary volume shared by all init containers. | [][InitCommand](#initcommand) | false |
| snapshots | Whether the operator should create volume snapshots according to this config. | *[VolumeSnapshotsConfig](#volumesnapshotsconfig) | false |
| restoreFromSnapshot | Restore from the specified snapshot when creating the PVC for this node. | *[PvcSnapshot](#pvcsnapshot) | false |

[Back to Custom Resources](#custom-resources)

#### PvcSnapshot

PvcSnapshot represents a snapshot to be used to restore a PVC.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name of resource being referenced. | string | true |
| kind | Type of resource being referenced. Defaults to `VolumeSnapshot`. | *string | false |
| apiGroup | Group for the resource being referenced. Defaults to `snapshot.storage.k8s.io`. | *string | false |

[Back to Custom Resources](#custom-resources)

#### ServiceMonitorSpec

ServiceMonitorSpec allows enabling/disabling deployment of ServiceMonitor for this node.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| enable | Whether a service monitor should be deployed for this node. | bool | true |
| selector | Indicates the prometheus installation that will be using this service monitor. | map[string]string | false |

[Back to Custom Resources](#custom-resources)

#### SidecarSpec

SidecarSpec allows configuring additional containers to run alongside the node.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| name | Name to be assigned to the container. | string | true |
| image | Container image to be used. | string | true |
| imagePullPolicy | Indicates the desired pull policy when creating nodes. Defaults to `Always` if `version` is `latest` and `IfNotPresent` otherwise. | corev1.PullPolicy | false |
| mountDataVolume | Where data volume will be mounted on this container. It is not mounted if not specified. | *string | false |
| command | Command to be run by this container. Defaults to entrypoint defined in image. | []string | false |
| args | Args to be passed to this container. Defaults to cmd defined in image. | []string | false |
| env | Environment variables to be passed to this container. | []corev1.EnvVar | false |
| securityContext | Security options the container should be run with. | *corev1.SecurityContext | false |
| resources | Compute Resources for the sidecar container. | corev1.ResourceRequirements | false |

[Back to Custom Resources](#custom-resources)

#### StateSyncConfig

StateSyncConfig holds configurations for enabling state-sync snapshots on a node.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| snapshotInterval | Block interval at which local state sync snapshots are taken (0 to disable). | int | true |
| snapshotKeepRecent | Number of recent snapshots to keep and serve (0 to keep all). Defaults to 2. | *int | false |

[Back to Custom Resources](#custom-resources)

#### TmKMS

TmKMS allows configuring tmkms for signing for this validator node instead of using plaintext private key file.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| provider | Signing provider to be used by tmkms. Currently only `vault` is supported. | [TmKmsProvider](#tmkmsprovider) | true |
| keyFormat | Format and type of key for chain. Defaults to `{\"type\": \"bech32\", \"account_key_prefix\": \"nibipub\", \"consensus_key_prefix\": \"nibivalconspub\"}`. | *[TmKmsKeyFormat](#tmkmskeyformat) | false |
| validatorProtocol | Tendermint's protocol version to be used. Valid options are: - `v0.34` (default) - `v0.33` - `legacy` | *tmkms.ProtocolVersion | false |

[Back to Custom Resources](#custom-resources)

#### TmKmsKeyFormat

TmKmsKeyFormat represents key format for tmKMS.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type | Key type | string | true |
| account_key_prefix | Account keys prefixes | string | true |
| consensus_key_prefix | Consensus keys prefix | string | true |

[Back to Custom Resources](#custom-resources)

#### TmKmsProvider

TmKmsProvider allows configuring providers for tmKMS. Note that only one should be configured.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| vault | Vault provider. | *[TmKmsVaultProvider](#tmkmsvaultprovider) | false |

[Back to Custom Resources](#custom-resources)

#### TmKmsVaultProvider

TmKmsVaultProvider holds `vault` provider specific configurations.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| address | Full address of the Vault cluster. | string | true |
| key | Key to be used by this validator. | string | true |
| certificateSecret | Secret containing the CA certificate of the Vault cluster. | *corev1.SecretKeySelector | false |
| tokenSecret | Secret containing the token to be used. | *corev1.SecretKeySelector | true |
| uploadGenerated | UploadGenerated indicates if the controller should upload the generated private key to vault. Defaults to `false`. Will be set to `true` if this validator is initializing a new genesis. This should not be used in production. | bool | false |
| autoRenewToken | Whether to automatically renew vault token. Defaults to `false`. | bool | false |

[Back to Custom Resources](#custom-resources)

#### Upgrade

Upgrade represents an upgrade processed by the operator and added to status.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| height | Height at which the upgrade should occur. | int64 | true |
| image | Container image replacement to be used in the upgrade. | string | true |
| status | Upgrade status. | UpgradePhase | true |
| source | Where the operator got this upgrade from. | UpgradeSource | true |

[Back to Custom Resources](#custom-resources)

#### UpgradeSpec

UpgradeSpec represents a manual upgrade.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| height | Height at which the upgrade should occur. | int64 | true |
| image | Container image replacement to be used in the upgrade. | string | true |

[Back to Custom Resources](#custom-resources)

#### ValidatorInfo

ValidatorInfo contains information about this validator.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| moniker | Moniker to be used by this validator. Defaults to the ChainNode name. | *string | false |
| details | Details of this validator. | *string | false |
| website | Website of the validator. | *string | false |
| identity | Identity signature of this validator. | *string | false |

[Back to Custom Resources](#custom-resources)

#### VolumeSnapshotsConfig

VolumeSnapshotsConfig holds the configuration of snapshotting feature.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| frequency | How often a snapshot should be created. | string | true |
| retention | How long a snapshot should be retained. Default is indefinite retention. | *string | false |
| snapshotClass | Name of the volume snapshot class to be used. Uses the default class if not specified. | *string | false |
| stopNode | Whether the node should be stopped while the snapshot is taken. Defaults to `false`. | *bool | false |
| exportTarball | Whether to create a tarball of data directory in each snapshot and upload it to external storage. | *[ExportTarballConfig](#exporttarballconfig) | false |

[Back to Custom Resources](#custom-resources)
