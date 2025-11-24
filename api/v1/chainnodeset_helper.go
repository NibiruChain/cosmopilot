package v1

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NibiruChain/cosmoseed/pkg/cosmoseed"
	"github.com/goccy/go-json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

const (
	// DefaultGroupInstances is the default number of nodes in a group.
	DefaultGroupInstances = 1

	// DefaultCosmoseedLogLevel is the log level used by Cosmoseed when none is specified.
	DefaultCosmoseedLogLevel = "info"

	// DefaultCosmoseedAddrBookFile is the path to the Cosmoseed address book file.
	DefaultCosmoseedAddrBookFile = "data/addrbook.json"

	// DefaultIngressClass is the default ingress class name.
	DefaultIngressClass = "nginx"
)

func (nodeSet *ChainNodeSet) GetNamespacedName() string {
	return types.NamespacedName{Namespace: nodeSet.GetNamespace(), Name: nodeSet.GetName()}.String()
}

func (nodeSet *ChainNodeSet) HasValidator() bool {
	return nodeSet.Spec.Validator != nil
}

func (nodeSet *ChainNodeSet) ShouldInitGenesis() bool {
	return nodeSet.Spec.Validator != nil && nodeSet.Spec.Validator.Init != nil
}

func (nodeSet *ChainNodeSet) GetLastUpgradeVersion() string {
	version := nodeSet.Spec.App.GetImageVersion()
	var h int64 = 0
	for _, u := range nodeSet.Status.Upgrades {
		if (u.Status == UpgradeCompleted || u.Status == UpgradeSkipped) && u.Height > h && u.Height <= nodeSet.Status.LatestHeight {
			h = u.Height
			version = u.GetVersion()
		}
	}
	return version
}

func (nodeSet *ChainNodeSet) GetAppSpecWithUpgrades() AppSpec {
	spec := nodeSet.Spec.App.DeepCopy()

	for _, u := range nodeSet.Status.Upgrades {
		upgradeSpec := UpgradeSpec{
			Height: u.Height,
			Image:  u.Image,
		}
		if u.Source == OnChainUpgrade {
			upgradeSpec.ForceOnChain = pointer.Bool(true)
		}

		if !utils.SliceContainsObj(spec.Upgrades, upgradeSpec, func(a UpgradeSpec, b UpgradeSpec) bool {
			return a.Height == b.Height
		}) {
			spec.Upgrades = append(spec.Upgrades, upgradeSpec)
		}
	}

	// Sort upgrades by height
	sort.Slice(spec.Upgrades, func(i, j int) bool {
		return spec.Upgrades[i].Height < spec.Upgrades[j].Height
	})

	return *spec
}

func (nodeSet *ChainNodeSet) GetValidatorMinimumGasPrices() string {
	if nodeSet.HasValidator() && nodeSet.Spec.Validator.Config != nil && nodeSet.Spec.Validator.Config.Override != nil {
		cfgOverride := *nodeSet.Spec.Validator.Config.Override
		if cfgRaw, ok := cfgOverride["app.toml"]; ok {
			var cfg map[string]interface{}
			if err := json.Unmarshal(cfgRaw.Raw, &cfg); err != nil {
				return ""
			}
			if price, ok := cfg["minimum-gas-prices"]; ok {
				return price.(string)
			}
		}
	}
	return ""
}

// Node group methods

func (group *NodeGroupSpec) GetInstances() int {
	if group.Instances != nil {
		return *group.Instances
	}
	return DefaultGroupInstances
}

func (group *NodeGroupSpec) GetIngressSecretName(owner client.Object) string {
	if group.Ingress != nil && group.Ingress.TlsSecretName != nil {
		return *group.Ingress.TlsSecretName
	}
	return fmt.Sprintf("%s-%s-tls", owner.GetName(), group.Name)
}

func (group *NodeGroupSpec) GetIngressClass() string {
	if group.Ingress != nil && group.Ingress.IngressClass != nil {
		return *group.Ingress.IngressClass
	}
	return DefaultIngressClass
}

func (group *NodeGroupSpec) GetGrpcAnnotations() map[string]string {
	if group.Ingress != nil && group.Ingress.GrpcAnnotations != nil {
		return group.Ingress.GrpcAnnotations
	}
	if strings.Contains(group.GetIngressClass(), DefaultIngressClass) {
		return map[string]string{
			"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
		}
	}
	return nil
}

func (group *NodeGroupSpec) UseInternal() bool {
	if group.Ingress != nil && group.Ingress.UseInternalServices != nil {
		return *group.Ingress.UseInternalServices
	}
	return false
}

