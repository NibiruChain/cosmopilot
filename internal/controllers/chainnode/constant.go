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

	timeoutPodRunning              = 5 * time.Minute
	timeoutPodDeleted              = 2 * time.Minute
	timeoutWaitServiceIP           = 5 * time.Minute
	minimumTimeBeforeFirstSnapshot = 10 * time.Minute
	livenessProbeTimeoutSeconds    = 5
	readinessProbeTimeoutSeconds   = 5

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
