package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Storage struct {
	client   *s3.S3
	uploader *s3manager.Uploader
	bucket   string
	endpoint string
}

func NewS3Storage() *S3Storage {
	endpoint := os.Getenv("MG_BACKEND_OBJECT_STORAGE_ENDPOINT") // Optional custom endpoint
	if endpoint == "" {
		endpoint = "http://seaweedfs-s3:8333" // Default for Local Dev
	}
	accessKey := os.Getenv("MG_BACKEND_OBJECT_STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("MG_BACKEND_OBJECT_STORAGE_SECRET_KEY")
	bucket := os.Getenv("MG_BACKEND_OBJECT_STORAGE_BUCKET")
	if bucket == "" {
		bucket = "cold-storage"
	}

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String("us-east-1"),
		S3ForcePathStyle: aws.Bool(true),
	}
	sess, _ := session.NewSession(s3Config)

	return &S3Storage{
		client:   s3.New(sess),
		uploader: s3manager.NewUploader(sess),
		bucket:   bucket,
		endpoint: endpoint,
	}
}

func (s *S3Storage) Upload(ctx context.Context, filename string, data io.Reader) (string, error) {
	// 5 Minute timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, err := s.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filename),
		Body:   data,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, filename), nil
}

func (s *S3Storage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	out, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	out, err := s.client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	var files []string
	for _, obj := range out.Contents {
		files = append(files, *obj.Key)
	}
	return files, nil
}
