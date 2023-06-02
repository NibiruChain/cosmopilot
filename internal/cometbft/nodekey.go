package cometbft

import (
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
)

func GenerateNodeKey() (string, []byte, error) {
	privKey := ed25519.GenPrivKey()
	nodeKey := &p2p.NodeKey{
		PrivKey: privKey,
	}
	b, err := json.Marshal(nodeKey)
	if err != nil {
		return "", nil, err
	}
	return string(p2p.PubKeyToID(nodeKey.PubKey())), b, nil
}

func GetNodeID(key []byte) (string, error) {
	var nodeKey p2p.NodeKey
	if err := json.Unmarshal(key, &nodeKey); err != nil {
		return "", err
	}
	return string(p2p.PubKeyToID(nodeKey.PubKey())), nil
}
