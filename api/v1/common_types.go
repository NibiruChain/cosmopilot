package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// Reasons for events
const (
	ReasonPvcResized         = "PvcResized"
	ReasonPvcMaxReached      = "PvcMaxSizeReached"
	ReasonDataInitialized    = "DataInitialized"
	ReasonNodeKeyCreated     = "NodeKeyCreated"
	ReasonNodeKeyImported    = "NodeKeyImported"
	ReasonPrivateKeyCreated  = "PrivateKeyCreated"
	ReasonPrivateKeyImported = "PrivateKeyImported"
	ReasonAccountCreated     = "AccountCreated"
	ReasonAccountImported    = "AccountImported"
	ReasonGenesisInitialized = "GenesisCreated"
	ReasonGenesisImported    = "GenesisImported"
	ReasonConfigsCreated     = "ConfigsCreated"
	ReasonConfigsUpdated     = "ConfigsUpdated"
	ReasonNodeStarted        = "NodeStarted"
	ReasonNodeRestarted      = "NodeRestarted"
	ReasonNodeSyncing        = "NodeSyncing"
	ReasonNodeRunning        = "NodeRunning"
	ReasonValidatorJailed    = "ValidatorJailed"
	ReasonValidatorUnjailed  = "ValidatorUnjailed"
	ReasonNodeCreated        = "NodeCreated"
	ReasonNodeUpdated        = "NodeUpdated"
	ReasonNodeDeleted        = "NodeDeleted"
	ReasonInitGenesisFailure = "InitGenesisFail"
)

// AppSpec specifies the source image and binary name of the app to run
type AppSpec struct {
	// Image indicates the docker image to be used
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Version is the image tag to be used. Defaults to `latest`.
	// +optional
	// +default=latest
	Version *string `json:"version,omitempty"`

	// ImagePullPolicy indicates the desired pull policy when creating nodes. Defaults to `Always` if `version`
	// is `latest` and `IfNotPresent` otherwise.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// App is the name of the binary of the application to be run
	App string `json:"app"`
}

// ValidatorInfo contains information about this validator.
type ValidatorInfo struct {
	// Moniker to be used by this validator. Defaults to the ChainNode name.
	// +optional
	Moniker *string `json:"moniker,omitempty"`

	// Details of this validator.
	// +optional
	Details *string `json:"details,omitempty"`

	// Website indicates this validator's website.
	// +optional
	Website *string `json:"website,omitempty"`

	// Identity signature of this validator.
	// +optional
	Identity *string `json:"identity,omitempty"`
}

// GenesisInitConfig specifies configs and initialization commands for creating a new chain and its genesis
type GenesisInitConfig struct {
	// ChainID of the chain to initialize.
	ChainID string `json:"chainID"`

	// AccountMnemonicSecret is the name of the secret containing the mnemonic of the account to be used by
	// this validator. Defaults to `<chainnode>-account`. Will be created if does not exist.
	AccountMnemonicSecret *string `json:"accountMnemonicSecret,omitempty"`

	// AccountHDPath is the HD path for the validator account. Defaults to `m/44'/118'/0'/0/0`.
	// +optional
	AccountHDPath *string `json:"accountHDPath,omitempty"`

	// AccountPrefix is the prefix for accounts. Defaults to `nibi`.
	// +optional
	AccountPrefix *string `json:"accountPrefix,omitempty"`

	// ValPrefix is the prefix for validator accounts. Defaults to `nibivaloper`.
	// +optional
	ValPrefix *string `json:"valPrefix,omitempty"`

	// Assets is the list of tokens and their amounts to be assigned to this validators account.
	Assets []string `json:"assets"`

	// StakeAmount represents the amount to be staked by this validator.
	StakeAmount string `json:"stakeAmount"`

	// Accounts specify additional accounts and respective assets to be added to this chain.
	// +optional
	Accounts []AccountAssets `json:"accounts,omitempty"`

	// UnbondingTime is the time that takes to unbond delegations. Defaults to `1814400s`.
	// +optional
	UnbondingTime *string `json:"unbondingTime,omitempty"`

	// VotingPeriod indicates the voting period for this chain. Defaults to `120h`.
	// +optional
	VotingPeriod *string `json:"votingPeriod,omitempty"`

	// AdditionalInitCommands are additional commands to run on genesis initialization.
	// App home is at `/home/app` and `/temp` is a temporary volume shared by all init containers.
	// +optional
	AdditionalInitCommands []InitCommand `json:"additionalInitCommands,omitempty"`
}

type AccountAssets struct {
	// Address of the account.
	Address string `json:"address"`

	// Assets to be assigned to this account.
	Assets []string `json:"assets"`
}

type InitCommand struct {
	// Image to be used to run this command. Defaults to app image.
	// +optional
	Image *string `json:"image,omitempty"`

	// Command to be used. Defaults to image entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args to be passed to this command.
	Args []string `json:"args"`
}

// GenesisConfig specifies how genesis will be retrieved
type GenesisConfig struct {
	// URL to download the genesis from.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Url *string `json:"url,omitempty"`

	// ConfigMap specifies a configmap to load the genesis from
	// +optional
	ConfigMap *string `json:"configMap,omitempty"`
}

type Peer struct {
	// ID refers to tendermint node ID for this node
	ID string `json:"id"`

	// Address is the hostname or IP address of this peer
	Address string `json:"address"`

	// Port is the P2P port to be used. Defaults to `26656`.
	// +optional
	Port *int `json:"port,omitempty"`

	// Unconditional marks this peer as unconditional.
	// +optional
	Unconditional *bool `json:"unconditional,omitempty"`

	// Private marks this peer as private.
	// +optional
	Private *bool `json:"private,omitempty"`
}
