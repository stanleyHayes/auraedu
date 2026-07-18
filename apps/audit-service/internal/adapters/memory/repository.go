// Package memory provides an in-memory implementation of the audit repository port.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
)

// Repository is an in-memory implementation of ports.Repository for tests and
// local stubs. It is protected by a mutex and therefore safe for concurrent use.
type Repository struct {
	mu   sync.Mutex
	logs []*domain.AuditLog
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository creates an empty in-memory repository.
func NewRepository() *Repository {
	return &Repository{}
}

// Insert appends the audit log to the in-memory store.
func (r *Repository) Insert(_ context.Context, log *domain.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

// List returns a tenant-scoped page. Pagination matches the Postgres adapter:
// newest-first by id (UUID v7), with the cursor being the last id of the
// previous page.
func (r *Repository) List(_ context.Context, tenantID string, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var scoped []*domain.AuditLog
	for _, log := range r.logs {
		if log.TenantID.String() == tenantID {
			scoped = append(scoped, log)
		}
	}
	logs, next := page(scoped, limit, cursor)
	return logs, next, nil
}

// ListAll returns a cross-tenant page for platform super admins, using the
// same ordering and cursor semantics as List.
func (r *Repository) ListAll(_ context.Context, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	logs, next := page(r.logs, limit, cursor)
	return logs, next, nil
}

func page(logs []*domain.AuditLog, limit int, cursor string) ([]*domain.AuditLog, string) {
	if limit <= 0 {
		limit = 25
	}

	sorted := make([]*domain.AuditLog, len(logs))
	copy(sorted, logs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID.String() > sorted[j].ID.String()
	})

	var out []*domain.AuditLog
	for _, log := range sorted {
		if cursor != "" && log.ID.String() >= cursor {
			continue
		}
		if len(out) == limit {
			break
		}
		out = append(out, log)
	}

	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID.String()
	}
	return out, next
}
