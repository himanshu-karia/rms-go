package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	rootDir string
}

func NewLocalStorage(rootDir string) *LocalStorage {
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		os.MkdirAll(rootDir, 0755)
	}
	return &LocalStorage{rootDir: rootDir}
}

func (s *LocalStorage) Upload(ctx context.Context, filename string, data io.Reader) (string, error) {
	filePath := filepath.Join(s.rootDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, data)
	return filePath, err
}

func (s *LocalStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.rootDir, filename)
	return os.Open(filePath)
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	// Simple glob matching or readdir
	// For V1, we just list all in rootDir
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}
