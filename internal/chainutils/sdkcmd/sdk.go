package sdkcmd

import (
	"fmt"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

type sdkCreator func(globalOptions ...Option) SDK

var sdkVersions map[appsv1.SdkVersion]sdkCreator

func RegisterSDK(version appsv1.SdkVersion, creator sdkCreator) {
	if sdkVersions == nil {
		sdkVersions = make(map[appsv1.SdkVersion]sdkCreator)
	}
	sdkVersions[version] = creator
}

func GetSDK(version appsv1.SdkVersion, globalOptions ...Option) (SDK, error) {
	if creator, ok := sdkVersions[version]; ok {
		return creator(globalOptions...), nil
	}
	return nil, fmt.Errorf("unsupported sdk version")
}

// SDK defines the interface for Cosmos SDK version-specific commands.
// Each SDK version (v0.45, v0.47, v0.50, v0.53) implements this interface
// with version-appropriate command arguments and genesis modifications.
type SDK interface {
	// InitArgs returns arguments for initializing a new chain with the given moniker and chain ID.
	InitArgs(moniker, chainID string) []string

	// RecoverAccountArgs returns arguments for recovering an account from a mnemonic.
	RecoverAccountArgs(account string) []string

	// AddGenesisAccountArgs returns arguments for adding an account with assets to the genesis file.
	AddGenesisAccountArgs(account string, assets []string) []string

	// GenTxArgs returns arguments for generating a genesis transaction for a validator.
	GenTxArgs(account, moniker, stakeAmount, chainID string, options ...*ArgOption) []string

	// CollectGenTxsArgs returns arguments for collecting genesis transactions.
	CollectGenTxsArgs() []string

	// CreateValidatorArgs returns arguments for creating a validator on an existing chain.
	CreateValidatorArgs(account, pubKey, moniker, stakeAmount, chainID, gasPrices string, options ...*ArgOption) []string

	// GenesisSetUnbondingTimeCmd returns a shell command to set the unbonding time in the genesis file.
	GenesisSetUnbondingTimeCmd(unbondingTime, genesisFile string) string

	// GenesisSetVotingPeriodCmd returns a shell command to set the voting period in the genesis file.
	GenesisSetVotingPeriodCmd(votingPeriod, genesisFile string) string

	// GenesisSetExpeditedVotingPeriodCmd returns a shell command to set the expedited voting period.
	// Returns empty string for SDK versions that don't support it (< v0.50).
	GenesisSetExpeditedVotingPeriodCmd(votingPeriod, genesisFile string) string
}

type Options struct {
	GlobalArgs        []string
	GenesisSubcommand bool
}

type Option func(*Options)

func newDefaultOptions() *Options {
	return &Options{
		GlobalArgs:        make([]string, 0),
		GenesisSubcommand: true, // Default for v0.47+
	}
}

func WithGlobalArg(key, value string) Option {
	return func(opts *Options) {
		opts.GlobalArgs = append(opts.GlobalArgs, fmt.Sprintf("--%s", key), value)
	}
}

func WithGenesisSubcommand(use bool) Option {
	return func(opts *Options) {
		opts.GenesisSubcommand = use
	}
}

type ArgOption struct {
	Key   string
	Value string
}

func WithArg(key, value string) *ArgOption {
	return &ArgOption{Key: key, Value: value}
}

func WithOptionalArg(key string, value *string) *ArgOption {
	if value == nil {
		return nil
	}
	return WithArg(key, *value)
}

func applyArgOption(args []string, option *ArgOption) []string {
	if option != nil {
		args = append(args, fmt.Sprintf("--%s", option.Key), option.Value)
	}
	return args
}

func applyArgOptions(args []string, options []*ArgOption) []string {
	for _, o := range options {
		args = applyArgOption(args, o)
	}
	return args
}
