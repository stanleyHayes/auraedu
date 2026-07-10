package ports

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
)

// Repository persists Academic aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, e *domain.Academic) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Academic, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Academic, string, error)
}
