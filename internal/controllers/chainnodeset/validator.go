package chainnodeset

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

func (r *Reconciler) ensureValidator(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	validator, err := r.getValidatorSpec(nodeSet)
	if err != nil {
		return err
	}

	if err := r.ensureNode(ctx, nodeSet, validator); err != nil {
		return err
	}

	if nodeSet.Status.ChainID != validator.Status.ChainID {
		nodeSet.Status.ChainID = validator.Status.ChainID
		return r.Status().Update(ctx, nodeSet)
	}
	return nil
}

func (r *Reconciler) getValidatorSpec(nodeSet *appsv1.ChainNodeSet) (*appsv1.ChainNode, error) {
	validator := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-validator", nodeSet.GetName()),
			Namespace: nodeSet.GetNamespace(),
			Labels: map[string]string{
				labelChainNodeSet:          nodeSet.GetName(),
				labelChainNodeSetValidator: strconv.FormatBool(true),
			},
		},
		Spec: appsv1.ChainNodeSpec{
			Genesis:     nodeSet.Spec.Genesis,
			App:         nodeSet.Spec.App,
			Config:      nodeSet.Spec.Validator.Config,
			Persistence: nodeSet.Spec.Validator.Persistence,
			Validator:   nodeSet.Spec.Validator,
		},
	}
	return validator, controllerutil.SetControllerReference(nodeSet, validator, r.Scheme)
}
