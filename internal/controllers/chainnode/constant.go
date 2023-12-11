package chainnode

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	nodeKeyFilename    = "node_key.json"
	PrivKeyFilename    = "priv_validator_key.json"
	appTomlFilename    = "app.toml"
	configTomlFilename = "config.toml"
	GenesisFilename    = "genesis.json"
	genesisLocation    = "data/genesis.json"
	MnemonicKey        = "mnemonic"
	tarballFinished    = "finished"
	upgradesConfigFile = "upgrades.json"

	LabelNodeID    = "node-id"
	LabelChainID   = "chain-id"
	LabelValidator = "validator"
	LabelChainNode = "chain-node"

	AnnotationStateSyncTrustHeight = "apps.k8s.nibiru.org/state-sync-trust-height"
	AnnotationStateSyncTrustHash   = "apps.k8s.nibiru.org/state-sync-trust-hash"

	annotationSafeEvict             = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	annotationConfigHash            = "apps.k8s.nibiru.org/config-hash"
	annotationDataInitialized       = "apps.k8s.nibiru.org/data-initialized"
	annotationVaultKeyUploaded      = "apps.k8s.nibiru.org/vault-key-uploaded"
	annotationPvcSnapshotInProgress = "apps.k8s.nibiru.org/snapshotting-pvc"
	annotationLastPvcSnapshot       = "apps.k8s.nibiru.org/last-pvc-snapshot"
	annotationSnapshotRetention     = "apps.k8s.nibiru.org/snapshot-retention"
	annotationPvcSnapshotReady      = "apps.k8s.nibiru.org/snapshot-ready"
	annotationExportingTarball      = "apps.k8s.nibiru.org/exporting-tarball"

	timeoutPodRunning              = 5 * time.Minute
	timeoutPodDeleted              = 2 * time.Minute
	startupTimeout                 = 1 * time.Hour
	timeoutWaitServiceIP           = 5 * time.Minute
	minimumTimeBeforeFirstSnapshot = 10 * time.Minute

	prometheusScrapeInterval = "15s"

	nodeUtilsContainerName = "node-utils"
	nodeUtilsCPU           = "100m"
	nodeUtilsMemory        = "100Mi"
	nodeUtilsPortName      = "node-utils"
	nodeUtilsPort          = 8000

	nonRootId = 1000

	privValidatorListenAddress = "tcp://0.0.0.0:26659"

	defaultStateSyncTrustPeriod = "168h0m0s"
	defaultLogsLineCount        = 50

	snapshotCheckPeriod = 15 * time.Second

	firewallContainerName   = "firewall"
	firewallVolumeName      = "firewall-config"
	firewallRpcPort         = 16657
	firewallLcdPort         = 11317
	firewallGrpcPort        = 19090
	firewallMetricsPortName = "metrics"
	firewallMetricsPort     = 9001
	firewallCpu             = "200m"
	firewallMemory          = "250Mi"
)

var (
	nodeUtilsCpuResources    = resource.MustParse(nodeUtilsCPU)
	nodeUtilsMemoryResources = resource.MustParse(nodeUtilsMemory)

	firewallCpuResources    = resource.MustParse(firewallCpu)
	firewallMemoryResources = resource.MustParse(firewallMemory)
)
