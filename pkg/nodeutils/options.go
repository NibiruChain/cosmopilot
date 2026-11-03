package nodeutils

import "time"

const (
	// DefaultPort is the default port for the node-utils HTTP server.
	DefaultPort = 8000

	// DefaultHost is the default host for the node-utils HTTP server.
	DefaultHost = "0.0.0.0"

	// DefaultDataPath is the default path to the node's data directory.
	DefaultDataPath = "/home/app/data"

	// DefaultUpgradesConfig is the default path to the upgrades configuration file.
	DefaultUpgradesConfig = "/config/upgrades.json"

	// DefaultTerminationMessagePath is captured by kubelet when node-utils exits.
	DefaultTerminationMessagePath = "/dev/termination-log"
)

func defaultOptions() *Options {
	return &Options{
		DataPath:               DefaultDataPath,
		Host:                   DefaultHost,
		Port:                   DefaultPort,
		BlockThreshold:         0,
		UpgradesConfig:         DefaultUpgradesConfig,
		TmkmsProxy:             false,
		HaltHeight:             0,
		TerminationMessagePath: DefaultTerminationMessagePath,
	}
}

type Options struct {
	Host                   string
	Port                   int
	DataPath               string
	BlockThreshold         time.Duration
	UpgradesConfig         string
	TmkmsProxy             bool
	HaltHeight             int64
	TerminationMessagePath string
	MockMode               bool
}

type Option func(*Options)

func WithHost(s string) Option {
	return func(opts *Options) {
		opts.Host = s
	}
}

func WithPort(v int) Option {
	return func(opts *Options) {
		opts.Port = v
	}
}

func WithDataPath(path string) Option {
	return func(opts *Options) {
		opts.DataPath = path
	}
}

func WithUpgradesConfig(path string) Option {
	return func(opts *Options) {
		opts.UpgradesConfig = path
	}
}

func WithBlockThreshold(n time.Duration) Option {
	return func(opts *Options) {
		opts.BlockThreshold = n
	}
}

func WithTmkmsProxy(enable bool) Option {
	return func(opts *Options) {
		opts.TmkmsProxy = enable
	}
}

func WithHaltHeight(height int64) Option {
	return func(opts *Options) {
		opts.HaltHeight = height
	}
}

func WithTerminationMessagePath(path string) Option {
	return func(opts *Options) {
		opts.TerminationMessagePath = path
	}
}

func WithMockMode(enable bool) Option {
	return func(opts *Options) {
		opts.MockMode = enable
	}
}
