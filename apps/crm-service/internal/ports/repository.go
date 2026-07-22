// Package ports defines the CRM service boundaries.
package ports

import (
	"context"

	"github.com/auraedu/crm-service/internal/domain"
)

type LeadFilter struct {
	Stage       *domain.LeadStage
	OwnerUserID *string
	Search      string
}

// CaptureResult distinguishes a new lead from a tenant-local dedupe match.
type CaptureResult struct {
	Lead    *domain.Lead
	Created bool
	Replay  bool
}

type FeedbackResult struct {
	Feedback *domain.Feedback
	Replay   bool
}

type FeedbackRepository interface {
	SubmitFeedback(ctx context.Context, feedback *domain.Feedback, idempotencyKeyHash, requestHash string) (FeedbackResult, error)
}

type CallbackResult struct {
	Callback *domain.CallbackRequest
	Replay   bool
}

type CallbackRepository interface {
	FindCallbackReplay(ctx context.Context, tenantID, idempotencyKeyHash, requestHash string) (CallbackResult, bool, error)
	ScheduleCallback(ctx context.Context, callback *domain.CallbackRequest, idempotencyKeyHash, requestHash string) (CallbackResult, error)
	ListCallbacks(ctx context.Context, tenantID string, status domain.CallbackStatus, limit int) ([]*domain.CallbackRequest, error)
}

// Repository implementations must apply tenant scope to every operation. Capture
// atomically enforces idempotency, deduplicates contact data and records the first
// inbound interaction when supplied.
type Repository interface {
	Capture(ctx context.Context, lead *domain.Lead, idempotencyKeyHash, requestHash string, initial *domain.Interaction) (CaptureResult, error)
	GetLead(ctx context.Context, tenantID, leadID string) (*domain.Lead, error)
	ListLeads(ctx context.Context, tenantID string, limit int, cursor string, filter LeadFilter) ([]*domain.Lead, string, error)
	UpdateLead(ctx context.Context, tenantID string, lead *domain.Lead) error
	CreateInteraction(ctx context.Context, tenantID string, interaction *domain.Interaction) error
	ListInteractions(ctx context.Context, tenantID, leadID string, limit int, cursor string) ([]*domain.Interaction, string, error)
	GetScoringEvidence(ctx context.Context, tenantID, leadID string) (domain.ScoringEvidence, error)
	SaveLeadScore(ctx context.Context, tenantID, leadID, triggeredBy string, score domain.LeadScore) (bool, error)
}
