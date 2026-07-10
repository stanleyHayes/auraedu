package ports

import (
	"context"

	"github.com/auraedu/student-service/internal/domain"
)

// Repository persists Student aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, e *domain.Student) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Student, string, error)
}
