package chainutils

import (
	"fmt"

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
		return nil, fmt.Errorf("generating entropy: %w", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return nil, fmt.Errorf("creating mnemonic: %w", err)
	}

	// create master key and derive first key for keyring
	derivedPriv, err := hd.Secp256k1.Derive()(mnemonic, "", hdPath)
	if err != nil {
		return nil, fmt.Errorf("deriving private key: %w", err)
	}
	privKey := hd.Secp256k1.Generate()(derivedPriv)

	accBech32, err := sdk.Bech32ifyAddressBytes(accPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, fmt.Errorf("encoding account address with prefix %s: %w", accPrefix, err)
	}

	valBech32, err := sdk.Bech32ifyAddressBytes(valPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, fmt.Errorf("encoding validator address with prefix %s: %w", valPrefix, err)
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
		return nil, fmt.Errorf("deriving private key from mnemonic: %w", err)
	}
	privKey := hd.Secp256k1.Generate()(derivedPriv)

	accBech32, err := sdk.Bech32ifyAddressBytes(accPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, fmt.Errorf("encoding account address with prefix %s: %w", accPrefix, err)
	}

	valBech32, err := sdk.Bech32ifyAddressBytes(valPrefix, privKey.PubKey().Address().Bytes())
	if err != nil {
		return nil, fmt.Errorf("encoding validator address with prefix %s: %w", valPrefix, err)
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
		return "", fmt.Errorf("decoding validator address %s: %w", valAddr, err)
	}

	accAddr, err := sdk.Bech32ifyAddressBytes(accPrefix, valBytes)
	if err != nil {
		return "", fmt.Errorf("encoding account address with prefix %s: %w", accPrefix, err)
	}

	return accAddr, nil
}
