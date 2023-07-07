package tmkms

type ProtocolVersion string

const (
	ProtocolVersionLegacy ProtocolVersion = "legacy"
	ProtocolVersionV0_33  ProtocolVersion = "v0.33"
	ProtocolVersionV0_34  ProtocolVersion = "v0.34"

	defaultSecretKey = "/data/kms-identity.key"
)

type ValidatorConfig struct {
	ChainID         string          `toml:"chain_id"`
	Address         string          `toml:"addr"`
	ProtocolVersion ProtocolVersion `toml:"protocol_version"`
	Reconnect       bool            `toml:"reconnect"`
	SecretKey       string          `toml:"secret_key"`
}

type ValidatorOption func(*ValidatorConfig)

func WithProtocolVersion(v ProtocolVersion) ValidatorOption {
	return func(cfg *ValidatorConfig) {
		cfg.ProtocolVersion = v
	}
}

func WithReconnect(v bool) ValidatorOption {
	return func(cfg *ValidatorConfig) {
		cfg.Reconnect = v
	}
}

func WithValidator(chainID, address string, opts ...ValidatorOption) Option {
	validatorConfig := &ValidatorConfig{
		ChainID:         chainID,
		Address:         address,
		ProtocolVersion: ProtocolVersionV0_34,
		Reconnect:       true,
		SecretKey:       defaultSecretKey,
	}
	for _, opt := range opts {
		opt(validatorConfig)
	}

	return func(cfg *Config) {
		cfg.Validators = append(cfg.Validators, validatorConfig)
	}
}
