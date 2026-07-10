package memory

import (
	"context"
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

// List returns a tenant-scoped page. Pagination is ordered newest-first by id.
func (r *Repository) List(_ context.Context, tenantID string, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 25
	}

	var out []*domain.AuditLog
	var next string
	found := cursor == ""
	for _, log := range r.logs {
		if log.TenantID.String() != tenantID {
			continue
		}
		if !found {
			if log.ID.String() == cursor {
				found = true
			}
			continue
		}
		if len(out) < limit {
			out = append(out, log)
		} else {
			next = log.ID.String()
			break
		}
	}
	return out, next, nil
}
