package chainnode

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	nodeKeyFilename    = "node_key.json"
	privKeyFilename    = "priv_validator_key.json"
	appTomlFilename    = "app.toml"
	configTomlFilename = "config.toml"
	genesisFilename    = "genesis.json"
	mnemonicKey        = "mnemonic"

	LabelNodeID    = "node-id"
	LabelChainID   = "chain-id"
	LabelValidator = "validator"

	AnnotationStateSyncTrustHeight = "apps.k8s.nibiru.org/state-sync-trust-height"
	AnnotationStateSyncTrustHash   = "apps.k8s.nibiru.org/state-sync-trust-hash"

	annotationConfigHash       = "apps.k8s.nibiru.org/config-hash"
	annotationDataInitialized  = "apps.k8s.nibiru.org/data-initialized"
	annotationVaultKeyUploaded = "apps.k8s.nibiru.org/vault-key-uploaded"

	timeoutPodRunning = 5 * time.Minute
	timeoutPodDeleted = 30 * time.Second

	nodeUtilsContainerName = "node-utils"
	nodeUtilsCPU           = "100m"
	nodeUtilsMemory        = "100Mi"
	nodeUtilsPortName      = "node-utils"
	nodeUtilsPort          = 8000

	startupTimeout = 5 * time.Minute
	nonRootId      = 1000

	privValidatorListenAddress = "tcp://0.0.0.0:26658"

	defaultStateSyncTrustPeriod = "168h0m0s"
)

var (
	nodeUtilsCpuResources    = resource.MustParse(nodeUtilsCPU)
	nodeUtilsMemoryResources = resource.MustParse(nodeUtilsMemory)
)
