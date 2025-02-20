package dataexporter

import (
	"fmt"
)

type Provider string

type Exporter interface {
	Provider() Provider
	Upload(dir, bucket, name string, opts ...UploadOption) error
	Delete(bucket, name string, opts ...DeleteOption) error
}

func FromProvider(p Provider) (Exporter, error) {
	switch p {
	case GCS:
		return NewGcsExporter()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", p)
	}
}
