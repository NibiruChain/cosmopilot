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

	labelNodeID    = "node-id"
	labelChainID   = "chain-id"
	labelValidator = "validator"

	annotationConfigHash = "apps.k8s.nibiru.org/config-hash"

	timeoutPodRunning = 5 * time.Minute
	timeoutPodDeleted = 30 * time.Second

	appContainerName = "app"

	nodeUtilsContainerName = "node-utils"
	nodeUtilsCPU           = "100m"
	nodeUtilsMemory        = "100Mi"
	nodeUtilsPortName      = "node-utils"
	nodeUtilsPort          = 8000

	startupTimeout = 5 * time.Minute
)

var (
	nodeUtilsCpuResources    = resource.MustParse(nodeUtilsCPU)
	nodeUtilsMemoryResources = resource.MustParse(nodeUtilsMemory)
)
