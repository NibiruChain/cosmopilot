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
	genesisLocation    = "data/genesis.json"
	MnemonicKey        = "mnemonic"
	tarballFinished    = "finished"
	upgradesConfigFile = "upgrades.json"

	LabelNodeID    = "node-id"
	LabelChainID   = "chain-id"
	LabelValidator = "validator"
	LabelChainNode = "chain-node"

	StringValueTrue  = "true"
	StringValueFalse = "false"

	AnnotationStateSyncTrustHeight    = "apps.k8s.nibiru.org/state-sync-trust-height"
	AnnotationStateSyncTrustHash      = "apps.k8s.nibiru.org/state-sync-trust-hash"
	annotationDataHeight              = "apps.k8s.nibiru.org/data-height"
	annotationSafeEvict               = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	annotationConfigHash              = "apps.k8s.nibiru.org/config-hash"
	annotationDataInitialized         = "apps.k8s.nibiru.org/data-initialized"
	annotationGenesisDownloaded       = "apps.k8s.nibiru.org/genesis-downloaded"
	annotationVaultKeyUploaded        = "apps.k8s.nibiru.org/vault-key-uploaded"
	annotationPvcSnapshotInProgress   = "apps.k8s.nibiru.org/snapshotting-pvc"
	annotationLastPvcSnapshot         = "apps.k8s.nibiru.org/last-pvc-snapshot"
	annotationSnapshotRetention       = "apps.k8s.nibiru.org/snapshot-retention"
	annotationPvcSnapshotReady        = "apps.k8s.nibiru.org/snapshot-ready"
	annotationExportingTarball        = "apps.k8s.nibiru.org/exporting-tarball"
	annotationSnapshotIntegrityStatus = "apps.k8s.nibiru.org/snapshot-integrity-status"
	annotationPodSpecHash             = "apps.k8s.nibiru.org/pod-spec-hash"

	timeoutPodRunning              = 5 * time.Minute
	timeoutPodDeleted              = 2 * time.Minute
	timeoutWaitServiceIP           = 5 * time.Minute
	minimumTimeBeforeFirstSnapshot = 10 * time.Minute
	livenessProbeTimeoutSeconds    = 5
	readinessProbeTimeoutSeconds   = 5

	prometheusScrapeInterval = "15s"

	nodeUtilsContainerName = "node-utils"
	nodeUtilsPortName      = "node-utils"
	nodeUtilsPort          = 8000

	nonRootId = 1000

	defaultAddrBookFile         = "/home/app/data/addrbook.json"
	defaultStateSyncTrustPeriod = "168h0m0s"
	defaultLogsLineCount        = 50

	snapshotCheckPeriod   = 15 * time.Second
	pvcDeletionWaitPeriod = 15 * time.Second

	cosmoGuardContainerName = "cosmoguard"
	cosmoGuardVolumeName    = "cosmoguard-config"

	initContainerCPU    = "100m"
	initContainerMemory = "250Mi"

	volumeSnapshot = "volume-snapshot"

	VolumeSnapshotDataSourceKind     = "VolumeSnapshot"
	VolumeSnapshotDataSourceApiGroup = "snapshot.storage.k8s.io"

	ReasonImagePullBackOff = "ImagePullBackOff"
	ReasonErrImagePull     = "ErrImagePull"
)

var (
	initContainerCpuResources    = resource.MustParse(initContainerCPU)
	initContainerMemoryResources = resource.MustParse(initContainerMemory)
)
