package chainnode

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

func (r *Reconciler) updateJailedStatus(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	client, err := r.getQueryClient(chainNode)
	if err != nil {
		return err
	}

	validator, err := client.QueryValidator(ctx, chainNode.Status.ValidatorAddress)
	if err != nil {
		return err
	}

	if chainNode.Status.Jailed != validator.Jailed {
		logger.Info("updating jailed status", "jailed", validator.Jailed)
		chainNode.Status.Jailed = validator.Jailed
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
		return r.Status().Update(ctx, chainNode)
	}

	return nil
}

func (r *Reconciler) updateLatestHeight(ctx context.Context, chainNode *appsv1.ChainNode) error {
	client, err := r.getQueryClient(chainNode)
	if err != nil {
		return err
	}

	block, err := client.GetLatestBlock(ctx)
	if err != nil {
		return err
	}

	chainNode.Status.LatestHeight = block.Header.Height
	return r.Status().Update(ctx, chainNode)
}
