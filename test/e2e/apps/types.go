package apps

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
)

// TestApp defines a blockchain application configuration for e2e testing.
// Each TestApp contains all the configuration needed to run a validator
// and fullnodes for a specific blockchain.
type TestApp struct {
	// Name is the display name for test output
	Name string

	// AppSpec defines the container image and binary configuration
	AppSpec appsv1.AppSpec

	// ValidatorConfig contains validator-specific test settings
	ValidatorConfig ValidatorTestConfig

	// FullnodeConfig contains optional fullnode-specific settings
	// If nil, fullnodes will inherit from validator config where applicable
	FullnodeConfig *FullnodeTestConfig

	// Architectures lists the CPU architectures supported by this app's Docker image.
	// Valid values are "amd64" and "arm64". If empty, defaults to all architectures.
	Architectures []string

	// UpgradeTests contains configurations for upgrade e2e tests.
	// Each entry defines a from->to version upgrade scenario.
	// If empty, upgrade tests will be skipped for this app.
	UpgradeTests []UpgradeTestConfig
}

// UpgradeTestConfig contains versions for testing app upgrades via governance.
type UpgradeTestConfig struct {
	// UpgradeName is the name used in the governance upgrade proposal.
	// This must match the upgrade handler registered in the binary.
	UpgradeName string

	// FromVersion is the version to start from before the upgrade.
	FromVersion string

	// ToVersion is the version to upgrade to. If empty, uses the default version.
	ToVersion string

	// ToImage is the full image reference to upgrade to. If empty, uses default image with ToVersion.
	ToImage string
}

// ValidatorTestConfig contains configuration specific to validator nodes
type ValidatorTestConfig struct {
	// ChainID for the test network
	ChainID string

	// Denom is the primary staking denomination
	Denom string

	// Assets are the genesis account balances
	Assets []string

	// StakeAmount is the initial stake for the validator
	StakeAmount string

	// AccountPrefix is the bech32 account prefix (e.g., "nibi", "osmo")
	AccountPrefix string

	// ValPrefix is the bech32 validator prefix (e.g., "nibivaloper")
	ValPrefix string

	// AdditionalVolumes for WASM, etc.
	AdditionalVolumes []appsv1.VolumeSpec

	// RunFlags are runtime flags for the node binary
	RunFlags []string

	// AdditionalInitCommands run during genesis initialization
	AdditionalInitCommands []appsv1.InitCommand

	// PrivKey is a pre-generated private key JSON for import tests.
	// If empty, the private key import test will be skipped for this app.
	PrivKey string

	// ExpectedPubKey is the expected public key JSON after importing PrivKey.
	// Must be set if PrivKey is set.
	ExpectedPubKey string
}

// FullnodeTestConfig contains configuration specific to fullnodes
type FullnodeTestConfig struct {
	// AdditionalVolumes for WASM, etc.
	AdditionalVolumes []appsv1.VolumeSpec

	// RunFlags are runtime flags for the node binary
	RunFlags []string
}

// BuildChainNode creates a ChainNode resource for testing
func (t TestApp) BuildChainNode(namespace string) *appsv1.ChainNode {
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "e2e-chainnode-",
			Namespace:    namespace,
		},
		Spec: appsv1.ChainNodeSpec{
			App: t.AppSpec,
			Validator: &appsv1.ValidatorConfig{
				Init: &appsv1.GenesisInitConfig{
					ChainID:               t.ValidatorConfig.ChainID,
					Assets:                t.ValidatorConfig.Assets,
					StakeAmount:           t.ValidatorConfig.StakeAmount,
					AccountPrefix:         ptr.To(t.ValidatorConfig.AccountPrefix),
					ValPrefix:             ptr.To(t.ValidatorConfig.ValPrefix),
					VotingPeriod:          ptr.To[string]("15s"),
					ExpeditedVotingPeriod: ptr.To[string]("10s"),
				},
			},
		},
	}

	// Add persistence config if there are additional volumes
	if len(t.ValidatorConfig.AdditionalVolumes) > 0 {
		chainNode.Spec.Persistence = &appsv1.Persistence{
			AdditionalVolumes: t.ValidatorConfig.AdditionalVolumes,
		}
	}

	// Add config with run flags
	if len(t.ValidatorConfig.RunFlags) > 0 {
		chainNode.Spec.Config = &appsv1.Config{
			RunFlags: t.ValidatorConfig.RunFlags,
		}
	}

	// Add additional init commands
	if len(t.ValidatorConfig.AdditionalInitCommands) > 0 {
		chainNode.Spec.Validator.Init.AdditionalInitCommands = t.ValidatorConfig.AdditionalInitCommands
	}

	return chainNode
}

