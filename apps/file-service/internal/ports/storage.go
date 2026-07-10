package ports

import (
	"context"
	"io"
)

// Storage persists and retrieves file bytes. Implementations MUST scope every object by tenantID.
type Storage interface {
	// Save writes the contents of r for the given tenant and file ID. It returns the
	// storage-specific path that can later be passed to Open.
	Save(ctx context.Context, tenantID, fileID string, r io.Reader) (path string, err error)
	// Open returns a reader for the stored object at path, scoped to tenantID.
	Open(ctx context.Context, tenantID, path string) (io.ReadCloser, error)
	// Delete removes the stored object at path, scoped to tenantID.
	Delete(ctx context.Context, tenantID, path string) error
}
