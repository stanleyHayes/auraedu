package ports

import (
	"context"

	"github.com/auraedu/file-service/internal/domain"
)

// Repository persists File aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, e *domain.File) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.File, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.File, string, error)
}
