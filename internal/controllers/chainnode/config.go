package chainnode

import (
	"strings"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

func GetKeyFormatter(chainNode *appsv1.ChainNode) *KeyFormatter {
	return &KeyFormatter{
		IsValidator: chainNode.IsValidator(),
		UseTmkms:    chainNode.UsesTmKms(),
		UseDashes:   chainNode.Spec.Config.UseDashedConfigToml(),
	}
}

type KeyFormatter struct {
	IsValidator bool
	UseTmkms    bool
	UseDashes   bool
}

func (kf *KeyFormatter) FormatKey(key string) string {
	if kf.UseDashes {
		return strings.ReplaceAll(key, "_", "-")
	}
	return key
}

func (kf *KeyFormatter) GetBaseConfigToml() map[string]interface{} {
	cfg := map[string]interface{}{
		kf.RPC(): map[string]interface{}{
			kf.Laddr():              "tcp://0.0.0.0:26657",
			kf.CorsAllowedOrigins(): []string{"*"},
		},
		kf.P2P(): map[string]interface{}{
			kf.AddrBookStrict():   false,
			kf.AllowDuplicateIP(): true,
		},
		kf.Instrumentation(): map[string]interface{}{
			kf.Prometheus(): true,
		},
		kf.LogFormat(): "json",
	}

	if kf.IsValidator {
		cfg[kf.P2P()].(map[string]interface{})[kf.Pex()] = false

		if kf.UseTmkms {
			cfg[kf.PrivValidatorLaddr()] = "tcp://0.0.0.0:5555"
		}
	}

	return cfg
}

func (kf *KeyFormatter) GetBaseAppToml() map[string]interface{} {
	cfg := map[string]interface{}{
		kf.API(): map[string]interface{}{
			kf.Enable():  true,
			kf.Address(): "tcp://0.0.0.0:1317",
		},
		kf.FormatKey("grpc"): map[string]interface{}{
			kf.Enable():  true,
			kf.Address(): "0.0.0.0:9090",
		},
		// For EVM
		kf.FormatKey("json-rpc"): map[string]interface{}{
			kf.Enable():    true,
			kf.Address():   "0.0.0.0:8545",
			kf.WsAddress(): "0.0.0.0:8546",
		},
	}
	return cfg
}

func (kf *KeyFormatter) RPC() string {
	return kf.FormatKey("rpc")
}

func (kf *KeyFormatter) Laddr() string {
	return kf.FormatKey("laddr")
}

func (kf *KeyFormatter) CorsAllowedOrigins() string {
	return kf.FormatKey("cors_allowed_origins")
}

func (kf *KeyFormatter) P2P() string {
	return kf.FormatKey("p2p")
}

func (kf *KeyFormatter) AddrBookStrict() string {
	return kf.FormatKey("addr_book_strict")
}

func (kf *KeyFormatter) AllowDuplicateIP() string {
	return kf.FormatKey("allow_duplicate_ip")
}

func (kf *KeyFormatter) Instrumentation() string {
	return kf.FormatKey("instrumentation")
}

func (kf *KeyFormatter) Prometheus() string {
	return kf.FormatKey("prometheus")
}

func (kf *KeyFormatter) LogFormat() string {
	return kf.FormatKey("log_format")
}

func (kf *KeyFormatter) Pex() string {
	return kf.FormatKey("pex")
}

func (kf *KeyFormatter) PrivValidatorLaddr() string {
	return kf.FormatKey("priv_validator_laddr")
}

func (kf *KeyFormatter) API() string {
	return kf.FormatKey("api")
}

func (kf *KeyFormatter) Enable() string {
	return kf.FormatKey("enable")
}

func (kf *KeyFormatter) Address() string {
	return kf.FormatKey("address")
}

func (kf *KeyFormatter) GRPC() string {
	return kf.FormatKey("grpc")
}

func (kf *KeyFormatter) JsonRPC() string {
	return kf.FormatKey("json-rpc")
}

func (kf *KeyFormatter) WsAddress() string {
	return kf.FormatKey("ws-address")
}

func (kf *KeyFormatter) Moniker() string {
	return kf.FormatKey("moniker")
}

func (kf *KeyFormatter) AddrBookFile() string {
	return kf.FormatKey("addr_book_file")
}

func (kf *KeyFormatter) ExternalAddress() string {
	return kf.FormatKey("external_address")
}

func (kf *KeyFormatter) StateSync() string {
	return kf.FormatKey("statesync")
}

func (kf *KeyFormatter) SnapshotInterval() string {
	return kf.FormatKey("snapshot-interval")
}

func (kf *KeyFormatter) SnapshotKeepRecent() string {
	return kf.FormatKey("snapshot-keep-recent")
}

func (kf *KeyFormatter) SeedMode() string {
	return kf.FormatKey("seed_mode")
}

func (kf *KeyFormatter) GenesisFile() string {
	return kf.FormatKey("genesis_file")
}

func (kf *KeyFormatter) StateDashSync() string {
	return kf.FormatKey("state-sync")
}

func (kf *KeyFormatter) RPCServers() string {
	return kf.FormatKey("rpc_servers")
}

func (kf *KeyFormatter) TrustHeight() string {
	return kf.FormatKey("trust_height")
}

func (kf *KeyFormatter) TrustHash() string {
	return kf.FormatKey("trust_hash")
}

func (kf *KeyFormatter) TrustPeriod() string {
	return kf.FormatKey("trust_period")
}

func (kf *KeyFormatter) PersistentPeers() string {
	return kf.FormatKey("persistent_peers")
}

func (kf *KeyFormatter) UnconditionalPeerIDs() string {
	return kf.FormatKey("unconditional_peer_ids")
}

func (kf *KeyFormatter) PrivatePeerIDs() string {
	return kf.FormatKey("private_peer_ids")
}
