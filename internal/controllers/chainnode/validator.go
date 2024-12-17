package chainnode

import (
	"bytes"
	"context"
	"fmt"

	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/cometbft"
)

func (r *Reconciler) createValidator(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	logger.Info("submitting create-validator tx")
	params := &chainutils.Params{
		ChainID:                 chainNode.Status.ChainID,
		StakeAmount:             chainNode.Spec.Validator.CreateValidator.StakeAmount,
		CommissionMaxChangeRate: chainNode.Spec.Validator.GetCommissionMaxChangeRate(),
		CommissionMaxRate:       chainNode.Spec.Validator.GetCommissionMaxRate(),
		CommissionRate:          chainNode.Spec.Validator.GetCommissionRate(),
		MinSelfDelegation:       chainNode.Spec.Validator.GetMinSelfDelegation(),
		GasPrices:               chainNode.Spec.Validator.CreateValidator.GasPrices,
	}

	accountSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: chainNode.GetNamespace(),
		Name:      chainNode.Spec.Validator.GetAccountSecretName(chainNode),
	}, accountSecret); err != nil {
		return err
	}

	account, err := chainutils.AccountFromMnemonic(
		string(accountSecret.Data[MnemonicKey]),
		chainNode.Spec.Validator.GetAccountPrefix(),
		chainNode.Spec.Validator.GetValPrefix(),
		chainNode.Spec.Validator.GetAccountHDPath(),
	)
	if err != nil {
		return err
	}

	// Gather validator info
	nodeInfo := &chainutils.NodeInfo{}
	nodeInfo.Moniker = chainNode.GetMoniker()
	if chainNode.Spec.Validator.Info != nil {
		nodeInfo.Details = chainNode.Spec.Validator.Info.Details
		nodeInfo.Website = chainNode.Spec.Validator.Info.Website
		nodeInfo.Identity = chainNode.Spec.Validator.Info.Identity
	}

	if err := app.CreateValidator(ctx,
		chainNode.Status.PubKey,
		account,
		nodeInfo,
		params,
		fmt.Sprintf("tcp://%s:%d", chainNode.GetNodeFQDN(), chainutils.RpcPort),
	); err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonCreateValidatorFailure,
			"failed to create-validator: %s", err.Error())
		return err
	}

	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonCreateValidatorSuccess,
		"successfully submited create-validator tx")
	return nil
}

func (r *Reconciler) updateValidatorStatus(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	client, err := r.getChainNodeClient(chainNode)
	if err != nil {
		return err
	}

	var validator *stakingTypes.Validator
	if chainNode.Status.ValidatorAddress == "" {
		status, err := client.GetNodeStatus(ctx)
		if err != nil {
			return err
		}

		validators, err := client.GetValidators(ctx)
		if err != nil {
			return err
		}

		found := false
		for _, val := range validators {
			pk, err := cometbft.UnpackPubKey(val.ConsensusPubkey)
			if err != nil {
				return err
			}
			if bytes.Equal(status.ValidatorInfo.PubKey.Address().Bytes(), pk.Address().Bytes()) {
				chainNode.Status.ValidatorAddress = val.OperatorAddress
				validator = &val
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("validator not found")
		}
	} else {
		validator, err = client.QueryValidator(ctx, chainNode.Status.ValidatorAddress)
		if err != nil {
			return err
		}
	}

	pk, err := cometbft.UnpackPubKey(validator.ConsensusPubkey)
	if err != nil {
		return err
	}

	pkStr, err := cometbft.PubKeyToString(pk)
	if err != nil {
		return err
	}

	accountAddr, err := chainutils.AccountAddressFromValidatorAddress(validator.OperatorAddress,
		chainNode.Spec.Validator.GetValPrefix(),
		chainNode.Spec.Validator.GetAccountPrefix(),
	)
	if err != nil {
		return err
	}

	validatorStatus := getValidatorStatus(validator.Status)

	if !chainNode.Status.Validator ||
		chainNode.Status.ValidatorAddress == "" ||
		chainNode.Status.ValidatorStatus != validatorStatus ||
		chainNode.Status.AccountAddress != accountAddr ||
		chainNode.Status.ValidatorAddress != validator.OperatorAddress ||
		chainNode.Status.Jailed != validator.Jailed ||
		chainNode.Status.PubKey != pkStr {
		if chainNode.Status.Jailed != validator.Jailed {
			logger.Info("updating jailed status", "jailed", validator.Jailed)

			if validator.Jailed {
				r.recorder.Eventf(chainNode,
					corev1.EventTypeWarning,
					appsv1.ReasonValidatorJailed,
					"Validator is jailed",
				)
			} else {
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonValidatorUnjailed,
					"Validator was successfully unjailed",
				)
			}
		}
		chainNode.Status.ValidatorAddress = validator.OperatorAddress
		chainNode.Status.AccountAddress = accountAddr
		chainNode.Status.Jailed = validator.Jailed
		chainNode.Status.ValidatorStatus = validatorStatus
		chainNode.Status.PubKey = pkStr
		chainNode.Status.Validator = true
		return r.Status().Update(ctx, chainNode)
	}

	return nil
}

func getValidatorStatus(status stakingTypes.BondStatus) appsv1.ValidatorStatus {
	switch status {
	case stakingTypes.Bonded:
		return appsv1.ValidatorStatusBonded
	case stakingTypes.Unbonding:
		return appsv1.ValidatorStatusUnbonding
	case stakingTypes.Unbonded:
		return appsv1.ValidatorStatusUnbonded
	case stakingTypes.Unspecified:
		fallthrough
	default:
		return appsv1.ValidatorStatusUnknown
	}
}
