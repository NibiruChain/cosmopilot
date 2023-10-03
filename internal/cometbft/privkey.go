package cometbft

import (
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/privval"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
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

func GetPubKey(keyb []byte) (string, error) {
	pvKey := privval.FilePVKey{}
	err := json.Unmarshal(keyb, &pvKey)
	if err != nil {
		return "", err
	}

	sdkPK, err := cryptocodec.FromTmPubKeyInterface(pvKey.PrivKey.PubKey())
	if err != nil {
		return "", err
	}

	reg := types.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(reg)
	c := codec.NewProtoCodec(reg)
	b, err := c.MarshalInterfaceJSON(sdkPK)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func UnpackPubKey(x *types.Any) (string, error) {
	reg := types.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(reg)
	c := codec.NewProtoCodec(reg)

	var pk cryptotypes.PubKey
	if err := reg.UnpackAny(x, &pk); err != nil {
		return "", err
	}

	b, err := c.MarshalInterfaceJSON(pk)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
