package tmkms

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultTmKmsImage = "ghcr.io/iqlusioninc/tmkms"
	configFileName    = "config.toml"
	tmkmsAppName      = "tmkms"
	identityKeyName   = "kms-identity.key"
)

func defaultConfig() *Config {
	return &Config{
		Image:        DefaultTmKmsImage,
		Chains:       make([]*ChainConfig, 0),
		Validators:   make([]*ValidatorConfig, 0),
		Providers:    make(map[string][]Provider),
		PersistState: true,
	}
}

type Config struct {
	Image        string                `toml:"-"`
	Chains       []*ChainConfig        `toml:"chain"`
	Validators   []*ValidatorConfig    `toml:"validator"`
	Providers    map[string][]Provider `toml:"providers"`
	PersistState bool                  `toml:"-"`
}

type Option func(*Config)

type Provider interface {
	getVolumes() []corev1.Volume
	getVolumeMounts() []corev1.VolumeMount
	getContainers() []corev1.Container
}

func WithImage(s string) Option {
	return func(cfg *Config) {
		cfg.Image = s
	}
}

func PersistState(b bool) Option {
	return func(cfg *Config) {
		cfg.PersistState = b
	}
}

func WithProvider(p Provider) Option {
	return func(cfg *Config) {
		if _, ok := cfg.Providers[hashicorpProviderName]; !ok {
			cfg.Providers[hashicorpProviderName] = make([]Provider, 0)
		}
		cfg.Providers[hashicorpProviderName] = append(cfg.Providers[hashicorpProviderName], p)
	}
}
