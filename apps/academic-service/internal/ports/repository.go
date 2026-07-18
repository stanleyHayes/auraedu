package ports

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
)

// AcademicYearRepository persists AcademicYear aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type AcademicYearRepository interface {
	Create(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.AcademicYear, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AcademicYear, string, error)
	Update(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	Delete(ctx context.Context, tenantID, id string) error
}

// TermRepository persists Term aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type TermRepository interface {
	Create(ctx context.Context, tenantID string, t *domain.Term) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Term, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Term, string, error)
	Update(ctx context.Context, tenantID string, t *domain.Term) error
	Delete(ctx context.Context, tenantID, id string) error
}

// ClassRepository persists Class aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type ClassRepository interface {
	Create(ctx context.Context, tenantID string, c *domain.Class) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Class, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Class, string, error)
	Update(ctx context.Context, tenantID string, c *domain.Class) error
	Delete(ctx context.Context, tenantID, id string) error
}

// SubjectRepository persists Subject aggregates. Implementations MUST scope every query
// by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type SubjectRepository interface {
	Create(ctx context.Context, tenantID string, s *domain.Subject) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Subject, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Subject, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Subject) error
	Delete(ctx context.Context, tenantID, id string) error
}
