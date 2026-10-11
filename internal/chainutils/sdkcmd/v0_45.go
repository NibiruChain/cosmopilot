package sdkcmd

import (
	"fmt"
	"strings"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

func init() {
	RegisterSDK(appsv1.V0_45, func(globalOptions ...Option) SDK {
		return newV0_45(globalOptions...)
	})
}

func newV0_45(globalOptions ...Option) *v0_45 {
	options := newDefaultOptions()
	for _, option := range globalOptions {
		option(options)
	}
	return &v0_45{options: options}
}

type v0_45 struct {
	options *Options
}

func (sdk *v0_45) InitArgs(moniker, chainID string) []string {
	return append(
		[]string{"init", moniker, "--chain-id", chainID},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_45) RecoverAccountArgs(account string) []string {
	return append(
		[]string{"keys", "add", account, "--recover",
			"--keyring-backend", "test",
		},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_45) AddGenesisAccountArgs(account string, assets []string) []string {
	return append(
		[]string{"add-genesis-account", account, strings.Join(assets, ",")},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_45) GenTxArgs(account, moniker, stakeAmount, chainID string, options ...*ArgOption) []string {
	args := []string{
		"gentx", account, stakeAmount,
		"--moniker", moniker,
		"--chain-id", chainID,
		"--keyring-backend", "test",
		"--yes",
	}
	args = applyArgOptions(args, options)
	return append(args, sdk.options.GlobalArgs...)
}

func (sdk *v0_45) CollectGenTxsArgs() []string {
	return append(
		[]string{"collect-gentxs"},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_45) CreateValidatorArgs(account, pubKey, moniker, stakeAmount, chainID, gasPrices string, options ...*ArgOption) []string {
	args := []string{
		"tx", "staking", "create-validator",
		"--amount", stakeAmount,
		"--moniker", moniker,
		"--chain-id", chainID,
		"--pubkey", pubKey,
		"--gas-prices", gasPrices,
		"--from", account,
		"--keyring-backend", "test",
		"--yes",
	}
	args = applyArgOptions(args, options)
	return append(args, sdk.options.GlobalArgs...)
}

func (sdk *v0_45) GenesisSetUnbondingTimeCmd(unbondingTime, genesisFile string) string {
	return fmt.Sprintf("jq '.app_state.staking.params.unbonding_time = %q' %s > /tmp/genesis.tmp && mv /tmp/genesis.tmp %s",
		unbondingTime, genesisFile, genesisFile,
	)
}

func (sdk *v0_45) GenesisSetVotingPeriodCmd(votingPeriod, genesisFile string) string {
	return fmt.Sprintf("jq '.app_state.gov.voting_params.voting_period = %q' %s > /tmp/genesis.tmp && mv /tmp/genesis.tmp %s",
		votingPeriod, genesisFile, genesisFile,
	)
}

// GenesisSetExpeditedVotingPeriodCmd is not supported in v0.45, returns empty string.
func (sdk *v0_45) GenesisSetExpeditedVotingPeriodCmd(votingPeriod, genesisFile string) string {
	return ""
}
