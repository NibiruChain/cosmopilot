package chainutils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

const testMnemonic = "upset promote follow flag you way eagle plunge scorpion oil version afraid churn fog tiger almost noise define license pistol post raise report time"

func TestAccountFromMnemonic(t *testing.T) {
	tests := []struct {
		provided string
		expected *Account
	}{
		{
			provided: testMnemonic,
			expected: &Account{
				Mnemonic:         testMnemonic,
				Address:          "nibi1ll3njapxnyqqvfz65puwvmmya23a0xcqhfkkat",
				ValidatorAddress: "nibivaloper1ll3njapxnyqqvfz65puwvmmya23a0xcq7jcdfk",
			},
		},
	}

	for _, test := range tests {
		result, err := AccountFromMnemonic(test.provided, appsv1.DefaultAccountPrefix, appsv1.DefaultValPrefix, appsv1.DefaultHDPath)
		assert.NoError(t, err)
		assert.Equal(t, test.expected.Mnemonic, result.Mnemonic)
		assert.Equal(t, test.expected.ValidatorAddress, result.ValidatorAddress)
		assert.Equal(t, test.expected.Address, result.Address)
	}
}
