package controllers

const (
	CosmoGuardRpcPort         = 16657
	CosmoGuardLcdPort         = 11317
	CosmoGuardGrpcPort        = 19090
	CosmoGuardMetricsPortName = "fw-metrics"
	CosmoGuardMetricsPort     = 9001
	CosmoGuardEvmRpcPort      = 18545
	CosmoGuardEvmRpcWsPort    = 18546

	CosmoseedName = "cosmoseed"

	AnnotationStateSyncTrustHeight    = "apps.k8s.nibiru.org/state-sync-trust-height"
	AnnotationStateSyncTrustHash      = "apps.k8s.nibiru.org/state-sync-trust-hash"
	AnnotationDataHeight              = "apps.k8s.nibiru.org/data-height"
	AnnotationSafeEvict               = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	AnnotationConfigHash              = "apps.k8s.nibiru.org/config-hash"
	AnnotationDataInitialized         = "apps.k8s.nibiru.org/data-initialized"
	AnnotationGenesisDownloaded       = "apps.k8s.nibiru.org/genesis-downloaded"
	AnnotationVaultKeyUploaded        = "apps.k8s.nibiru.org/vault-key-uploaded"
	AnnotationPvcSnapshotInProgress   = "apps.k8s.nibiru.org/snapshotting-pvc"
	AnnotationLastPvcSnapshot         = "apps.k8s.nibiru.org/last-pvc-snapshot"
	AnnotationSnapshotRetention       = "apps.k8s.nibiru.org/snapshot-retention"
	AnnotationPvcSnapshotReady        = "apps.k8s.nibiru.org/snapshot-ready"
	AnnotationExportingTarball        = "apps.k8s.nibiru.org/exporting-tarball"
	AnnotationSnapshotIntegrityStatus = "apps.k8s.nibiru.org/snapshot-integrity-status"
	AnnotationPodSpecHash             = "apps.k8s.nibiru.org/pod-spec-hash"
	AnnotationVPAResources            = "apps.k8s.nibiru.org/vpa-resources"
	AnnotationVPALastCPUScale         = "apps.k8s.nibiru.org/last-cpu-scale"
	AnnotationVPALastMemoryScale      = "apps.k8s.nibiru.org/last-memory-scale"
	AnnotationStatefulSetPodName      = "statefulset.kubernetes.io/pod-name"

	LabelNodeID                = "node-id"
	LabelChainID               = "chain-id"
	LabelValidator             = "validator"
	LabelChainNode             = "chain-node"
	LabelChainNodeSet          = "nodeset"
	LabelChainNodeSetGroup     = "group"
	LabelChainNodeSetValidator = "validator"
	LabelGlobalIngress         = "global-ingress"
	LabelScope                 = "scope"
	LabelApp                   = "app"
	LabelSeed                  = "seed"
	LabelPeer                  = "peer"

	StringValueTrue  = "true"
	StringValueFalse = "false"

	EvmRpcPortName   = "evm-rpc"
	EvmRpcPort       = 8545
	EvmRpcWsPortName = "evm-rpc-ws"
	EvmRpcWsPort     = 8546

	AppTomlFile         = "app.toml"
	MinimumGasPricesKey = "minimum-gas-prices"
)
