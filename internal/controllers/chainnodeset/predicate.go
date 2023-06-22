package chainnodeset

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

type GenerationChangedPredicate struct {
	predicate.Funcs
}

var ignoreSuffixes = []string{"config-generator", "data-init", "genesis-init"}

// Create implements default CreateEvent filter
func (p GenerationChangedPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}
	return p.Funcs.Create(e)
}

// Delete implements default DeleteEvent filter
func (p GenerationChangedPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}
	return p.Funcs.Delete(e)
}

// Update implements default UpdateEvent filter for validating generation change
func (p GenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	switch e.ObjectNew.(type) {
	case *appsv1.ChainNodeSet:
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()

	default:
		return p.Funcs.Update(e)
	}
}
