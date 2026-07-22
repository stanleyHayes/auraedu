package ports

import (
	"context"
	"io"
)

// ReportStorage persists generated PDFs outside the report-service process.
// Implementations must reject object keys outside the supplied tenant scope.
type ReportStorage interface {
	Save(ctx context.Context, tenantID, objectKey string, content []byte) (string, error)
	Open(ctx context.Context, tenantID, storagePath string) (io.ReadCloser, error)
}
