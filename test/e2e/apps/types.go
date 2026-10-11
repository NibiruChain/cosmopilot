package apps

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

// defaultConfigOverride contains hardcoded config overrides applied to all test chains.
// These can be extended by TestApp.ConfigOverride which will be merged on top.
var defaultConfigOverride = map[string]runtime.RawExtension{
	"config.toml": {
		Raw: []byte(`{"consensus":{"timeout_commit":"1s"}}`),
	},
}

// defaultConfig returns the default Config for e2e tests.
// This sets faster reconciliation for quicker test feedback.
func defaultConfig() *appsv1.Config {
	return &appsv1.Config{
		ReconcilePeriod: ptr.To("5s"),
		BlockThreshold:  ptr.To("0s"),
	}
}

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

	// ConfigOverride allows overriding configs on `.toml` configuration files.
	ConfigOverride *map[string]runtime.RawExtension
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

	// SkipChainIDForQueries skips --chain-id flag for query commands.
	// Some chains (like Allora) get chain-id from the node and fail if it's specified.
	SkipChainIDForQueries bool
}

// FullnodeTestConfig contains configuration specific to fullnodes
type FullnodeTestConfig struct {
	// AdditionalVolumes for WASM, etc.
	AdditionalVolumes []appsv1.VolumeSpec

	// RunFlags are runtime flags for the node binary
	RunFlags []string
}

// buildConfig creates a Config by starting with defaults and merging TestApp-specific settings.
// It merges: defaultConfig -> runFlags -> config overrides (defaultConfigOverride + TestApp.ConfigOverride)
func (t TestApp) buildConfig(runFlags []string) *appsv1.Config {
	cfg := defaultConfig()

	// Merge run flags
	if len(runFlags) > 0 {
		cfg.RunFlags = runFlags
	}

	// Merge config overrides
	merged := mergeConfigOverrides(defaultConfigOverride, t.ConfigOverride)
	if merged != nil {
		cfg.Override = merged
	}

	return cfg
}

// buildPersistence creates a Persistence config, or nil if no additional volumes are specified.
func buildPersistence(volumes []appsv1.VolumeSpec) *appsv1.Persistence {
	if len(volumes) == 0 {
		return nil
	}
	return &appsv1.Persistence{
		AdditionalVolumes: volumes,
	}
}

// mergeConfigOverrides merges multiple config override maps together.
// For each file key, if both maps have JSON objects, they are deep-merged.
// Returns nil if the result is empty.
func mergeConfigOverrides(base map[string]runtime.RawExtension, overlay *map[string]runtime.RawExtension) *map[string]runtime.RawExtension {
	if len(base) == 0 && overlay == nil {
		return nil
	}

	result := make(map[string]runtime.RawExtension)

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Merge overlay
	if overlay != nil {
		for k, v := range *overlay {
			if existing, ok := result[k]; ok {
				// Both have the same file key, deep merge the JSON
				merged := mergeJSON(existing.Raw, v.Raw)
				result[k] = runtime.RawExtension{Raw: merged}
			} else {
				result[k] = v
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return &result
}

// mergeJSON deep-merges two JSON byte slices. Values in b override values in a.
func mergeJSON(a, b []byte) []byte {
	var aMap, bMap map[string]interface{}

	if err := json.Unmarshal(a, &aMap); err != nil {
		// If a is not valid JSON, just return b
		return b
	}
	if err := json.Unmarshal(b, &bMap); err != nil {
		// If b is not valid JSON, just return a
		return a
	}

	merged := deepMerge(aMap, bMap)
	result, err := json.Marshal(merged)
	if err != nil {
		return b
	}
	return result
}

// deepMerge recursively merges two maps. Values in b override values in a.
func deepMerge(a, b map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all from a
	for k, v := range a {
		result[k] = v
	}

	// Merge b into result
	for k, vb := range b {
		if va, ok := result[k]; ok {
			// Both have the same key, check if both are maps
			aMap, aIsMap := va.(map[string]interface{})
			bMap, bIsMap := vb.(map[string]interface{})
			if aIsMap && bIsMap {
				result[k] = deepMerge(aMap, bMap)
			} else {
				result[k] = vb
			}
		} else {
			result[k] = vb
		}
	}

	return result
}

// BuildChainNode creates a ChainNode resource for testing
func (t TestApp) BuildChainNode(namespace string) *appsv1.ChainNode {
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-%s-chainnode-", strings.ToLower(t.Name)),
			Namespace:    namespace,
		},
		Spec: appsv1.ChainNodeSpec{
			App:         t.AppSpec,
			Config:      t.buildConfig(t.ValidatorConfig.RunFlags),
			Persistence: buildPersistence(t.ValidatorConfig.AdditionalVolumes),
			Validator: &appsv1.ValidatorConfig{
				AccountPrefix: ptr.To(t.ValidatorConfig.AccountPrefix),
				ValPrefix:     ptr.To(t.ValidatorConfig.ValPrefix),
				Init: &appsv1.GenesisInitConfig{
					ChainID:                t.ValidatorConfig.ChainID,
					Assets:                 t.ValidatorConfig.Assets,
					StakeAmount:            t.ValidatorConfig.StakeAmount,
					VotingPeriod:           ptr.To[string]("15s"),
					ExpeditedVotingPeriod:  ptr.To[string]("10s"),
					AdditionalInitCommands: t.ValidatorConfig.AdditionalInitCommands,
				},
			},
		},
	}

	return chainNode
}

// BuildChainNodeSet creates a ChainNodeSet resource for testing
func (t TestApp) BuildChainNodeSet(namespace string, fullnodes int) *appsv1.ChainNodeSet {
	// Determine fullnode config (use dedicated config or inherit from validator)
	var fullnodeRunFlags []string
	var fullnodeVolumes []appsv1.VolumeSpec
	if t.FullnodeConfig != nil {
		fullnodeRunFlags = t.FullnodeConfig.RunFlags
		fullnodeVolumes = t.FullnodeConfig.AdditionalVolumes
	} else {
		fullnodeRunFlags = t.ValidatorConfig.RunFlags
		fullnodeVolumes = t.ValidatorConfig.AdditionalVolumes
	}

	return &appsv1.ChainNodeSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-%s-chainnodeset-", strings.ToLower(t.Name)),
			Namespace:    namespace,
		},
		Spec: appsv1.ChainNodeSetSpec{
			App: t.AppSpec,
			Validator: &appsv1.NodeSetValidatorConfig{
				Config:        t.buildConfig(t.ValidatorConfig.RunFlags),
				Persistence:   buildPersistence(t.ValidatorConfig.AdditionalVolumes),
				AccountPrefix: ptr.To(t.ValidatorConfig.AccountPrefix),
				ValPrefix:     ptr.To(t.ValidatorConfig.ValPrefix),
				Init: &appsv1.GenesisInitConfig{
					ChainID:                t.ValidatorConfig.ChainID,
					Assets:                 t.ValidatorConfig.Assets,
					StakeAmount:            t.ValidatorConfig.StakeAmount,
					VotingPeriod:           ptr.To[string]("15s"),
					ExpeditedVotingPeriod:  ptr.To[string]("10s"),
					AdditionalInitCommands: t.ValidatorConfig.AdditionalInitCommands,
				},
			},
			Nodes: []appsv1.NodeGroupSpec{
				{
					Name:        "fullnodes",
					Instances:   ptr.To(fullnodes),
					Config:      t.buildConfig(fullnodeRunFlags),
					Persistence: buildPersistence(fullnodeVolumes),
				},
			},
		},
	}
}

