// Package ports defines knowledge application boundaries.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/knowledge-service/internal/domain"
)

// TransactionalApprovalRepository commits approval and its public lifecycle
// event atomically. PostgreSQL implements this boundary with an outbox.
type TransactionalApprovalRepository interface {
	ApproveWithEvent(context.Context, string, string, string, string, time.Time, string) (domain.Source, error)
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

type Repository interface {
	Create(context.Context, domain.Source) error
	Get(context.Context, string, string) (domain.Source, error)
	List(context.Context, string, domain.Status, int) ([]domain.Source, error)
	Approve(context.Context, string, string, string, string, time.Time) (domain.Source, error)
	Retire(context.Context, string, string, time.Time) (domain.Source, error)
	Search(context.Context, string, string, string, int, time.Time) ([]domain.SearchResult, error)
}

type EventPublisher interface {
	Publish(context.Context, string, string, map[string]any) error
}

func ApprovalEventData(source domain.Source) map[string]any {
	return map[string]any{
		"source_id":    source.ID,
		"source_type":  source.SourceType,
		"locale":       source.Locale,
		"version":      source.Version,
		"effective_at": source.EffectiveAt.Format(time.RFC3339),
	}
}
