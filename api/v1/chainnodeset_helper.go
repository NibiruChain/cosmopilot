package v1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultGroupInstances = 1
)

func (nodeSet *ChainNodeSet) HasValidator() bool {
	return nodeSet.Spec.Validator != nil
}

func (nodeSet *ChainNodeSet) ShouldInitGenesis() bool {
	return nodeSet.Spec.Validator != nil && nodeSet.Spec.Validator.Init != nil
}

// Node group methods

func (group *NodeGroupSpec) GetInstances() int {
	if group.Instances != nil {
		return *group.Instances
	}
	return DefaultGroupInstances
}

func (group *NodeGroupSpec) GetIngressSecretName(owner client.Object) string {
	return fmt.Sprintf("%s-%s-tls", owner.GetName(), group.Name)
}

func (group *NodeGroupSpec) GetServiceName(owner client.Object) string {
	return fmt.Sprintf("%s-%s", owner.GetName(), group.Name)
}
