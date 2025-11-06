// Package dataexporter provides interfaces and implementations for exporting
// blockchain node data to cloud storage providers.
package dataexporter

import (
	"fmt"
)

// Provider identifies a cloud storage provider.
type Provider string

// Exporter provides methods to upload and delete data snapshots from cloud storage.
type Exporter interface {
	// Provider returns the cloud storage provider type.
	Provider() Provider

	// Upload uploads a directory as a compressed tarball to the specified bucket.
	// The name parameter specifies the object name in the bucket.
	// Options can be provided to customize the upload behavior.
	Upload(dir, bucket, name string, opts ...UploadOption) error

	// Delete removes an object from the specified bucket.
	// Options can be provided to customize the delete behavior.
	Delete(bucket, name string, opts ...DeleteOption) error
}

// FromProvider creates an Exporter for the specified provider.
// Returns an error if the provider is not supported.
func FromProvider(p Provider) (Exporter, error) {
	switch p {
	case GCS:
		return NewGcsExporter()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", p)
	}
}
