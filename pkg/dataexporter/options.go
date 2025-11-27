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

// UploadOptions configures the behavior of data uploads to cloud storage.
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

// UploadOption is a functional option for configuring uploads.
type UploadOption func(*UploadOptions)

// WithChunkSize sets the chunk size for uploads.
func WithChunkSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.ChunkSize = datasize.MustParseString(size)
	}
}

// WithPartSize sets the part size for multi-part uploads.
func WithPartSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.PartSize = datasize.MustParseString(size)
	}
}

// WithSizeLimit sets the maximum size before splitting into parts.
func WithSizeLimit(size string) UploadOption {
	return func(o *UploadOptions) {
		o.SizeLimit = datasize.MustParseString(size)
	}
}

// WithBufferSize sets the buffer size for upload operations.
func WithBufferSize(size string) UploadOption {
	return func(o *UploadOptions) {
		o.BufferSize = datasize.MustParseString(size)
	}
}

// WithReportPeriod sets how often progress is reported.
func WithReportPeriod(period time.Duration) UploadOption {
	return func(o *UploadOptions) {
		o.ReportPeriod = period
	}
}

// WithConcurrentUploadJobs sets the number of concurrent upload workers.
func WithConcurrentUploadJobs(concurrentJobs int) UploadOption {
	return func(o *UploadOptions) {
		o.ConcurrentJobs = concurrentJobs
	}
}

// DeleteOptions configures the behavior of data deletion from cloud storage.
type DeleteOptions struct {
	ConcurrentJobs int
}

func defaultDeleteOptions() *DeleteOptions {
	return &DeleteOptions{
		ConcurrentJobs: DefaultConcurrentJobs,
	}
}

// DeleteOption is a functional option for configuring deletions.
type DeleteOption func(*DeleteOptions)

// WithConcurrentDeleteJobs sets the number of concurrent delete workers.
func WithConcurrentDeleteJobs(concurrentJobs int) DeleteOption {
	return func(o *DeleteOptions) {
		o.ConcurrentJobs = concurrentJobs
	}
}
