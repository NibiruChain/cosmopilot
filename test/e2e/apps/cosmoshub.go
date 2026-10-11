package apps

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

// CosmosHub returns the test configuration for Cosmos Hub (Gaia) blockchain.
// Configuration based on examples/cosmoshub/testnet-with-fullnode.yaml
func CosmosHub() TestApp {
	wasmVolumes := []appsv1.VolumeSpec{
		{
			Name:           "wasm",
			Size:           "1Gi",
			Path:           "/home/app/wasm",
			DeleteWithNode: ptr.To(true),
		},
	}

	// Replace stake with uatom in genesis
	denomReplacementCmd := appsv1.InitCommand{
		Image:   ptr.To("busybox"),
		Command: []string{"sh", "-c"},
		Args: []string{
			`sed -i 's/"stake"/"uatom"/g' /home/app/config/genesis.json;`,
		},
	}

	return TestApp{
		Name:          "CosmosHub",
		Architectures: []string{"amd64"}, // No arm64 image available
		UpgradeTests: []UpgradeTestConfig{
			{
				UpgradeName: "v25.2.0",
				FromVersion: "v25.1.0",
				ToVersion:   "v25.2.0",
			},
		},
		AppSpec: appsv1.AppSpec{
			Image:      "ghcr.io/cosmos/gaia",
			Version:    ptr.To("v25.2.0"),
			App:        "gaiad",
			SdkVersion: ptr.To(appsv1.V0_53),
		},
		ConfigOverride: &map[string]runtime.RawExtension{
			"app.toml": {
				Raw: []byte(`{"minimum-gas-prices": "0.025uatom"}`),
			},
		},
		ValidatorConfig: ValidatorTestConfig{
			ChainID:                "cosmoshub-e2e",
			Denom:                  "uatom",
			Assets:                 []string{"1000000000000000000uatom"},
			StakeAmount:            "100000000uatom",
			AccountPrefix:          "cosmos",
			ValPrefix:              "cosmosvaloper",
			AdditionalVolumes:      wasmVolumes,
			AdditionalInitCommands: []appsv1.InitCommand{denomReplacementCmd},
			PrivKey:                `{"address":"2E60D23142399E57CE90383488209D4D0D611417","pub_key":{"type":"tendermint/PubKeyEd25519","value":"qMFljV27udoPfWnn6OGku96t37ica2UqnpGn5tT8jKQ="},"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"Wq733bqBkyBm463FQHUpvZFek9uO2kh9NGTHKnCbpiyowWWNXbu52g99aefo4aS73q3fuJxrZSqekafm1PyMpA=="}}`,
			ExpectedPubKey:         `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"qMFljV27udoPfWnn6OGku96t37ica2UqnpGn5tT8jKQ="}`,
		},
		FullnodeConfig: &FullnodeTestConfig{
			AdditionalVolumes: wasmVolumes,
		},
	}
}
