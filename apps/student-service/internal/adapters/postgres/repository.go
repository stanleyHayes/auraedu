package postgres

import (
	"context"

	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
)

// Repository is the Postgres implementation of ports.Repository.
// TODO(AURA): wire the pgx pool from platform/db; every query must SET app.tenant_id
// (RLS) and filter by tenant_id.
type Repository struct{}

var _ ports.Repository = (*Repository)(nil)

func NewRepository() *Repository { return &Repository{} }

func (r *Repository) Create(ctx context.Context, tenantID string, e *domain.Student) error {
	_ = ctx
	_ = tenantID
	_ = e
	return nil // TODO: INSERT
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error) {
	_ = ctx
	_ = tenantID
	_ = id
	return nil, domain.ErrNotFound // TODO: SELECT
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Student, string, error) {
	_ = ctx
	_ = tenantID
	_ = limit
	_ = cursor
	return nil, "", nil // TODO: SELECT ... cursor pagination
}
