package chainnodeset

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

func (r *Reconciler) ensureValidator(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	if !nodeSet.HasValidator() {
		// If there was a validator before lets delete it
		return r.maybeDeleteNode(ctx, nodeSet, fmt.Sprintf("%s-validator", nodeSet.GetName()))
	}

	validator, err := r.getValidatorSpec(nodeSet)
	if err != nil {
		return err
	}

	if err := r.ensureNode(ctx, nodeSet, validator, true); err != nil {
		return err
	}

	nodeSetCopy := nodeSet.DeepCopy()
	AddOrUpdateNodeStatus(nodeSet, appsv1.ChainNodeSetNodeStatus{
		Name:    validator.Name,
		ID:      validator.Status.NodeID,
		Address: validator.Status.IP,
		Port:    chainutils.P2pPort,
		Seed:    validator.Status.SeedMode,
		Public:  false,
		Group:   validatorGroupName,
	})

	if !reflect.DeepEqual(nodeSet.Status, nodeSetCopy.Status) {
		logger.Info("updating .status fields",
			"chainID", validator.Status.ChainID,
			"validatorAddress", validator.Status.ValidatorAddress,
			"validatorStatus", validator.Status.ValidatorStatus,
			"pubKey", validator.Status.PubKey,
		)
		nodeSet.Status.ChainID = validator.Status.ChainID
		nodeSet.Status.ValidatorAddress = validator.Status.ValidatorAddress
		nodeSet.Status.ValidatorStatus = validator.Status.ValidatorStatus
		nodeSet.Status.PubKey = validator.Status.PubKey
		return r.Status().Update(ctx, nodeSet)
	}
	return nil
}

func (r *Reconciler) getValidatorSpec(nodeSet *appsv1.ChainNodeSet) (*appsv1.ChainNode, error) {
	validator := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-validator", nodeSet.GetName()),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				LabelChainNodeSet:          nodeSet.GetName(),
				LabelChainNodeSetGroup:     validatorGroupName,
				LabelChainNodeSetValidator: strconv.FormatBool(true),
			}),
		},
		Spec: appsv1.ChainNodeSpec{
			Genesis:     nodeSet.Spec.Genesis,
			App:         nodeSet.GetAppSpecWithUpgrades(),
			Config:      nodeSet.Spec.Validator.Config,
			Persistence: nodeSet.Spec.Validator.Persistence,
			Validator: &appsv1.ValidatorConfig{
				PrivateKeySecret: nodeSet.Spec.Validator.PrivateKeySecret,
				Info:             nodeSet.Spec.Validator.Info,
				Init:             nodeSet.Spec.Validator.Init,
				TmKMS:            nodeSet.Spec.Validator.TmKMS,
				CreateValidator:  nodeSet.Spec.Validator.CreateValidator,
			},
			Resources:        nodeSet.Spec.Validator.Resources,
			Affinity:         nodeSet.Spec.Validator.Affinity,
			NodeSelector:     nodeSet.Spec.Validator.NodeSelector,
			StateSyncRestore: nodeSet.Spec.Validator.StateSyncRestore,
		},
	}
	setChainNodeServiceMonitor(nodeSet, validator)
	return validator, controllerutil.SetControllerReference(nodeSet, validator, r.Scheme)
}
