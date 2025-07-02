package chainnodeset

import (
	"context"
	"fmt"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

func (r *Reconciler) EnsurePodDisruptionBudgets(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	if nodeSet.Spec.Validator.HasPdbEnabled() {
		pdb := getPdbSpec(
			nodeSet,
			fmt.Sprintf("%s-validator", nodeSet.GetName()),
			nodeSet.Spec.Validator.GetPdbMinAvailable(),
			map[string]string{
				LabelChainID:               nodeSet.Status.ChainID,
				LabelChainNodeSetValidator: StringValueTrue,
			},
		)
		if err := r.ensurePodDisruptionBudget(ctx, pdb); err != nil {
			return err
		}
	} else {
		if err := r.maybeDeletePDB(ctx, fmt.Sprintf("%s-validator", nodeSet.GetName()), nodeSet.GetNamespace()); err != nil {
			return err
		}
	}

	for _, group := range nodeSet.Spec.Nodes {
		if group.HasPdbEnabled() {
			pdb := getPdbSpec(
				nodeSet,
				group.GetServiceName(nodeSet),
				group.GetPdbMinAvailable(),
				map[string]string{
					LabelChainNodeSet:      nodeSet.GetName(),
					LabelChainNodeSetGroup: group.Name,
				},
			)
			if err := r.ensurePodDisruptionBudget(ctx, pdb); err != nil {
				return err
			}
		} else {
			if err := r.maybeDeletePDB(ctx, group.GetServiceName(nodeSet), nodeSet.GetNamespace()); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPdbSpec(nodeSet *appsv1.ChainNodeSet, name string, min int, labels map[string]string) *policyv1.PodDisruptionBudget {
	minAvailable := intstr.FromInt32(int32(min))
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodeSet.GetNamespace(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
}

func (r *Reconciler) ensurePodDisruptionBudget(ctx context.Context, pdb *policyv1.PodDisruptionBudget) error {
	logger := log.FromContext(ctx)

	currentPdb := &policyv1.PodDisruptionBudget{}
	err := r.Get(ctx, client.ObjectKeyFromObject(pdb), currentPdb)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pod disruption budget", "pdb", pdb.GetName())
			return r.Create(ctx, pdb)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(currentPdb, pdb)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		logger.Info("updating pod disruption budget", "pdb", pdb.GetName())

		pdb.ObjectMeta.ResourceVersion = currentPdb.ObjectMeta.ResourceVersion
		if err := r.Update(ctx, pdb); err != nil {
			return err
		}
	}

	*pdb = *currentPdb
	return nil
}

func (r *Reconciler) maybeDeletePDB(ctx context.Context, name, namespace string) error {
	logger := log.FromContext(ctx)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.Delete(ctx, pdb)

	if err == nil {
		logger.Info("deleted pod disruption budget", "pdb", pdb.GetName())
		return nil
	}
	if errors.IsNotFound(err) {
		return nil
	}

	return err
}
