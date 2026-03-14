package storage

import (
	"context"
	"io"
)

// StorageProvider defines the interface for Cloud Storage
type StorageProvider interface {
	// Upload streams data to the bucket
	Upload(ctx context.Context, filename string, data io.Reader) (string, error)

	// Download retrieves data from the bucket
	Download(ctx context.Context, filename string) (io.ReadCloser, error)

	// List returns a list of files with a prefix
	List(ctx context.Context, prefix string) ([]string, error)
}
