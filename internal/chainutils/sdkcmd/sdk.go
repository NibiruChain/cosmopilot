package sdkcmd

import (
	"fmt"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
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

type SDK interface {
	InitArgs(moniker, chainID string) []string
	RecoverAccountArgs(account string) []string
	AddGenesisAccountArgs(account string, assets []string) []string
	GenTxArgs(account, moniker, stakeAmount, chainID string, options ...*ArgOption) []string
	CollectGenTxsArgs() []string
	CreateValidatorArgs(account, pubKey, moniker, stakeAmount, chainID, gasPrices string, options ...*ArgOption) []string

	GenesisSetUnbondingTimeCmd(unbondingTime, genesisFile string) string
	GenesisSetVotingPeriodCmd(votingPeriod, genesisFile string) string
}

type Options struct {
	GlobalArgs []string
}

type Option func(*Options)

func newDefaultOptions() *Options {
	return &Options{
		GlobalArgs: make([]string, 0),
	}
}

func WithGlobalArg(key, value string) Option {
	return func(opts *Options) {
		opts.GlobalArgs = append(opts.GlobalArgs, fmt.Sprintf("--%s", key), value)
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
