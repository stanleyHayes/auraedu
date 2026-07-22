// Package storage implements file-service storage backends.
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
)

// LocalStorage stores files on the local filesystem, scoped by tenantID.
type LocalStorage struct {
	baseDir string
}

var _ ports.Storage = (*LocalStorage)(nil)
var _ ports.DeliveryURLProvider = (*LocalStorage)(nil)

// NewLocalStorage creates a local filesystem storage adapter backed by baseDir.
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{baseDir: baseDir}
}

// Backend returns the domain backend key for local storage.
func (s *LocalStorage) Backend() string { return string(domain.BackendLocal) }

// DeliveryURL is not supported for local filesystem storage.
func (s *LocalStorage) DeliveryURL(_, path, resourceType, transform string) (string, error) {
	_ = path
	_ = resourceType
	_ = transform
	return "", fmt.Errorf("local storage does not support delivery URLs")
}

// Save writes the contents of r to a tenant-scoped path and returns the relative path.
func (s *LocalStorage) Save(ctx context.Context, tenantID, fileID string, r io.Reader) (string, error) {
	_ = ctx
	if tenantID == "" || fileID == "" {
		return "", fmt.Errorf("tenant_id and file_id are required")
	}
	dir, err := s.tenantRoot(tenantID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create tenant dir: %w", err)
	}
	path, err := confinedPath(dir, filepath.Join(dir, fileID))
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // confinedPath proves the target remains under the tenant root.
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Default().ErrorContext(ctx, "failed to close local file", "path", path, "err", err)
		}
	}()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return path, nil
}

// Open returns a reader for the stored object at path.
func (s *LocalStorage) Open(ctx context.Context, tenantID, path string) (io.ReadCloser, error) {
	_ = ctx
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	root, err := s.tenantRoot(tenantID)
	if err != nil {
		return nil, err
	}
	path, err = confinedPath(root, path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) //nolint:gosec // confinedPath proves the target remains under the tenant root.
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
	if path == "" {
		return nil
	}
	root, err := s.tenantRoot(tenantID)
	if err != nil {
		return err
	}
	path, err = confinedPath(root, path)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

func (s *LocalStorage) tenantRoot(tenantID string) (string, error) {
	if tenantID == "" {
		return "", fmt.Errorf("tenant_id is required")
	}
	base, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	root, err := filepath.Abs(filepath.Join(base, tenantID))
	if err != nil {
		return "", fmt.Errorf("resolve tenant storage root: %w", err)
	}
	if _, err := confinedPath(base, root); err != nil {
		return "", fmt.Errorf("invalid tenant storage root: %w", err)
	}
	return root, nil
}

func confinedPath(root, candidate string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	candidate, err = filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes tenant storage root")
	}
	return candidate, nil
}
