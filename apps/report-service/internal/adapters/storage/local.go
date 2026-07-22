// Package storage provides durable-object adapters for generated reports.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/auraedu/report-service/internal/ports"
)

// Local stores reports below a configured root. It is intended for tests and
// local Compose, where server and worker mount the same named volume.
type Local struct{ root string }

var _ ports.ReportStorage = (*Local)(nil)

func NewLocal(root string) (*Local, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("report storage root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve report storage root: %w", err)
	}
	return &Local{root: abs}, nil
}

func (s *Local) Save(_ context.Context, tenantID, objectKey string, content []byte) (string, error) {
	path, err := s.path(tenantID, objectKey)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create report storage directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", fmt.Errorf("write report object: %w", err)
	}
	return filepath.ToSlash(filepath.Join(tenantID, objectKey)), nil
}

func (s *Local) Open(_ context.Context, tenantID, storagePath string) (io.ReadCloser, error) {
	path, err := s.path(tenantID, storagePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) //nolint:gosec // path is tenant-confined above.
	if err != nil {
		return nil, fmt.Errorf("open report object: %w", err)
	}
	return f, nil
}

func (s *Local) path(tenantID, objectKey string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	objectKey = filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if tenantID == "" || objectKey == "." || filepath.IsAbs(objectKey) {
		return "", fmt.Errorf("invalid report storage key")
	}
	prefix := tenantID + string(filepath.Separator)
	objectKey = strings.TrimPrefix(objectKey, prefix)
	if objectKey == ".." || strings.HasPrefix(objectKey, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("report storage key escapes tenant scope")
	}
	path := filepath.Join(s.root, tenantID, objectKey)
	root := filepath.Join(s.root, tenantID) + string(filepath.Separator)
	if !strings.HasPrefix(path, root) {
		return "", fmt.Errorf("report storage key escapes tenant scope")
	}
	return path, nil
}
