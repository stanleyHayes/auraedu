package ports

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
)

// Repository persists AcademicYear aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.AcademicYear, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AcademicYear, string, error)
	Update(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	Delete(ctx context.Context, tenantID, id string) error
}
