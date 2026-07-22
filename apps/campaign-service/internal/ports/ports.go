// Package ports defines Campaign core adapter contracts.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/campaign-service/internal/domain"
)

type Repository interface {
	Create(context.Context, domain.Campaign) error
	Get(context.Context, string, string) (domain.Campaign, error)
	List(context.Context, string, domain.Status, int) ([]domain.Campaign, error)
	Update(context.Context, domain.Campaign, domain.Status) error
}

type EventPublisher interface {
	Publish(context.Context, string, string, map[string]any) error
}

// StatusChangedEventData is the canonical status-transition payload shared by
// direct publication and the transactional outbox.
func StatusChangedEventData(campaign domain.Campaign, previous domain.Status) map[string]any {
	return map[string]any{
		"campaign_id":     campaign.ID,
		"previous_status": previous,
		"status":          campaign.Status,
		"changed_at":      campaign.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// TransactionalRepository persists a state transition and its integration
// event atomically. PostgreSQL implements this with the outbox pattern so an
// unavailable event bus can never create a false API failure after commit.
type TransactionalRepository interface {
	UpdateWithEvent(context.Context, domain.Campaign, domain.Status, string, map[string]any) error
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

type OutboxRepository interface {
	ClaimPending(context.Context, int) ([]OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}
