// Package ports defines market-intelligence application boundaries.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/domain"
)

// LifecycleEventData is the canonical privacy-minimized payload shared by
// Market Intelligence application transitions and its transactional outbox.
func LifecycleEventData(id string, kind domain.Kind, actor string, at time.Time) map[string]any {
	return map[string]any{
		"id":            id,
		"kind":          kind,
		"actor_user_id": actor,
		"occurred_at":   at.UTC().Format(time.RFC3339Nano),
	}
}

type Repository interface {
	CreateSource(context.Context, domain.Source, string, map[string]any) error
	GetSource(context.Context, string, string) (domain.Source, error)
	ListSources(context.Context, string, domain.Kind, int) ([]domain.Source, error)
	UpdateSource(context.Context, domain.Source, domain.Status, string, map[string]any) error
	CreateObservation(context.Context, domain.Observation, string, map[string]any) error
	GetObservation(context.Context, string, string) (domain.Observation, error)
	ListObservations(context.Context, string, domain.Kind, domain.Status, int) ([]domain.Observation, error)
	UpdateObservation(context.Context, domain.Observation, domain.Status, string, map[string]any) error
	GetAlertRule(context.Context, string) (domain.AlertRule, error)
	UpsertAlertRule(context.Context, domain.AlertRule, string, map[string]any) error
	ListAlerts(context.Context, string, string, int) ([]domain.Alert, error)
	GetAlert(context.Context, string, string) (domain.Alert, error)
	AcknowledgeAlert(context.Context, domain.Alert, string, map[string]any) error
	BuildSummaryItems(context.Context, string, time.Time, time.Time) ([]domain.SummaryItem, error)
	CreateSummary(context.Context, domain.CompetitorSummary, string, map[string]any) error
	GetSummary(context.Context, string, string) (domain.CompetitorSummary, error)
	ListSummaries(context.Context, string, domain.Status, int) ([]domain.CompetitorSummary, error)
	UpdateSummary(context.Context, domain.CompetitorSummary, domain.Status, string, map[string]any) error
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
