// Package ports defines the website-service driven ports.
package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/website-service/internal/domain"
)

const (
	WebsiteMutationPageCreate    = "page_create"
	WebsiteMutationPageUpdate    = "page_update"
	WebsiteMutationPageDelete    = "page_delete"
	WebsiteMutationSectionCreate = "section_create"
	WebsiteMutationSectionUpdate = "section_update"
	WebsiteMutationSectionDelete = "section_delete"
)

type LifecycleEvent struct {
	EventType string
	Payload   map[string]any
}

type LifecycleRepository interface {
	CommitWebsiteLifecycle(context.Context, string, string, *domain.Page, *domain.Section, []LifecycleEvent) error
	ProvisionDefaultWebsite(context.Context, string, *domain.Page, *domain.Section, []LifecycleEvent) error
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
}

type OutboxRepository interface {
	ClaimPendingWebsiteEvents(context.Context, int) ([]OutboxEvent, error)
	MarkWebsiteEventPublished(context.Context, string) error
	MarkWebsiteEventFailed(context.Context, string, string) error
}

func PageEventData(page *domain.Page, meta map[string]any) map[string]any {
	data := map[string]any{"page_id": page.ID, "slug": page.Slug, "title": page.Title}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

func SectionEventData(section *domain.Section, meta map[string]any) map[string]any {
	data := map[string]any{"section_id": section.ID, "page_id": section.PageID, "type": section.Type}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

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
