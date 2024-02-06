package nodeutils

import "time"

func defaultOptions() *Options {
	return &Options{
		DataPath:       "/home/app/data",
		Host:           "0.0.0.0",
		Port:           8000,
		BlockThreshold: 0,
		UpgradesConfig: "/config/upgrades.json",
		TraceStore:     "/trace/trace.fifo",
		CreateFifo:     false,
		TmkmsProxy:     false,
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
