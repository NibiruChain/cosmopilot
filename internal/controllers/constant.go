package controllers

const (
	CosmoGuardRpcPortName      = "fw-rpc"
	CosmoGuardRpcPort          = 16657
	CosmoGuardLcdPortName      = "fw-lcd"
	CosmoGuardLcdPort          = 11317
	CosmoGuardGrpcPortName     = "fw-grpc"
	CosmoGuardGrpcPort         = 19090
	CosmoGuardMetricsPortName  = "fw-metrics"
	CosmoGuardMetricsPort      = 9001
	CosmoGuardEvmRpcPortName   = "fw-evm-rpc"
	CosmoGuardEvmRpcPort       = 18545
	CosmoGuardEvmRpcWsPortName = "fw-evm-rpc-ws"
	CosmoGuardEvmRpcWsPort     = 18546

	CosmoseedName = "cosmoseed"

	AnnotationStateSyncTrustHeight    = "cosmopilot.voluzi.com/state-sync-trust-height"
	AnnotationStateSyncTrustHash      = "cosmopilot.voluzi.com/state-sync-trust-hash"
	AnnotationDataHeight              = "cosmopilot.voluzi.com/data-height"
	AnnotationSafeEvict               = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	AnnotationConfigHash              = "cosmopilot.voluzi.com/config-hash"
	AnnotationDataInitialized         = "cosmopilot.voluzi.com/data-initialized"
	AnnotationGenesisDownloaded       = "cosmopilot.voluzi.com/genesis-downloaded"
	AnnotationVaultKeyUploaded        = "cosmopilot.voluzi.com/vault-key-uploaded"
	AnnotationPvcSnapshotInProgress   = "cosmopilot.voluzi.com/snapshotting-pvc"
	AnnotationLastPvcSnapshot         = "cosmopilot.voluzi.com/last-pvc-snapshot"
	AnnotationSnapshotRetention       = "cosmopilot.voluzi.com/snapshot-retention"
	AnnotationPvcSnapshotReady        = "cosmopilot.voluzi.com/snapshot-ready"
	AnnotationExportingTarball        = "cosmopilot.voluzi.com/exporting-tarball"
	AnnotationSnapshotIntegrityStatus = "cosmopilot.voluzi.com/snapshot-integrity-status"
	AnnotationPodSpecHash             = "cosmopilot.voluzi.com/pod-spec-hash"
	AnnotationVPAResources            = "cosmopilot.voluzi.com/vpa-resources"
	AnnotationVPALastCPUScale         = "cosmopilot.voluzi.com/last-cpu-scale"
	AnnotationVPALastMemoryScale      = "cosmopilot.voluzi.com/last-memory-scale"
	AnnotationVPAOOMRecoveryHistory   = "cosmopilot.voluzi.com/oom-recovery-history"
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
	LabelUpgrading             = "upgrading"

	StringValueTrue  = "true"
	StringValueFalse = "false"

	EvmRpcPortName   = "evm-rpc"
	EvmRpcPort       = 8545
	EvmRpcWsPortName = "evm-rpc-ws"
	EvmRpcWsPort     = 8546

	AppTomlFile         = "app.toml"
	MinimumGasPricesKey = "minimum-gas-prices"
)
