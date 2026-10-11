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

	// DefaultTraceStore is the default path to the trace store FIFO.
	DefaultTraceStore = "/trace/trace.fifo"
)

func defaultOptions() *Options {
	return &Options{
		DataPath:       DefaultDataPath,
		Host:           DefaultHost,
		Port:           DefaultPort,
		BlockThreshold: 0,
		UpgradesConfig: DefaultUpgradesConfig,
		TraceStore:     DefaultTraceStore,
		CreateFifo:     false,
		TmkmsProxy:     false,
		HaltHeight:     0,
	}
}

type Options struct {
	Host           string
	Port           int
	DataPath       string
	BlockThreshold time.Duration
	UpgradesConfig string
	TraceStore     string
	CreateFifo     bool
	TmkmsProxy     bool
	HaltHeight     int64
	MockMode       bool
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

func WithTraceStore(path string) Option {
	return func(opts *Options) {
		opts.TraceStore = path
	}
}

func CreateFifo(create bool) Option {
	return func(opts *Options) {
		opts.CreateFifo = create
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

func WithMockMode(enable bool) Option {
	return func(opts *Options) {
		opts.MockMode = enable
	}
}
