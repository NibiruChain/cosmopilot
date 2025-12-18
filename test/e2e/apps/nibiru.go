package apps

import (
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
)

// Nibiru returns the test configuration for Nibiru blockchain.
// Configuration based on examples/nibiru/testnet-with-fullnode.yaml
func Nibiru() TestApp {
	// Add sudo root account
	sudoRootAccountCmd := appsv1.InitCommand{
		Command: []string{"sh", "-c"},
		Args: []string{
			`nibid genesis add-sudo-root-account $(nibid keys show account -a --home=/home/app --keyring-backend test) --home=/home/app`,
		},
	}

	return TestApp{
		Name:          "Nibiru",
		Architectures: []string{"amd64", "arm64"},
		UpgradeTests: []UpgradeTestConfig{
			{
				UpgradeName: "v2.9.0",
				FromVersion: "2.8.0",
				ToVersion:   "2.9.0",
			},
		},
		AppSpec: appsv1.AppSpec{
			Image:      "ghcr.io/nibiruchain/nibiru",
			Version:    ptr.To("2.9.0"),
			App:        "nibid",
			SdkVersion: ptr.To(appsv1.V0_47),
		},
		ValidatorConfig: ValidatorTestConfig{
			ChainID:                "nibiru-e2e",
			Denom:                  "unibi",
			Assets:                 []string{"100000000000000unibi"},
			StakeAmount:            "100000000unibi",
			AccountPrefix:          "nibi",
			ValPrefix:              "nibivaloper",
			AdditionalInitCommands: []appsv1.InitCommand{sudoRootAccountCmd},
			PrivKey:                `{"address":"DE623086321818A30ADF4A8D68EEBEBDBF78B0F9","pub_key":{"type":"tendermint/PubKeyEd25519","value":"vwvZODnQoT31PwNN4ZhwIOwfSQ/iar4QAa0C6Tr5yVw="},"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"LXYv7Ogc5tiniSiKRUrkcVP5IpgyE5qr9h5wSTmphwu/C9k4OdChPfU/A03hmHAg7B9JD+JqvhABrQLpOvnJXA=="}}`,
			ExpectedPubKey:         `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"vwvZODnQoT31PwNN4ZhwIOwfSQ/iar4QAa0C6Tr5yVw="}`,
		},
	}
}
