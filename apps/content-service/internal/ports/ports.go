// Package ports defines content application boundaries.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/content-service/internal/domain"
)

type GenerateInput struct {
	ContentType, Title, Brief, Audience, Locale string
	KeyMessages                                 []string
	Facts                                       []domain.Fact
	Profile                                     domain.BrandProfile
}

type GenerateOutput struct {
	Content, Generator string
}

type Generator interface {
	Generate(context.Context, GenerateInput) (GenerateOutput, error)
}

type ListFilter struct {
	Status, ContentType, CampaignID string
	Limit                           int
}

type Repository interface {
	GetBrandProfile(context.Context, string) (domain.BrandProfile, error)
	UpsertBrandProfile(context.Context, domain.BrandProfile, int) error
	FindReplay(context.Context, string, string, string) (domain.Draft, string, bool, error)
	CreateDraftWithEvent(context.Context, domain.Draft, domain.Version, string, string, map[string]any) error
	GetDraft(context.Context, string, string) (domain.Draft, []domain.Version, error)
	ListDrafts(context.Context, string, ListFilter) ([]domain.Draft, error)
	UpdateDraftWithVersionAndEvent(context.Context, domain.Draft, int, time.Time, *domain.Version, string, map[string]any) error
}

type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
	CreatedAt               time.Time
}

type OutboxRepository interface {
	ClaimPending(context.Context, int) ([]OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}

func DraftGeneratedEventData(d domain.Draft) map[string]any {
	return map[string]any{"content_id": d.ID, "campaign_id": d.CampaignID, "content_type": d.ContentType, "version": d.Version,
		"generator": d.Generator, "brand_profile_version": d.BrandProfileVersion, "created_at": d.CreatedAt.UTC().Format(time.RFC3339)}
}

func StatusChangedEventData(d domain.Draft, previous domain.Status, actorID string) map[string]any {
	return map[string]any{"content_id": d.ID, "version": d.Version, "previous_status": previous, "status": d.Status,
		"actor_id": actorID, "changed_at": d.UpdatedAt.UTC().Format(time.RFC3339)}
}
