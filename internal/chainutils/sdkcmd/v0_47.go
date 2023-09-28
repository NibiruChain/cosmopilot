package sdkcmd

import (
	"fmt"
	"strings"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

func init() {
	sdkCreator := func(globalOptions ...Option) SDK {
		options := newDefaultOptions()
		for _, option := range globalOptions {
			option(options)
		}
		v := v0_47{options: options}
		v.v0_45.options = options
		return &v
	}
	RegisterSDK(appsv1.V0_47, sdkCreator)
}

type v0_47 struct {
	v0_45   // Use this version for non specified methods
	options *Options
}

func (sdk *v0_47) AddGenesisAccountArgs(account string, assets []string) []string {
	return append(
		[]string{"genesis", "add-genesis-account", account, strings.Join(assets, ",")},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_47) GenTxArgs(account, moniker, stakeAmount, chainID string, options ...*ArgOption) []string {
	args := []string{
		"genesis", "gentx", account, stakeAmount,
		"--moniker", moniker,
		"--chain-id", chainID,
		"--keyring-backend", "test",
		"--yes",
	}
	args = applyArgOptions(args, options)
	return append(args, sdk.options.GlobalArgs...)
}

func (sdk *v0_47) CollectGenTxsArgs() []string {
	return append(
		[]string{"genesis", "collect-gentxs"},
		sdk.options.GlobalArgs...,
	)
}

func (sdk *v0_47) GenesisSetVotingPeriodCmd(votingPeriod, genesisFile string) string {
	return fmt.Sprintf("jq '.app_state.gov.params.voting_period = %q' %s > /tmp/genesis.tmp && mv /tmp/genesis.tmp %s",
		votingPeriod, genesisFile, genesisFile,
	)
}
