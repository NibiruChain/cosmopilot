package tmkms

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultTmKmsImage = "ghcr.io/nibiruchain/tmkms:hashicorp"
	DefaultPvcSize    = "10Gi"
	configFileName    = "config.toml"
	labelApp          = "app"
	tmkmsAppName      = "tmkms"
	labelOwner        = "owner"
)

func defaultConfig() *Config {
	return &Config{
		Image:      DefaultTmKmsImage,
		Chains:     make([]*ChainConfig, 0),
		Validators: make([]*ValidatorConfig, 0),
		Providers:  make(map[string][]Provider),
	}
}

type Config struct {
	Image      string                `toml:"-"`
	Chains     []*ChainConfig        `toml:"chain"`
	Validators []*ValidatorConfig    `toml:"validator"`
	Providers  map[string][]Provider `toml:"providers"`
}

type Option func(*Config)

type Provider interface {
	process(kms *KMS, ctx context.Context) error
	getVolumes() []corev1.Volume
	getVolumeMounts() []corev1.VolumeMount
}

func WithImage(s string) Option {
	return func(cfg *Config) {
		cfg.Image = s
	}
}