// WithVersion returns a copy of the TestApp with a different version
func (t TestApp) WithVersion(version string) TestApp {
	copy := t
	copy.AppSpec.Version = ptr.To(version)
	return copy
}

// BuildChainNodeWithMockMode creates a ChainNode with node-utils mock mode enabled.
// This is useful for E2E testing VPA functionality with controllable resource usage.
func (t TestApp) BuildChainNodeWithMockMode(namespace string) *appsv1.ChainNode {
	chainNode := t.BuildChainNode(namespace)
	chainNode.Spec.Config.NodeUtilsEnv = append(chainNode.Spec.Config.NodeUtilsEnv,
		corev1.EnvVar{
			Name:  "MOCK_MODE",
			Value: "true",
		},
		corev1.EnvVar{
			Name:  "LOG_LEVEL",
			Value: "debug",
		},
	)
	return chainNode
}

// BuildChainNodeWithVPA creates a ChainNode with VPA and mock mode enabled.
// This is the recommended setup for E2E testing VPA functionality.
func (t TestApp) BuildChainNodeWithVPA(namespace string, vpa *appsv1.VerticalAutoscalingConfig) *appsv1.ChainNode {
	chainNode := t.BuildChainNodeWithMockMode(namespace)
	chainNode.Spec.VPA = vpa
	return chainNode
}

// TmKMSConfig holds configuration for building a ChainNode with TMKMS
type TmKMSConfig struct {
	// VaultAddress is the full address of the Vault cluster
	VaultAddress string

	// KeyName is the key path in Vault (e.g., "transit/keys/chain-validator")
	KeyName string

	// TokenSecretName is the name of the secret containing the Vault token
	TokenSecretName string

	// CASecretName is the name of the secret containing the Vault CA certificate
	CASecretName string
}

// BuildChainNodeWithTmKMS creates a ChainNode resource with TMKMS Vault configuration
func (t TestApp) BuildChainNodeWithTmKMS(namespace string, tmkmsConfig TmKMSConfig) *appsv1.ChainNode {
	chainNode := t.BuildChainNode(namespace)

	// Configure TMKMS with Vault provider
	chainNode.Spec.Validator.TmKMS = &appsv1.TmKMS{
		Provider: appsv1.TmKmsProvider{
			Hashicorp: &appsv1.TmKmsHashicorpProvider{
				Address: tmkmsConfig.VaultAddress,
				Key:     tmkmsConfig.KeyName,
				TokenSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tmkmsConfig.TokenSecretName,
					},
					Key: "token",
				},
				CertificateSecret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tmkmsConfig.CASecretName,
					},
					Key: "ca.crt",
				},
				UploadGenerated: true,
			},
		},
		KeyFormat: &appsv1.TmKmsKeyFormat{
			Type:               "bech32",
			AccountKeyPrefix:   t.ValidatorConfig.AccountPrefix + "pub",
			ConsensusKeyPrefix: t.ValidatorConfig.ValPrefix + "conspub",
		},
	}

	return chainNode
}
