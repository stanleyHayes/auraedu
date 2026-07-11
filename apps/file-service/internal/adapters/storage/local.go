package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
)

// LocalStorage stores files on the local filesystem, scoped by tenantID.
type LocalStorage struct {
	baseDir string
}

var _ ports.Storage = (*LocalStorage)(nil)

// NewLocalStorage creates a local filesystem storage adapter backed by baseDir.
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// Backend returns the domain backend key for local storage.
func (s *LocalStorage) Backend() string { return string(domain.BackendLocal) }

// Save writes the contents of r to a tenant-scoped path and returns the relative path.
func (s *LocalStorage) Save(ctx context.Context, tenantID, fileID string, r io.Reader) (string, error) {
	_ = ctx
	if tenantID == "" || fileID == "" {
		return "", fmt.Errorf("tenant_id and file_id are required")
	}
	dir := filepath.Join(s.baseDir, tenantID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create tenant dir: %w", err)
	}
	path := filepath.Join(dir, fileID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return path, nil
}

// Open returns a reader for the stored object at path.
func (s *LocalStorage) Open(ctx context.Context, tenantID, path string) (io.ReadCloser, error) {
	_ = ctx
	_ = tenantID
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes the stored object at path.
func (s *LocalStorage) Delete(ctx context.Context, tenantID, path string) error {
	_ = ctx
	_ = tenantID
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}