// BuildChainNodeSet creates a ChainNodeSet resource for testing
func (t TestApp) BuildChainNodeSet(namespace string, fullnodes int) *appsv1.ChainNodeSet {
	chainNodeSet := &appsv1.ChainNodeSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "e2e-chainnodeset-",
			Namespace:    namespace,
		},
		Spec: appsv1.ChainNodeSetSpec{
			App: t.AppSpec,
			Validator: &appsv1.NodeSetValidatorConfig{
				Init: &appsv1.GenesisInitConfig{
					ChainID:               t.ValidatorConfig.ChainID,
					Assets:                t.ValidatorConfig.Assets,
					StakeAmount:           t.ValidatorConfig.StakeAmount,
					AccountPrefix:         ptr.To(t.ValidatorConfig.AccountPrefix),
					ValPrefix:             ptr.To(t.ValidatorConfig.ValPrefix),
					VotingPeriod:          ptr.To[string]("15s"),
					ExpeditedVotingPeriod: ptr.To[string]("10s"),
				},
			},
			Nodes: []appsv1.NodeGroupSpec{
				{
					Name:      "fullnodes",
					Instances: ptr.To(fullnodes),
				},
			},
		},
	}

	// Add validator persistence config
	if len(t.ValidatorConfig.AdditionalVolumes) > 0 {
		chainNodeSet.Spec.Validator.Persistence = &appsv1.Persistence{
			AdditionalVolumes: t.ValidatorConfig.AdditionalVolumes,
		}
	}

	// Add validator config with run flags
	if len(t.ValidatorConfig.RunFlags) > 0 {
		chainNodeSet.Spec.Validator.Config = &appsv1.Config{
			RunFlags: t.ValidatorConfig.RunFlags,
		}
	}

	// Add additional init commands
	if len(t.ValidatorConfig.AdditionalInitCommands) > 0 {
		chainNodeSet.Spec.Validator.Init.AdditionalInitCommands = t.ValidatorConfig.AdditionalInitCommands
	}

	// Add fullnode-specific config if provided
	if t.FullnodeConfig != nil {
		if len(t.FullnodeConfig.AdditionalVolumes) > 0 {
			chainNodeSet.Spec.Nodes[0].Persistence = &appsv1.Persistence{
				AdditionalVolumes: t.FullnodeConfig.AdditionalVolumes,
			}
		}
		if len(t.FullnodeConfig.RunFlags) > 0 {
			chainNodeSet.Spec.Nodes[0].Config = &appsv1.Config{
				RunFlags: t.FullnodeConfig.RunFlags,
			}
		}
	} else {
		// Inherit from validator config for fullnodes
		if len(t.ValidatorConfig.AdditionalVolumes) > 0 {
			chainNodeSet.Spec.Nodes[0].Persistence = &appsv1.Persistence{
				AdditionalVolumes: t.ValidatorConfig.AdditionalVolumes,
			}
		}
		if len(t.ValidatorConfig.RunFlags) > 0 {
			chainNodeSet.Spec.Nodes[0].Config = &appsv1.Config{
				RunFlags: t.ValidatorConfig.RunFlags,
			}
		}
	}

	return chainNodeSet
}

// WithVersion returns a copy of the TestApp with a different version
func (t TestApp) WithVersion(version string) TestApp {
	copy := t
	copy.AppSpec.Version = ptr.To(version)
	return copy
}
