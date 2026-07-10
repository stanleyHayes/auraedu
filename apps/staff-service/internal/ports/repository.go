package ports

import (
	"context"

	"github.com/auraedu/staff-service/internal/domain"
)

// Repository persists Staff aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, e *domain.Staff) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Staff, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Staff, string, error)
}