func (group *NodeGroupSpec) GetServiceName(owner client.Object) string {
	if group.UseInternal() {
		return fmt.Sprintf("%s-%s-internal", owner.GetName(), group.Name)
	}
	return fmt.Sprintf("%s-%s", owner.GetName(), group.Name)
}

func (group *NodeGroupSpec) ShouldInheritValidatorGasPrice() bool {
	if group.InheritValidatorGasPrice != nil {
		return *group.InheritValidatorGasPrice
	}
	return true
}

func (group *NodeGroupSpec) HasPdbEnabled() bool {
	if group.PDB != nil {
		return group.PDB.Enabled
	}
	return false
}

func (group *NodeGroupSpec) GetPdbMinAvailable() int {
	if group.PDB != nil && group.PDB.MinAvailable != nil {
		return *group.PDB.MinAvailable
	}
	return group.GetInstances() - 1
}

func (group *NodeGroupSpec) GetSnapshotNodeIndex() int {
	if group.SnapshotNodeIndex != nil {
		return *group.SnapshotNodeIndex
	}
	return 0
}

func (group *NodeGroupSpec) ShouldIgnoreGroupLabelOnDisruptions() bool {
	if group != nil && group.IgnoreGroupOnDisruptionChecks != nil {
		return *group.IgnoreGroupOnDisruptionChecks
	}
	return false
}

// Validator methods

