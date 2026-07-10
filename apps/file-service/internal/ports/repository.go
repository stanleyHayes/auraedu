package ports

import (
	"context"

	"github.com/auraedu/file-service/internal/domain"
)

// Repository persists FileUpload aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, f *domain.FileUpload) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.FileUpload, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.FileUpload, string, error)
	Update(ctx context.Context, tenantID string, f *domain.FileUpload) error
	Delete(ctx context.Context, tenantID, id string) error
}
