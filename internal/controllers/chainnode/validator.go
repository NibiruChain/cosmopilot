package chainnode

import (
	"context"

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
		return r.Status().Update(ctx, chainNode)
	}

	return nil
}
