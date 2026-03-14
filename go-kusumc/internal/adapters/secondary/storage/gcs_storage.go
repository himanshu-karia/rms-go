package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GCSStorage struct {
	client *storage.Client
	bucket string
}

func NewGCSStorage() (*GCSStorage, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	// Ideally bucket name is from ENV
	return &GCSStorage{
		client: client,
		bucket: "unified-iot-cold-storage",
	}, nil
}

func (s *GCSStorage) Upload(ctx context.Context, filename string, data io.Reader) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	wc := s.client.Bucket(s.bucket).Object(filename).NewWriter(ctx)
	if _, err := io.Copy(wc, data); err != nil {
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("gs://%s/%s", s.bucket, filename), nil
}

func (s *GCSStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	return s.client.Bucket(s.bucket).Object(filename).NewReader(ctx)
}

func (s *GCSStorage) List(ctx context.Context, prefix string) ([]string, error) {
	it := s.client.Bucket(s.bucket).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})
	var files []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		files = append(files, attrs.Name)
	}
	return files, nil
}