func (val *NodeSetValidatorConfig) GetPrivKeySecretName(obj client.Object) string {
	if val != nil && val.PrivateKeySecret != nil {
		return *val.PrivateKeySecret
	}
	return fmt.Sprintf("%s-priv-key", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountHDPath() string {
	if val != nil && val.Init != nil && val.Init.AccountHDPath != nil {
		return *val.Init.AccountHDPath
	}
	return DefaultHDPath
}

func (val *NodeSetValidatorConfig) GetAccountSecretName(obj client.Object) string {
	if val != nil && val.Init != nil && val.Init.AccountMnemonicSecret != nil {
		return *val.Init.AccountMnemonicSecret
	}

	return fmt.Sprintf("%s-account", obj.GetName())
}

func (val *NodeSetValidatorConfig) GetAccountPrefix() string {
	if val != nil && val.Init != nil && val.Init.AccountPrefix != nil {
		return *val.Init.AccountPrefix
	}
	return DefaultAccountPrefix
}

func (val *NodeSetValidatorConfig) GetValPrefix() string {
	if val != nil && val.Init != nil && val.Init.ValPrefix != nil {
		return *val.Init.ValPrefix
	}
	return DefaultValPrefix
}

func (val *NodeSetValidatorConfig) GetInitUnbondingTime() string {
	if val != nil && val.Init != nil && val.Init.UnbondingTime != nil {
		return *val.Init.UnbondingTime
	}
	return DefaultUnbondingTime
}

func (val *NodeSetValidatorConfig) GetInitVotingPeriod() string {
	if val != nil && val.Init != nil && val.Init.VotingPeriod != nil {
		return *val.Init.VotingPeriod
	}
	return DefaultVotingPeriod
}

func (val *NodeSetValidatorConfig) HasPdbEnabled() bool {
	if val != nil && val.PDB != nil {
		return val.PDB.Enabled
	}
	return false
}

func (val *NodeSetValidatorConfig) GetPdbMinAvailable() int {
	if val != nil && val.PDB != nil && val.PDB.MinAvailable != nil {
		return *val.PDB.MinAvailable
	}
	return 0
}

// Global Ingress helper methods

func (gi *GlobalIngressConfig) GetName(owner client.Object) string {
	return fmt.Sprintf("%s-global-%s", owner.GetName(), gi.Name)
}

func (gi *GlobalIngressConfig) GetServiceName(owner client.Object) string {
	if gi.UseInternal() {
		return fmt.Sprintf("%s-global-%s-internal", owner.GetName(), gi.Name)
	}
	return fmt.Sprintf("%s-global-%s", owner.GetName(), gi.Name)
}

func (gi *GlobalIngressConfig) GetGrpcName(owner client.Object) string {
	return fmt.Sprintf("%s-global-%s-grpc", owner.GetName(), gi.Name)
}

func (gi *GlobalIngressConfig) GetTlsSecretName(owner client.Object) string {
	if gi.TlsSecretName != nil {
		return *gi.TlsSecretName
	}
	return fmt.Sprintf("%s-tls", gi.GetName(owner))
}

func (gi *GlobalIngressConfig) ShouldUseCosmoGuardPorts(nodeSet *ChainNodeSet) bool {
	for _, groupName := range gi.Groups {
		for _, group := range nodeSet.Spec.Nodes {
			if group.Name == groupName {
				if group.Config != nil && group.Config.CosmoGuardEnabled() {
					return true
				}
			}
		}
	}
	return false
}

func (gi *GlobalIngressConfig) HasGroup(name string) bool {
	for _, groupName := range gi.Groups {
		if groupName == name {
			return true
		}
	}
	return false
}

func (gi *GlobalIngressConfig) GetIngressClass() string {
	if gi != nil && gi.IngressClass != nil {
		return *gi.IngressClass
	}
	return DefaultIngressClass
}

func (gi *GlobalIngressConfig) GetGrpcAnnotations() map[string]string {
	if gi != nil && gi.GrpcAnnotations != nil {
		return gi.GrpcAnnotations
	}

	if strings.Contains(gi.GetIngressClass(), DefaultIngressClass) {
		return map[string]string{
			"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
		}
	}

	return nil
}

func (gi *GlobalIngressConfig) UseInternal() bool {
	if gi != nil && gi.UseInternalServices != nil {
		return *gi.UseInternalServices
	}
	return false
}

func (gi *GlobalIngressConfig) CreateServicesOnly() bool {
	if gi != nil && gi.ServicesOnly != nil {
		return *gi.ServicesOnly
	}
	return false
}

// Cosmoseed Helper Methods

func (cs *CosmoseedConfig) IsEnabled() bool {
	if cs != nil && cs.Enabled != nil {
		return *cs.Enabled
	}
	return false
}

func (cs *CosmoseedConfig) GetInstances() int {
	if !cs.IsEnabled() {
		return 0
	}
	if cs != nil && cs.Instances != nil {
		return *cs.Instances
	}
	return 1
}

func (cs *CosmoseedConfig) GetMaxInboundPeers() int {
	if cs != nil && cs.MaxInboundPeers != nil {
		return *cs.MaxInboundPeers
	}
	return 2000
}

func (cs *CosmoseedConfig) GetMaxOutboundPeers() int {
	if cs != nil && cs.MaxOutboundPeers != nil {
		return *cs.MaxOutboundPeers
	}
	return 20
}

func (cs *CosmoseedConfig) GetMaxPacketMsgPayloadSize() int {
	if cs != nil && cs.MaxPacketMsgPayloadSize != nil {
		return *cs.MaxPacketMsgPayloadSize
	}
	return 1024
}

func (cs *CosmoseedConfig) GetPeerQueueSize() int {
	if cs != nil && cs.PeerQueueSize != nil {
		return *cs.PeerQueueSize
	}
	return 1000
}

func (cs *CosmoseedConfig) GetDialWorkers() int {
	if cs != nil && cs.DialWorkers != nil {
		return *cs.DialWorkers
	}
	return 20
}

func (cs *CosmoseedConfig) GetLogLevel() string {
	if cs != nil && cs.LogLevel != nil {
		return *cs.LogLevel
	}
	return DefaultCosmoseedLogLevel
}

func (cs *CosmoseedConfig) GetAllowNonRoutable() bool {
	if cs != nil && cs.AllowNonRoutable != nil {
		return *cs.AllowNonRoutable
	}
	return false
}

func (cs *CosmoseedConfig) GetCosmoseedConfig(chainID, seeds string) (*cosmoseed.Config, error) {
	cfg, err := cosmoseed.DefaultConfig()
	if err != nil {
		return nil, err
	}

	cfg.ChainID = chainID
	cfg.Seeds = seeds

	cfg.AllowNonRoutable = cs.GetAllowNonRoutable()
	cfg.MaxOutboundPeers = cs.GetMaxOutboundPeers()
	cfg.MaxInboundPeers = cs.GetMaxInboundPeers()
	cfg.MaxPacketMsgPayloadSize = cs.GetMaxPacketMsgPayloadSize()
	cfg.PeerQueueSize = cs.GetPeerQueueSize()
	cfg.DialWorkers = cs.GetDialWorkers()
	cfg.LogLevel = cs.GetLogLevel()
	cfg.AddrBookFile = DefaultCosmoseedAddrBookFile
	return cfg, nil
}

func (csi *CosmoseedIngressConfig) GetIngressClass() string {
	if csi != nil && csi.IngressClass != nil {
		return *csi.IngressClass
	}
	return DefaultIngressClass
}
