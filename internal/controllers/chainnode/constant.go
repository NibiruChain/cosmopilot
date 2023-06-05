package chainnode

import "time"

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
)
