//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"github.com/NibiruChain/nibiru-operator/internal/tmkms"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AccountAssets) DeepCopyInto(out *AccountAssets) {
	*out = *in
	if in.Assets != nil {
		in, out := &in.Assets, &out.Assets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AccountAssets.
func (in *AccountAssets) DeepCopy() *AccountAssets {
	if in == nil {
		return nil
	}
	out := new(AccountAssets)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppSpec) DeepCopyInto(out *AppSpec) {
	*out = *in
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	if in.SdkVersion != nil {
		in, out := &in.SdkVersion, &out.SdkVersion
		*out = new(SdkVersion)
		**out = **in
	}
	if in.CheckGovUpgrades != nil {
		in, out := &in.CheckGovUpgrades, &out.CheckGovUpgrades
		*out = new(bool)
		**out = **in
	}
	if in.Upgrades != nil {
		in, out := &in.Upgrades, &out.Upgrades
		*out = make([]UpgradeSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppSpec.
func (in *AppSpec) DeepCopy() *AppSpec {
	if in == nil {
		return nil
	}
	out := new(AppSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNode) DeepCopyInto(out *ChainNode) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNode.
func (in *ChainNode) DeepCopy() *ChainNode {
	if in == nil {
		return nil
	}
	out := new(ChainNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChainNode) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeList) DeepCopyInto(out *ChainNodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ChainNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeList.
func (in *ChainNodeList) DeepCopy() *ChainNodeList {
	if in == nil {
		return nil
	}
	out := new(ChainNodeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChainNodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSet) DeepCopyInto(out *ChainNodeSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSet.
func (in *ChainNodeSet) DeepCopy() *ChainNodeSet {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChainNodeSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSetList) DeepCopyInto(out *ChainNodeSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ChainNodeSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSetList.
func (in *ChainNodeSetList) DeepCopy() *ChainNodeSetList {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ChainNodeSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSetNodeStatus) DeepCopyInto(out *ChainNodeSetNodeStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSetNodeStatus.
func (in *ChainNodeSetNodeStatus) DeepCopy() *ChainNodeSetNodeStatus {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSetNodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSetSpec) DeepCopyInto(out *ChainNodeSetSpec) {
	*out = *in
	in.App.DeepCopyInto(&out.App)
	if in.Genesis != nil {
		in, out := &in.Genesis, &out.Genesis
		*out = new(GenesisConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Validator != nil {
		in, out := &in.Validator, &out.Validator
		*out = new(NodeSetValidatorConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]NodeGroupSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ServiceMonitor != nil {
		in, out := &in.ServiceMonitor, &out.ServiceMonitor
		*out = new(ServiceMonitorSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.RollingUpdates != nil {
		in, out := &in.RollingUpdates, &out.RollingUpdates
		*out = new(bool)
		**out = **in
	}
	if in.Ingresses != nil {
		in, out := &in.Ingresses, &out.Ingresses
		*out = make([]GlobalIngressConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSetSpec.
func (in *ChainNodeSetSpec) DeepCopy() *ChainNodeSetSpec {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSetStatus) DeepCopyInto(out *ChainNodeSetStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]ChainNodeSetNodeStatus, len(*in))
		copy(*out, *in)
	}
	if in.Upgrades != nil {
		in, out := &in.Upgrades, &out.Upgrades
		*out = make([]Upgrade, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSetStatus.
func (in *ChainNodeSetStatus) DeepCopy() *ChainNodeSetStatus {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeSpec) DeepCopyInto(out *ChainNodeSpec) {
	*out = *in
	if in.Genesis != nil {
		in, out := &in.Genesis, &out.Genesis
		*out = new(GenesisConfig)
		(*in).DeepCopyInto(*out)
	}
	in.App.DeepCopyInto(&out.App)
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(Config)
		(*in).DeepCopyInto(*out)
	}
	if in.Persistence != nil {
		in, out := &in.Persistence, &out.Persistence
		*out = new(Persistence)
		(*in).DeepCopyInto(*out)
	}
	if in.Validator != nil {
		in, out := &in.Validator, &out.Validator
		*out = new(ValidatorConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AutoDiscoverPeers != nil {
		in, out := &in.AutoDiscoverPeers, &out.AutoDiscoverPeers
		*out = new(bool)
		**out = **in
	}
	if in.StateSyncRestore != nil {
		in, out := &in.StateSyncRestore, &out.StateSyncRestore
		*out = new(bool)
		**out = **in
	}
	if in.Peers != nil {
		in, out := &in.Peers, &out.Peers
		*out = make([]Peer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Expose != nil {
		in, out := &in.Expose, &out.Expose
		*out = new(ExposeConfig)
		(*in).DeepCopyInto(*out)
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeSpec.
func (in *ChainNodeSpec) DeepCopy() *ChainNodeSpec {
	if in == nil {
		return nil
	}
	out := new(ChainNodeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChainNodeStatus) DeepCopyInto(out *ChainNodeStatus) {
	*out = *in
	if in.Upgrades != nil {
		in, out := &in.Upgrades, &out.Upgrades
		*out = make([]Upgrade, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChainNodeStatus.
func (in *ChainNodeStatus) DeepCopy() *ChainNodeStatus {
	if in == nil {
		return nil
	}
	out := new(ChainNodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Config) DeepCopyInto(out *Config) {
	*out = *in
	if in.Override != nil {
		in, out := &in.Override, &out.Override
		*out = new(map[string]runtime.RawExtension)
		if **in != nil {
			in, out := *in, *out
			*out = make(map[string]runtime.RawExtension, len(*in))
			for key, val := range *in {
				(*out)[key] = *val.DeepCopy()
			}
		}
	}
	if in.Sidecars != nil {
		in, out := &in.Sidecars, &out.Sidecars
		*out = make([]SidecarSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.BlockThreshold != nil {
		in, out := &in.BlockThreshold, &out.BlockThreshold
		*out = new(string)
		**out = **in
	}
	if in.ReconcilePeriod != nil {
		in, out := &in.ReconcilePeriod, &out.ReconcilePeriod
		*out = new(string)
		**out = **in
	}
	if in.StateSync != nil {
		in, out := &in.StateSync, &out.StateSync
		*out = new(StateSyncConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.SeedMode != nil {
		in, out := &in.SeedMode, &out.SeedMode
		*out = new(bool)
		**out = **in
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SafeToEvict != nil {
		in, out := &in.SafeToEvict, &out.SafeToEvict
		*out = new(bool)
		**out = **in
	}
	if in.ServiceMonitor != nil {
		in, out := &in.ServiceMonitor, &out.ServiceMonitor
		*out = new(ServiceMonitorSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Firewall != nil {
		in, out := &in.Firewall, &out.Firewall
		*out = new(FirewallConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeUtilsLogLevel != nil {
		in, out := &in.NodeUtilsLogLevel, &out.NodeUtilsLogLevel
		*out = new(string)
		**out = **in
	}
	if in.StartupTime != nil {
		in, out := &in.StartupTime, &out.StartupTime
		*out = new(string)
		**out = **in
	}
	if in.IgnoreSyncing != nil {
		in, out := &in.IgnoreSyncing, &out.IgnoreSyncing
		*out = new(bool)
		**out = **in
	}
	if in.NodeUtilsResources != nil {
		in, out := &in.NodeUtilsResources, &out.NodeUtilsResources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.PersistAddressBook != nil {
		in, out := &in.PersistAddressBook, &out.PersistAddressBook
		*out = new(bool)
		**out = **in
	}
	if in.TerminationGracePeriodSeconds != nil {
		in, out := &in.TerminationGracePeriodSeconds, &out.TerminationGracePeriodSeconds
		*out = new(int64)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Config.
func (in *Config) DeepCopy() *Config {
	if in == nil {
		return nil
	}
	out := new(Config)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CreateValidatorConfig) DeepCopyInto(out *CreateValidatorConfig) {
	*out = *in
	if in.AccountMnemonicSecret != nil {
		in, out := &in.AccountMnemonicSecret, &out.AccountMnemonicSecret
		*out = new(string)
		**out = **in
	}
	if in.AccountHDPath != nil {
		in, out := &in.AccountHDPath, &out.AccountHDPath
		*out = new(string)
		**out = **in
	}
	if in.AccountPrefix != nil {
		in, out := &in.AccountPrefix, &out.AccountPrefix
		*out = new(string)
		**out = **in
	}
	if in.ValPrefix != nil {
		in, out := &in.ValPrefix, &out.ValPrefix
		*out = new(string)
		**out = **in
	}
	if in.CommissionMaxChangeRate != nil {
		in, out := &in.CommissionMaxChangeRate, &out.CommissionMaxChangeRate
		*out = new(string)
		**out = **in
	}
	if in.CommissionMaxRate != nil {
		in, out := &in.CommissionMaxRate, &out.CommissionMaxRate
		*out = new(string)
		**out = **in
	}
	if in.CommissionRate != nil {
		in, out := &in.CommissionRate, &out.CommissionRate
		*out = new(string)
		**out = **in
	}
	if in.MinSelfDelegation != nil {
		in, out := &in.MinSelfDelegation, &out.MinSelfDelegation
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CreateValidatorConfig.
func (in *CreateValidatorConfig) DeepCopy() *CreateValidatorConfig {
	if in == nil {
		return nil
	}
	out := new(CreateValidatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExportTarballConfig) DeepCopyInto(out *ExportTarballConfig) {
	*out = *in
	if in.Suffix != nil {
		in, out := &in.Suffix, &out.Suffix
		*out = new(string)
		**out = **in
	}
	if in.DeleteOnExpire != nil {
		in, out := &in.DeleteOnExpire, &out.DeleteOnExpire
		*out = new(bool)
		**out = **in
	}
	if in.GCS != nil {
		in, out := &in.GCS, &out.GCS
		*out = new(GcsExportConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExportTarballConfig.
func (in *ExportTarballConfig) DeepCopy() *ExportTarballConfig {
	if in == nil {
		return nil
	}
	out := new(ExportTarballConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExposeConfig) DeepCopyInto(out *ExposeConfig) {
	*out = *in
	if in.P2P != nil {
		in, out := &in.P2P, &out.P2P
		*out = new(bool)
		**out = **in
	}
	if in.P2pServiceType != nil {
		in, out := &in.P2pServiceType, &out.P2pServiceType
		*out = new(corev1.ServiceType)
		**out = **in
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExposeConfig.
func (in *ExposeConfig) DeepCopy() *ExposeConfig {
	if in == nil {
		return nil
	}
	out := new(ExposeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallConfig) DeepCopyInto(out *FirewallConfig) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(corev1.ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.RestartPodOnFailure != nil {
		in, out := &in.RestartPodOnFailure, &out.RestartPodOnFailure
		*out = new(bool)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallConfig.
func (in *FirewallConfig) DeepCopy() *FirewallConfig {
	if in == nil {
		return nil
	}
	out := new(FirewallConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FromNodeRPCConfig) DeepCopyInto(out *FromNodeRPCConfig) {
	*out = *in
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FromNodeRPCConfig.
func (in *FromNodeRPCConfig) DeepCopy() *FromNodeRPCConfig {
	if in == nil {
		return nil
	}
	out := new(FromNodeRPCConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GcsExportConfig) DeepCopyInto(out *GcsExportConfig) {
	*out = *in
	if in.CredentialsSecret != nil {
		in, out := &in.CredentialsSecret, &out.CredentialsSecret
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GcsExportConfig.
func (in *GcsExportConfig) DeepCopy() *GcsExportConfig {
	if in == nil {
		return nil
	}
	out := new(GcsExportConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenesisConfig) DeepCopyInto(out *GenesisConfig) {
	*out = *in
	if in.Url != nil {
		in, out := &in.Url, &out.Url
		*out = new(string)
		**out = **in
	}
	if in.FromNodeRPC != nil {
		in, out := &in.FromNodeRPC, &out.FromNodeRPC
		*out = new(FromNodeRPCConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.GenesisSHA != nil {
		in, out := &in.GenesisSHA, &out.GenesisSHA
		*out = new(string)
		**out = **in
	}
	if in.ConfigMap != nil {
		in, out := &in.ConfigMap, &out.ConfigMap
		*out = new(string)
		**out = **in
	}
	if in.UseDataVolume != nil {
		in, out := &in.UseDataVolume, &out.UseDataVolume
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenesisConfig.
func (in *GenesisConfig) DeepCopy() *GenesisConfig {
	if in == nil {
		return nil
	}
	out := new(GenesisConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenesisInitConfig) DeepCopyInto(out *GenesisInitConfig) {
	*out = *in
	if in.AccountMnemonicSecret != nil {
		in, out := &in.AccountMnemonicSecret, &out.AccountMnemonicSecret
		*out = new(string)
		**out = **in
	}
	if in.AccountHDPath != nil {
		in, out := &in.AccountHDPath, &out.AccountHDPath
		*out = new(string)
		**out = **in
	}
	if in.AccountPrefix != nil {
		in, out := &in.AccountPrefix, &out.AccountPrefix
		*out = new(string)
		**out = **in
	}
	if in.ValPrefix != nil {
		in, out := &in.ValPrefix, &out.ValPrefix
		*out = new(string)
		**out = **in
	}
	if in.CommissionMaxChangeRate != nil {
		in, out := &in.CommissionMaxChangeRate, &out.CommissionMaxChangeRate
		*out = new(string)
		**out = **in
	}
	if in.CommissionMaxRate != nil {
		in, out := &in.CommissionMaxRate, &out.CommissionMaxRate
		*out = new(string)
		**out = **in
	}
	if in.CommissionRate != nil {
		in, out := &in.CommissionRate, &out.CommissionRate
		*out = new(string)
		**out = **in
	}
	if in.MinSelfDelegation != nil {
		in, out := &in.MinSelfDelegation, &out.MinSelfDelegation
		*out = new(string)
		**out = **in
	}
	if in.Assets != nil {
		in, out := &in.Assets, &out.Assets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Accounts != nil {
		in, out := &in.Accounts, &out.Accounts
		*out = make([]AccountAssets, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.UnbondingTime != nil {
		in, out := &in.UnbondingTime, &out.UnbondingTime
		*out = new(string)
		**out = **in
	}
	if in.VotingPeriod != nil {
		in, out := &in.VotingPeriod, &out.VotingPeriod
		*out = new(string)
		**out = **in
	}
	if in.AdditionalInitCommands != nil {
		in, out := &in.AdditionalInitCommands, &out.AdditionalInitCommands
		*out = make([]InitCommand, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenesisInitConfig.
func (in *GenesisInitConfig) DeepCopy() *GenesisInitConfig {
	if in == nil {
		return nil
	}
	out := new(GenesisInitConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalIngressConfig) DeepCopyInto(out *GlobalIngressConfig) {
	*out = *in
	if in.Groups != nil {
		in, out := &in.Groups, &out.Groups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.TlsSecretName != nil {
		in, out := &in.TlsSecretName, &out.TlsSecretName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalIngressConfig.
func (in *GlobalIngressConfig) DeepCopy() *GlobalIngressConfig {
	if in == nil {
		return nil
	}
	out := new(GlobalIngressConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressConfig) DeepCopyInto(out *IngressConfig) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.TlsSecretName != nil {
		in, out := &in.TlsSecretName, &out.TlsSecretName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressConfig.
func (in *IngressConfig) DeepCopy() *IngressConfig {
	if in == nil {
		return nil
	}
	out := new(IngressConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InitCommand) DeepCopyInto(out *InitCommand) {
	*out = *in
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InitCommand.
func (in *InitCommand) DeepCopy() *InitCommand {
	if in == nil {
		return nil
	}
	out := new(InitCommand)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeGroupSpec) DeepCopyInto(out *NodeGroupSpec) {
	*out = *in
	if in.Instances != nil {
		in, out := &in.Instances, &out.Instances
		*out = new(int)
		**out = **in
	}
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(Config)
		(*in).DeepCopyInto(*out)
	}
	if in.Persistence != nil {
		in, out := &in.Persistence, &out.Persistence
		*out = new(Persistence)
		(*in).DeepCopyInto(*out)
	}
	if in.Peers != nil {
		in, out := &in.Peers, &out.Peers
		*out = make([]Peer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Expose != nil {
		in, out := &in.Expose, &out.Expose
		*out = new(ExposeConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Ingress != nil {
		in, out := &in.Ingress, &out.Ingress
		*out = new(IngressConfig)
		(*in).DeepCopyInto(*out)
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.StateSyncRestore != nil {
		in, out := &in.StateSyncRestore, &out.StateSyncRestore
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeGroupSpec.
func (in *NodeGroupSpec) DeepCopy() *NodeGroupSpec {
	if in == nil {
		return nil
	}
	out := new(NodeGroupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeSetValidatorConfig) DeepCopyInto(out *NodeSetValidatorConfig) {
	*out = *in
	if in.PrivateKeySecret != nil {
		in, out := &in.PrivateKeySecret, &out.PrivateKeySecret
		*out = new(string)
		**out = **in
	}
	if in.Info != nil {
		in, out := &in.Info, &out.Info
		*out = new(ValidatorInfo)
		(*in).DeepCopyInto(*out)
	}
	if in.Init != nil {
		in, out := &in.Init, &out.Init
		*out = new(GenesisInitConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(Config)
		(*in).DeepCopyInto(*out)
	}
	if in.Persistence != nil {
		in, out := &in.Persistence, &out.Persistence
		*out = new(Persistence)
		(*in).DeepCopyInto(*out)
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.TmKMS != nil {
		in, out := &in.TmKMS, &out.TmKMS
		*out = new(TmKMS)
		(*in).DeepCopyInto(*out)
	}
	if in.StateSyncRestore != nil {
		in, out := &in.StateSyncRestore, &out.StateSyncRestore
		*out = new(bool)
		**out = **in
	}
	if in.CreateValidator != nil {
		in, out := &in.CreateValidator, &out.CreateValidator
		*out = new(CreateValidatorConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeSetValidatorConfig.
func (in *NodeSetValidatorConfig) DeepCopy() *NodeSetValidatorConfig {
	if in == nil {
		return nil
	}
	out := new(NodeSetValidatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Peer) DeepCopyInto(out *Peer) {
	*out = *in
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int)
		**out = **in
	}
	if in.Unconditional != nil {
		in, out := &in.Unconditional, &out.Unconditional
		*out = new(bool)
		**out = **in
	}
	if in.Private != nil {
		in, out := &in.Private, &out.Private
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Peer.
func (in *Peer) DeepCopy() *Peer {
	if in == nil {
		return nil
	}
	out := new(Peer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Persistence) DeepCopyInto(out *Persistence) {
	*out = *in
	if in.Size != nil {
		in, out := &in.Size, &out.Size
		*out = new(string)
		**out = **in
	}
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
	if in.AutoResize != nil {
		in, out := &in.AutoResize, &out.AutoResize
		*out = new(bool)
		**out = **in
	}
	if in.AutoResizeThreshold != nil {
		in, out := &in.AutoResizeThreshold, &out.AutoResizeThreshold
		*out = new(int)
		**out = **in
	}
	if in.AutoResizeIncrement != nil {
		in, out := &in.AutoResizeIncrement, &out.AutoResizeIncrement
		*out = new(string)
		**out = **in
	}
	if in.AutoResizeMaxSize != nil {
		in, out := &in.AutoResizeMaxSize, &out.AutoResizeMaxSize
		*out = new(string)
		**out = **in
	}
	if in.AdditionalInitCommands != nil {
		in, out := &in.AdditionalInitCommands, &out.AdditionalInitCommands
		*out = make([]InitCommand, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Snapshots != nil {
		in, out := &in.Snapshots, &out.Snapshots
		*out = new(VolumeSnapshotsConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.RestoreFromSnapshot != nil {
		in, out := &in.RestoreFromSnapshot, &out.RestoreFromSnapshot
		*out = new(PvcSnapshot)
		**out = **in
	}
	if in.InitTimeout != nil {
		in, out := &in.InitTimeout, &out.InitTimeout
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Persistence.
func (in *Persistence) DeepCopy() *Persistence {
	if in == nil {
		return nil
	}
	out := new(Persistence)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PvcSnapshot) DeepCopyInto(out *PvcSnapshot) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PvcSnapshot.
func (in *PvcSnapshot) DeepCopy() *PvcSnapshot {
	if in == nil {
		return nil
	}
	out := new(PvcSnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceMonitorSpec) DeepCopyInto(out *ServiceMonitorSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceMonitorSpec.
func (in *ServiceMonitorSpec) DeepCopy() *ServiceMonitorSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceMonitorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SidecarSpec) DeepCopyInto(out *SidecarSpec) {
	*out = *in
	if in.MountDataVolume != nil {
		in, out := &in.MountDataVolume, &out.MountDataVolume
		*out = new(string)
		**out = **in
	}
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(corev1.SecurityContext)
		(*in).DeepCopyInto(*out)
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.RestartPodOnFailure != nil {
		in, out := &in.RestartPodOnFailure, &out.RestartPodOnFailure
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SidecarSpec.
func (in *SidecarSpec) DeepCopy() *SidecarSpec {
	if in == nil {
		return nil
	}
	out := new(SidecarSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StateSyncConfig) DeepCopyInto(out *StateSyncConfig) {
	*out = *in
	if in.SnapshotKeepRecent != nil {
		in, out := &in.SnapshotKeepRecent, &out.SnapshotKeepRecent
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StateSyncConfig.
func (in *StateSyncConfig) DeepCopy() *StateSyncConfig {
	if in == nil {
		return nil
	}
	out := new(StateSyncConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TmKMS) DeepCopyInto(out *TmKMS) {
	*out = *in
	in.Provider.DeepCopyInto(&out.Provider)
	if in.KeyFormat != nil {
		in, out := &in.KeyFormat, &out.KeyFormat
		*out = new(TmKmsKeyFormat)
		**out = **in
	}
	if in.ValidatorProtocol != nil {
		in, out := &in.ValidatorProtocol, &out.ValidatorProtocol
		*out = new(tmkms.ProtocolVersion)
		**out = **in
	}
	if in.PersistState != nil {
		in, out := &in.PersistState, &out.PersistState
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TmKMS.
func (in *TmKMS) DeepCopy() *TmKMS {
	if in == nil {
		return nil
	}
	out := new(TmKMS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TmKmsKeyFormat) DeepCopyInto(out *TmKmsKeyFormat) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TmKmsKeyFormat.
func (in *TmKmsKeyFormat) DeepCopy() *TmKmsKeyFormat {
	if in == nil {
		return nil
	}
	out := new(TmKmsKeyFormat)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TmKmsProvider) DeepCopyInto(out *TmKmsProvider) {
	*out = *in
	if in.Vault != nil {
		in, out := &in.Vault, &out.Vault
		*out = new(TmKmsVaultProvider)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TmKmsProvider.
func (in *TmKmsProvider) DeepCopy() *TmKmsProvider {
	if in == nil {
		return nil
	}
	out := new(TmKmsProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TmKmsVaultProvider) DeepCopyInto(out *TmKmsVaultProvider) {
	*out = *in
	if in.CertificateSecret != nil {
		in, out := &in.CertificateSecret, &out.CertificateSecret
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.TokenSecret != nil {
		in, out := &in.TokenSecret, &out.TokenSecret
		*out = new(corev1.SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TmKmsVaultProvider.
func (in *TmKmsVaultProvider) DeepCopy() *TmKmsVaultProvider {
	if in == nil {
		return nil
	}
	out := new(TmKmsVaultProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Upgrade) DeepCopyInto(out *Upgrade) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Upgrade.
func (in *Upgrade) DeepCopy() *Upgrade {
	if in == nil {
		return nil
	}
	out := new(Upgrade)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeSpec) DeepCopyInto(out *UpgradeSpec) {
	*out = *in
	if in.ForceOnChain != nil {
		in, out := &in.ForceOnChain, &out.ForceOnChain
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeSpec.
func (in *UpgradeSpec) DeepCopy() *UpgradeSpec {
	if in == nil {
		return nil
	}
	out := new(UpgradeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidatorConfig) DeepCopyInto(out *ValidatorConfig) {
	*out = *in
	if in.PrivateKeySecret != nil {
		in, out := &in.PrivateKeySecret, &out.PrivateKeySecret
		*out = new(string)
		**out = **in
	}
	if in.Info != nil {
		in, out := &in.Info, &out.Info
		*out = new(ValidatorInfo)
		(*in).DeepCopyInto(*out)
	}
	if in.Init != nil {
		in, out := &in.Init, &out.Init
		*out = new(GenesisInitConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.TmKMS != nil {
		in, out := &in.TmKMS, &out.TmKMS
		*out = new(TmKMS)
		(*in).DeepCopyInto(*out)
	}
	if in.CreateValidator != nil {
		in, out := &in.CreateValidator, &out.CreateValidator
		*out = new(CreateValidatorConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidatorConfig.
func (in *ValidatorConfig) DeepCopy() *ValidatorConfig {
	if in == nil {
		return nil
	}
	out := new(ValidatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidatorInfo) DeepCopyInto(out *ValidatorInfo) {
	*out = *in
	if in.Moniker != nil {
		in, out := &in.Moniker, &out.Moniker
		*out = new(string)
		**out = **in
	}
	if in.Details != nil {
		in, out := &in.Details, &out.Details
		*out = new(string)
		**out = **in
	}
	if in.Website != nil {
		in, out := &in.Website, &out.Website
		*out = new(string)
		**out = **in
	}
	if in.Identity != nil {
		in, out := &in.Identity, &out.Identity
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidatorInfo.
func (in *ValidatorInfo) DeepCopy() *ValidatorInfo {
	if in == nil {
		return nil
	}
	out := new(ValidatorInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotsConfig) DeepCopyInto(out *VolumeSnapshotsConfig) {
	*out = *in
	if in.Retention != nil {
		in, out := &in.Retention, &out.Retention
		*out = new(string)
		**out = **in
	}
	if in.SnapshotClassName != nil {
		in, out := &in.SnapshotClassName, &out.SnapshotClassName
		*out = new(string)
		**out = **in
	}
	if in.StopNode != nil {
		in, out := &in.StopNode, &out.StopNode
		*out = new(bool)
		**out = **in
	}
	if in.ExportTarball != nil {
		in, out := &in.ExportTarball, &out.ExportTarball
		*out = new(ExportTarballConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Verify != nil {
		in, out := &in.Verify, &out.Verify
		*out = new(bool)
		**out = **in
	}
	if in.DisableWhileSyncing != nil {
		in, out := &in.DisableWhileSyncing, &out.DisableWhileSyncing
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotsConfig.
func (in *VolumeSnapshotsConfig) DeepCopy() *VolumeSnapshotsConfig {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotsConfig)
	in.DeepCopyInto(out)
	return out
}
