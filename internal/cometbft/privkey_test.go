package cometbft

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPubKey(t *testing.T) {
	tests := []struct {
		provided string
		expected string
	}{
		{
			provided: "{\"address\":\"0E85BC4610C7710A01CA6CC98E2CC5CFE9935690\",\"pub_key\":{\"type\":\"tendermint/PubKeyEd25519\",\"value\":\"WSNiGcovSATN09MKkaqFDOQgypn1FPDhVwfYIPFVp34=\"},\"priv_key\":{\"type\":\"tendermint/PrivKeyEd25519\",\"value\":\"24Jswwf3dxtD5r6byv8agLgjCuvoz2I7UXCsJmrjLp1ZI2IZyi9IBM3T0wqRqoUM5CDKmfUU8OFXB9gg8VWnfg==\"}}",
			expected: "{\"@type\":\"/cosmos.crypto.ed25519.PubKey\",\"key\":\"WSNiGcovSATN09MKkaqFDOQgypn1FPDhVwfYIPFVp34=\"}",
		},
	}

	for _, test := range tests {
		pubKey, err := GetPubKey([]byte(test.provided))
		assert.NoError(t, err)
		assert.Equal(t, test.expected, pubKey)
	}
}
