// Package ports defines contracts between the orchestrator core and its adapters.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type KnowledgeRetriever interface {
	Search(context.Context, string, string, string, int, time.Time) ([]domain.KnowledgeResult, error)
}

type Repository interface {
	FindReplay(context.Context, string, string, string) (domain.Response, string, bool, error)
	Save(context.Context, domain.Response, string, string) error
	PurgeExpired(context.Context) (int64, error)
}

type TransactionalExchangeRepository interface {
	SaveWithEvent(context.Context, domain.Response, string, string, string, map[string]any) error
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

type EventPublisher interface {
	Publish(context.Context, string, string, map[string]any) error
}

func EscalationEventData(response domain.Response) map[string]any {
	return map[string]any{
		"session_id": response.SessionID,
		"message_id": response.MessageID,
		"locale":     response.Locale,
		"created_at": response.CreatedAt.Format(time.RFC3339),
	}
}

type ActionRepository interface {
	FindActionReplay(context.Context, string, string) (domain.ActionProposal, bool, error)
	CreateAction(context.Context, domain.ActionProposal, domain.ActionAuditEntry) error
	ListActions(context.Context, string, string, int) ([]domain.ActionProposal, error)
	GetAction(context.Context, string, string) (domain.ActionProposal, error)
	ReviewAction(context.Context, string, string, string, string, string, bool, time.Time) (domain.ActionProposal, error)
	StartActionExecution(context.Context, string, string, string, string, time.Time) (domain.ActionProposal, error)
	FinishActionExecution(context.Context, string, string, bool, json.RawMessage, string, string, time.Time) (domain.ActionProposal, error)
	ListActionAudit(context.Context, string, string) ([]domain.ActionAuditEntry, error)
}

type ActionExecutor interface {
	Execute(context.Context, domain.ActionProposal, auth.Actor) (domain.ActionExecutionResult, error)
}
