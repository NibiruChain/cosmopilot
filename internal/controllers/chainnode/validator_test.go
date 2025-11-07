package chainnode

import (
	"testing"

	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

func TestGetValidatorStatus(t *testing.T) {
	tests := []struct {
		name   string
		status stakingTypes.BondStatus
		want   appsv1.ValidatorStatus
	}{
		{
			name:   "bonded",
			status: stakingTypes.Bonded,
			want:   appsv1.ValidatorStatusBonded,
		},
		{
			name:   "unbonding",
			status: stakingTypes.Unbonding,
			want:   appsv1.ValidatorStatusUnbonding,
		},
		{
			name:   "unbonded",
			status: stakingTypes.Unbonded,
			want:   appsv1.ValidatorStatusUnbonded,
		},
		{
			name:   "unspecified",
			status: stakingTypes.Unspecified,
			want:   appsv1.ValidatorStatusUnknown,
		},
		{
			name:   "invalid status defaults to unknown",
			status: stakingTypes.BondStatus(999),
			want:   appsv1.ValidatorStatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getValidatorStatus(tt.status)
			if got != tt.want {
				t.Errorf("getValidatorStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
