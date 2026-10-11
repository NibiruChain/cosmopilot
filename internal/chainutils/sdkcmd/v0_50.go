package sdkcmd

import (
	"fmt"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

func init() {
	RegisterSDK(appsv1.V0_50, func(globalOptions ...Option) SDK {
		return newV0_50(globalOptions...)
	})
}

func newV0_50(globalOptions ...Option) *v0_50 {
	return &v0_50{v0_47: *newV0_47(globalOptions...)}
}

type v0_50 struct {
	v0_47
}

func (sdk *v0_50) GenesisSetExpeditedVotingPeriodCmd(votingPeriod, genesisFile string) string {
	return fmt.Sprintf("jq '.app_state.gov.params.expedited_voting_period = %q' %s > /tmp/genesis.tmp && mv /tmp/genesis.tmp %s",
		votingPeriod, genesisFile, genesisFile,
	)
}
