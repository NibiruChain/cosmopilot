package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func SetupChainNodeSetValidationWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ChainNodeSet{}).Complete()
}

var _ webhook.Validator = &ChainNodeSet{}
var chainNodeSetLogger = log.Log.WithName("chainnodeset-webhook")

func (nodeSet *ChainNodeSet) ValidateCreate() (warnings admission.Warnings, err error) {
	chainNodeSetLogger.V(1).Info("validating resource creation",
		"kind", "ChainNodeSet",
		"resource", nodeSet.GetNamespacedName(),
	)
	return nodeSet.Validate(nil)
}

func (nodeSet *ChainNodeSet) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {
	chainNodeSetLogger.V(1).Info("validating resource update",
		"kind", "ChainNodeSet",
		"resource", nodeSet.GetNamespacedName(),
	)
	return nodeSet.Validate(old.(*ChainNodeSet))
}

func (nodeSet *ChainNodeSet) ValidateDelete() (warnings admission.Warnings, err error) {
	chainNodeSetLogger.V(1).Info("validating resource deletion (not implemented)",
		"kind", "ChainNodeSet",
		"resource", nodeSet.GetNamespacedName(),
	)
	return nil, nil
}

func (nodeSet *ChainNodeSet) Validate(old *ChainNodeSet) (admission.Warnings, error) {
	// Ensure a genesis is specified when .spec.validator.init is not. Also, that only one genesis
	// retrieval method is used
	if nodeSet.Spec.Validator == nil || nodeSet.Spec.Validator.Init == nil {
		if nodeSet.Spec.Genesis == nil {
			return nil, fmt.Errorf(".spec.genesis is required except when initializing new genesis with .spec.validator.init")
		}

		counter := 0
		if nodeSet.Spec.Genesis.Url != nil {
			counter += 1
		}
		if nodeSet.Spec.Genesis.FromNodeRPC != nil {
			counter += 1
		}
		if nodeSet.Spec.Genesis.ConfigMap != nil {
			counter += 1
		}

		if counter == 0 {
			return nil, fmt.Errorf("a retrieval method must be specifyed on .spec.genesis")
		}

		if counter != 1 {
			return nil, fmt.Errorf("only one retrieval method must be specifyed on .spec.genesis")
		}

	}

	// Do not accept both genesis and validator init
	if nodeSet.Spec.Validator != nil && nodeSet.Spec.Validator.Init != nil && nodeSet.Spec.Genesis != nil {
		return nil, fmt.Errorf(".spec.genesis and .spec.validator.init are mutually exclusive")
	}

	// Validate each node group
	for i, group := range nodeSet.Spec.Nodes {
		// Validate persistence size
		if group.Persistence != nil && group.Persistence.Size != nil {
			_, err := resource.ParseQuantity(*group.Persistence.Size)
			if err != nil {
				return nil, fmt.Errorf("bad format for .spec.nodes[%d].persistence.size: %v", i, err)
			}
		}

	}

	return nil, nil
}
