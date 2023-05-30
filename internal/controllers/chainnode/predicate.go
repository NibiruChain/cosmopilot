package chainnode

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

type GenerationChangedPredicate struct {
	predicate.Funcs
}

// Create implements default CreateEvent filter
func (p GenerationChangedPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	// Ignore updates on config-generator pod events
	if strings.HasSuffix(e.Object.(metav1.Object).GetName(), "config-generator") {
		return false
	}

	// Ignore updates on data-init pod events
	if strings.HasSuffix(e.Object.(metav1.Object).GetName(), "data-init") {
		return false
	}

	return p.Funcs.Create(e)
}

// Delete implements default DeleteEvent filter
func (p GenerationChangedPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}

	// Ignore updates on config-generator pod events
	if strings.HasSuffix(e.Object.(metav1.Object).GetName(), "config-generator") {
		return false
	}

	// Ignore updates on data-init pod events
	if strings.HasSuffix(e.Object.(metav1.Object).GetName(), "data-init") {
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

	switch o := e.ObjectNew.(type) {
	case *appsv1.ChainNode:
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()

	case *corev1.Pod:
		// Ignore updates on config-generator pod events
		if strings.HasSuffix(o.Name, "config-generator") {
			return false
		}

		// Ignore updates on data-init pod events
		if strings.HasSuffix(o.Name, "data-init") {
			return false
		}

		return p.Funcs.Update(e)

	default:
		return p.Funcs.Update(e)
	}
}
