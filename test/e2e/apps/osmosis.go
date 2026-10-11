package apps

import (
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

// Osmosis returns the test configuration for Osmosis blockchain.
// Configuration based on examples/osmosis/testnet-with-fullnode.yaml
func Osmosis() TestApp {
	wasmVolumes := []appsv1.VolumeSpec{
		{
			Name:           "wasm",
			Size:           "1Gi",
			Path:           "/home/app/wasm",
			DeleteWithNode: ptr.To(true),
		},
		{
			Name:           "ibc-08-wasm",
			Size:           "1Gi",
			Path:           "/home/app/ibc_08-wasm",
			DeleteWithNode: ptr.To(true),
		},
	}

	// Replace stake with uosmo in genesis
	denomReplacementCmd := appsv1.InitCommand{
		Image:   ptr.To("busybox"),
		Command: []string{"sh", "-c"},
		Args: []string{
			`sed -i 's/"stake"/"uosmo"/g' /home/app/config/genesis.json`,
		},
	}

	return TestApp{
		Name:          "Osmosis",
		Architectures: []string{"amd64", "arm64"},
		UpgradeTests: []UpgradeTestConfig{
			{
				UpgradeName: "v31",
				FromVersion: "30.0.0",
				ToVersion:   "31.0.0",
			},
		},
		AppSpec: appsv1.AppSpec{
			Image:      "osmolabs/osmosis",
			Version:    ptr.To("31.0.0"),
			App:        "osmosisd",
			SdkVersion: ptr.To(appsv1.V0_53),
			SdkOptions: &appsv1.SdkOptions{
				GenesisSubcommand: ptr.To(false),
			},
		},
		ValidatorConfig: ValidatorTestConfig{
			ChainID:                "osmosis-e2e",
			Denom:                  "uosmo",
			Assets:                 []string{"100000000000000000000uosmo"},
			StakeAmount:            "100000000uosmo",
			AccountPrefix:          "osmo",
			ValPrefix:              "osmovaloper",
			AdditionalVolumes:      wasmVolumes,
			RunFlags:               []string{"--reject-config-defaults=true"},
			AdditionalInitCommands: []appsv1.InitCommand{denomReplacementCmd},
			PrivKey:                `{"address":"8C06367F54575B2F0A550FB527248A92B3BA0E56","pub_key":{"type":"tendermint/PubKeyEd25519","value":"bxDGkHG3H7/KOQyc0h46/DgZiKNTtLXSr7OpGH8ax+8="},"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"wYHlmPRbIzxTK42GzzZuzHCs7GE47tf0EfhgvWDayKpvEMaQcbcfv8o5DJzSHjr8OBmIo1O0tdKvs6kYfxrH7w=="}}`,
			ExpectedPubKey:         `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"bxDGkHG3H7/KOQyc0h46/DgZiKNTtLXSr7OpGH8ax+8="}`,
		},
		FullnodeConfig: &FullnodeTestConfig{
			AdditionalVolumes: wasmVolumes,
			RunFlags:          []string{"--reject-config-defaults=true"},
		},
	}
}
