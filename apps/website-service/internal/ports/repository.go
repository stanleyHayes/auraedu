package ports

import (
	"context"

	"github.com/auraedu/website-service/internal/domain"
)

// PageFilter filters page list results.
type PageFilter struct {
	Status *string
	Layout *string
}

// SectionFilter filters section list results.
type SectionFilter struct {
	Status *string
	Type   *string
}

// Repository persists Page and Section aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	CreatePage(ctx context.Context, tenantID string, p *domain.Page) error
	GetPageByID(ctx context.Context, tenantID, id string) (*domain.Page, error)
	GetPageBySlug(ctx context.Context, tenantID, slug string) (*domain.Page, error)
	ListPages(ctx context.Context, tenantID string, limit int, cursor string, filter PageFilter) ([]*domain.Page, string, error)
	UpdatePage(ctx context.Context, tenantID string, p *domain.Page) error
	DeletePage(ctx context.Context, tenantID, id string) error

	CreateSection(ctx context.Context, tenantID string, s *domain.Section) error
	GetSectionByID(ctx context.Context, tenantID, id string) (*domain.Section, error)
	ListSections(ctx context.Context, tenantID, pageID string, limit int, cursor string, filter SectionFilter) ([]*domain.Section, string, error)
	UpdateSection(ctx context.Context, tenantID string, s *domain.Section) error
	DeleteSection(ctx context.Context, tenantID, id string) error
	DeleteSectionsByPage(ctx context.Context, tenantID, pageID string) error
}
