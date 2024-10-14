package tmkms

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultTmKmsImage  = "ghcr.io/iqlusioninc/tmkms"
	configFileName     = "config.toml"
	tmkmsAppName       = "tmkms"
	identityKeyName    = "kms-identity.key"
	DefaultTmkmsCpu    = "100m"
	DefaultTmkmsMemory = "64Mi"
	tmkmsPvcSize       = "1Gi"
)

func defaultConfig() *Config {
	return &Config{
		Image:        DefaultTmKmsImage,
		Chains:       make([]*ChainConfig, 0),
		Validators:   make([]*ValidatorConfig, 0),
		Providers:    make(map[string][]Provider),
		PersistState: true,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(DefaultTmkmsCpu),
				corev1.ResourceMemory: resource.MustParse(DefaultTmkmsMemory),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(DefaultTmkmsCpu),
				corev1.ResourceMemory: resource.MustParse(DefaultTmkmsMemory),
			},
		},
	}
}

type Config struct {
	Image        string                      `toml:"-"`
	Chains       []*ChainConfig              `toml:"chain"`
	Validators   []*ValidatorConfig          `toml:"validator"`
	Providers    map[string][]Provider       `toml:"providers"`
	PersistState bool                        `toml:"-"`
	Resources    corev1.ResourceRequirements `toml:"-"`
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

func WithResources(res corev1.ResourceRequirements) Option {
	return func(cfg *Config) {
		cfg.Resources = res
	}
}
