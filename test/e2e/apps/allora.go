package apps

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

// Allora returns the test configuration for Allora Network blockchain.
// Configuration based on examples/allora/testnet-with-fullnode.yaml
func Allora() TestApp {
	// Replace stake with uallo in genesis
	denomReplacementCmd := appsv1.InitCommand{
		Image:   ptr.To("busybox"),
		Command: []string{"sh", "-c"},
		Args: []string{
			`sed -i 's/"stake"/"uallo"/g' /home/app/config/genesis.json;`,
		},
	}

	return TestApp{
		Name:          "Allora",
		Architectures: []string{"amd64"}, // No arm64 image available
		UpgradeTests: []UpgradeTestConfig{
			{
				UpgradeName: "v0.14.0",
				FromVersion: "v0.13.0",
				ToVersion:   "v0.14.0",
			},
		},
		AppSpec: appsv1.AppSpec{
			Image:      "alloranetwork/allora-chain",
			Version:    ptr.To("v0.14.0"),
			App:        "allorad",
			SdkVersion: ptr.To(appsv1.V0_50),
		},
		ConfigOverride: &map[string]runtime.RawExtension{
			"app.toml": {
				Raw: []byte(`{"minimum-gas-prices": "10uallo"}`),
			},
		},
		ValidatorConfig: ValidatorTestConfig{
			ChainID:                "allora-network-e2e",
			Denom:                  "uallo",
			Assets:                 []string{"1000000000allo"},
			StakeAmount:            "10allo",
			AccountPrefix:          "allo",
			ValPrefix:              "allovaloper",
			AdditionalInitCommands: []appsv1.InitCommand{denomReplacementCmd},
			PrivKey:                `{"address":"B9FC7DCF7902D8F4C9CF3516D8AD89AF82097921","pub_key":{"type":"tendermint/PubKeyEd25519","value":"lkQs7mneRKD46mJW+OQV9v4J9YbbbW/Y4xemUZN7FXE="},"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"DkCbnUW+xib4TrZFQbI4eXxGoGIWy/NDY26ypgrVyqWWRCzuad5EoPjqYlb45BX2/gn1htttb9jjF6ZRk3sVcQ=="}}`,
			ExpectedPubKey:         `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"lkQs7mneRKD46mJW+OQV9v4J9YbbbW/Y4xemUZN7FXE="}`,
			SkipChainIDForQueries:  true,
		},
	}
}
