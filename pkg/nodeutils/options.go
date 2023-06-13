package nodeutils

import "time"

func defaultOptions() *Options {
	return &Options{
		DataPath:       "/home/app/data",
		Host:           "0.0.0.0",
		Port:           8000,
		BlockThreshold: 0,
	}
}

type Options struct {
	Host           string
	Port           int
	DataPath       string
	BlockThreshold time.Duration
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

func WithBlockThreshold(n time.Duration) Option {
	return func(opts *Options) {
		opts.BlockThreshold = n
	}
}
