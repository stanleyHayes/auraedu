package ports

import (
	"context"

	"github.com/auraedu/audit-service/internal/domain"
)

// Repository persists immutable AuditLog aggregates. Implementations MUST scope
// every query by tenant_id and rely on platform/db.WithTx so that Postgres
// Row-Level Security is effective.
type Repository interface {
	// Insert writes a new audit log record for its tenant.
	Insert(ctx context.Context, log *domain.AuditLog) error
	// List returns a tenant-scoped page of audit logs ordered newest-first.
	// limit is normalized to a sensible page size by the implementation.
	// cursor is an opaque value returned by a previous call; empty means the first page.
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AuditLog, string, error)
}
