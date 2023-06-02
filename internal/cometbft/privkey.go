package cometbft

import (
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/privval"
)

func GeneratePrivKey() ([]byte, error) {
	privKey := ed25519.GenPrivKey()
	key := privval.FilePVKey{
		Address: privKey.PubKey().Address(),
		PubKey:  privKey.PubKey(),
		PrivKey: privKey,
	}
	return json.Marshal(key)
}
