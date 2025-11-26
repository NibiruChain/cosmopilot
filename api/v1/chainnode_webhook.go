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

func SetupChainNodeValidationWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&ChainNode{}).Complete()
}

var _ webhook.Validator = &ChainNode{}
var chainNodeLogger = log.Log.WithName("chainnode-webhook")

func (chainNode *ChainNode) ValidateCreate() (warnings admission.Warnings, err error) {
	chainNodeLogger.V(1).Info("validating resource creation",
		"kind", "ChainNode",
		"resource", chainNode.GetNamespacedName(),
	)
	return chainNode.Validate(nil)
}

func (chainNode *ChainNode) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {
	chainNodeLogger.V(1).Info("validating resource update",
		"kind", "ChainNode",
		"resource", chainNode.GetNamespacedName(),
	)
	return chainNode.Validate(old.(*ChainNode))
}

func (chainNode *ChainNode) ValidateDelete() (warnings admission.Warnings, err error) {
	chainNodeLogger.V(1).Info("validating resource deletion (not implemented)",
		"kind", "ChainNode",
		"resource", chainNode.GetNamespacedName(),
	)
	return nil, nil
}

func (chainNode *ChainNode) Validate(old *ChainNode) (admission.Warnings, error) {
	// Validate persistence size
	_, err := resource.ParseQuantity(chainNode.GetPersistenceSize())
	if err != nil {
		return nil, fmt.Errorf("bad format for .spec.size: %v", err)
	}

	// Ensure a genesis is specified when .spec.validator.init is not.
	if (chainNode.Spec.Validator == nil || chainNode.Spec.Validator.Init == nil) && chainNode.Spec.Genesis == nil {
		return nil, fmt.Errorf(".spec.genesis is required except when initializing new genesis with .spec.validator.init")
	}

	// Do not accept both genesis and validator init
	if chainNode.Spec.Validator != nil && chainNode.Spec.Validator.Init != nil && chainNode.Spec.Genesis != nil {
		return nil, fmt.Errorf(".spec.genesis and .spec.validator.init are mutually exclusive")
	}

	// Validate snapshots config
	if chainNode.Spec.Persistence != nil && chainNode.Spec.Persistence.Snapshots != nil {
		if err := validateSnapshotsConfig(chainNode.Spec.Persistence.Snapshots, ".spec.persistence.snapshots"); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func validateSnapshotsConfig(config *VolumeSnapshotsConfig, path string) error {
	if config.Retention != nil && config.Retain != nil {
		return fmt.Errorf("%s.retention and %s.retain are mutually exclusive", path, path)
	}
	return nil
}
