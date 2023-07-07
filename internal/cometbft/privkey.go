package cometbft

import (
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/privval"
)

type PrivKey struct {
	Address string `json:"address"`
	PubKey  struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"pub_key"`
	PrivKey struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"priv_key"`
}

func GeneratePrivKey() ([]byte, error) {
	privKey := ed25519.GenPrivKey()
	key := privval.FilePVKey{
		Address: privKey.PubKey().Address(),
		PubKey:  privKey.PubKey(),
		PrivKey: privKey,
	}
	return json.Marshal(key)
}

func LoadPrivKey(b []byte) (*PrivKey, error) {
	var key PrivKey
	if err := json.Unmarshal(b, &key); err != nil {
		return nil, err
	}
	return &key, nil
}
