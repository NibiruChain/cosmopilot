package tmkms

const (
	DefaultKeyType         = "bech32"
	DefaultAccountPrefix   = "nibipub"
	DefaultConsensusPrefix = "nibivalconspub"

	defaultStateFile = "/data/priv_validator_state.json"
)

type ChainConfig struct {
	ChainID   string     `toml:"id"`
	KeyFormat *KeyFormat `toml:"key_format"`
	StateFile string     `toml:"state_file"`
}

type KeyFormat struct {
	Type               string `toml:"type"`
	AccountKeyPrefix   string `toml:"account_key_prefix"`
	ConsensusKeyPrefix string `toml:"consensus_key_prefix"`
}

type ChainOption func(*ChainConfig)

func WithKeyFormat(keyType, accountKeyPrefix, consensusKeyPrefix string) ChainOption {
	return func(cfg *ChainConfig) {
		cfg.KeyFormat = &KeyFormat{
			Type:               keyType,
			AccountKeyPrefix:   accountKeyPrefix,
			ConsensusKeyPrefix: consensusKeyPrefix,
		}
	}
}

func WithChain(chainID string, opts ...ChainOption) Option {
	chainConfig := &ChainConfig{
		ChainID: chainID,
		KeyFormat: &KeyFormat{
			Type:               DefaultKeyType,
			AccountKeyPrefix:   DefaultAccountPrefix,
			ConsensusKeyPrefix: DefaultConsensusPrefix,
		},
		StateFile: defaultStateFile,
	}
	for _, opt := range opts {
		opt(chainConfig)
	}

	return func(cfg *Config) {
		cfg.Chains = append(cfg.Chains, chainConfig)
	}
}
