package dataexporter

import (
	"time"

	"github.com/c2h5oh/datasize"
)

const (
	DefaultSizeLimit      = "5TB"
	DefaultPartSize       = "500GB"
	DefaultChunkSize      = "250MB"
	DefaultBufferSize     = "32MB"
	DefaultReportPeriod   = time.Second
	DefaultConcurrentJobs = 10
)

type UploadOptions struct {
	PartSize       datasize.ByteSize
	ChunkSize      datasize.ByteSize
	SizeLimit      datasize.ByteSize
	ReportPeriod   time.Duration
	ConcurrentJobs int
	BufferSize     datasize.ByteSize
}

func defaultUploadOptions() *UploadOptions {
	return &UploadOptions{
		PartSize:       datasize.MustParseString(DefaultPartSize),
		ChunkSize:      datasize.MustParseString(DefaultChunkSize),
		SizeLimit:      datasize.MustParseString(DefaultSizeLimit),
		BufferSize:     datasize.MustParseString(DefaultBufferSize),
		ReportPeriod:   DefaultReportPeriod,
		ConcurrentJobs: DefaultConcurrentJobs,
	}
}

type UploadOption func(*UploadOptions)

func WithChunkSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.ChunkSize = datasize.MustParseString(size)
	}
}

func WithPartSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.PartSize = datasize.MustParseString(size)
	}
}

func WithSizeLimit(size string) UploadOption {
	return func(o *UploadOptions) {
		o.SizeLimit = datasize.MustParseString(size)
	}
}

func WithBufferSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.BufferSize = datasize.MustParseString(size)
	}
}

func WithReportPeriod(period time.Duration) UploadOption {
	return func(o *UploadOptions) {
		o.ReportPeriod = period
	}
}

func WithConcurrentUploadJobs(concurrentJobs int) UploadOption {
	return func(o *UploadOptions) {
		o.ConcurrentJobs = concurrentJobs
	}
}

type DeleteOptions struct {
	ConcurrentJobs int
}

func defaultDeleteOptions() *DeleteOptions {
	return &DeleteOptions{
		ConcurrentJobs: DefaultConcurrentJobs,
	}
}

type DeleteOption func(*DeleteOptions)

func WithConcurrentDeleteJobs(concurrentJobs int) DeleteOption {
	return func(o *DeleteOptions) {
		o.ConcurrentJobs = concurrentJobs
	}
}
