package chainutils

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
)

const mnemonicEntropySize = 256

type Account struct {
	Mnemonic         string
	Address          string
	ValidatorAddress string
}

func CreateAccount(accPrefix, valPrefix, hdPath string) (*Account, error) {
	entropySeed, err := bip39.NewEntropy(mnemonicEntropySize)
	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return nil, err
	}

	// create master key and derive first key for keyring
	derivedPriv, err := hd.Secp256k1.Derive()(mnemonic, "", hdPath)
	if err != nil {
		return nil, err
	}
	privKey := hd.Secp256k1.Generate()(derivedPriv)

	accBech32, err := sdk.Bech32ifyAddressBytes(accPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, err
	}

	valBech32, err := sdk.Bech32ifyAddressBytes(valPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, err
	}

	return &Account{
		Mnemonic:         mnemonic,
		Address:          accBech32,
		ValidatorAddress: valBech32,
	}, nil
}

func AccountFromMnemonic(mnemonic, accPrefix, valPrefix, hdPath string) (*Account, error) {
	// create master key and derive first key for keyring
	derivedPriv, err := hd.Secp256k1.Derive()(mnemonic, "", hdPath)
	if err != nil {
		return nil, err
	}
	privKey := hd.Secp256k1.Generate()(derivedPriv)

	accBech32, err := sdk.Bech32ifyAddressBytes(accPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, err
	}

	valBech32, err := sdk.Bech32ifyAddressBytes(valPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, err
	}

	return &Account{
		Mnemonic:         mnemonic,
		Address:          accBech32,
		ValidatorAddress: valBech32,
	}, nil
}

func AccountAddressFromValidatorAddress(valAddr, valPrefix, accPrefix string) (string, error) {
	valBytes, err := sdk.GetFromBech32(valAddr, valPrefix)
	if err != nil {
		return "", err
	}

	accAddr, err := sdk.Bech32ifyAddressBytes(accPrefix, valBytes)
	if err != nil {
		return "", err
	}

	return accAddr, nil
}
