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

// Validator methods

func (val *NodeSetValidatorConfig) GetPrivKeySecretName(obj client.Object) string {
	if val.PrivateKeySecret != nil {
		return *val.PrivateKeySecret
	}
	return fmt.Sprintf("%s-priv-key", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountHDPath() string {
	if val.Init != nil && val.Init.AccountHDPath != nil {
		return *val.Init.AccountHDPath
	}
	return DefaultHDPath
}

func (val *NodeSetValidatorConfig) GetAccountSecretName(obj client.Object) string {
	if val.Init != nil && val.Init.AccountMnemonicSecret != nil {
		return *val.Init.AccountMnemonicSecret
	}

	return fmt.Sprintf("%s-account", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountPrefix() string {
	if val.Init != nil && val.Init.AccountPrefix != nil {
		return *val.Init.AccountPrefix
	}
	return DefaultAccountPrefix
}

func (val *NodeSetValidatorConfig) GetValPrefix() string {
	if val.Init != nil && val.Init.ValPrefix != nil {
		return *val.Init.ValPrefix
	}
	return DefaultValPrefix
}

func (val *NodeSetValidatorConfig) GetInitUnbondingTime() string {
	if val.Init != nil && val.Init.UnbondingTime != nil {
		return *val.Init.UnbondingTime
	}
	return DefaultUnbondingTime
}

func (val *NodeSetValidatorConfig) GetInitVotingPeriod() string {
	if val.Init != nil && val.Init.VotingPeriod != nil {
		return *val.Init.VotingPeriod
	}
	return DefaultVotingPeriod
}
