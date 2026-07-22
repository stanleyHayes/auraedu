// Package ports defines the report service repository boundary.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/report-service/internal/domain"
)

// OutboxEvent is a durable integration event claimed by the worker. ID is
// stable across retries and becomes both the CloudEvent ID and idempotency key.
type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

// OutboxRepository dispatches events written in the same transaction as the
// aggregate transition that produced them.
type OutboxRepository interface {
	ClaimPendingReportEvents(context.Context, int) ([]OutboxEvent, error)
	MarkReportEventPublished(context.Context, string) error
	MarkReportEventFailed(context.Context, string, string) error
}

const (
	ReportMutationCreate = "create"
	ReportMutationUpdate = "update"
	ReportMutationDelete = "delete"
)

// LifecycleRepository atomically commits an aggregate mutation and the event
// that describes it. Production repositories should implement this boundary;
// simpler adapters may rely on the application service's legacy fallback.
type LifecycleRepository interface {
	CommitReportTemplateLifecycle(context.Context, string, *domain.ReportTemplate, string, string, map[string]any) error
	CommitReportCardLifecycle(context.Context, string, *domain.ReportCard, string, string, map[string]any) error
}

// Repository persists ReportTemplate and ReportCard aggregates. Implementations
// MUST scope every query by tenantID (defense-in-depth with Postgres RLS).
type Repository interface {
	// Report templates.
	CreateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error
	GetReportTemplateByID(ctx context.Context, tenantID, id string) (*domain.ReportTemplate, error)
	ListReportTemplates(ctx context.Context, tenantID string, filter ReportTemplateListFilter) ([]*domain.ReportTemplate, string, error)
	UpdateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error
	DeleteReportTemplate(ctx context.Context, tenantID, id string) error

	// Report cards.
	CreateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error
	GetReportCardByID(ctx context.Context, tenantID, id string) (*domain.ReportCard, error)
	ListReportCards(ctx context.Context, tenantID string, filter ReportCardListFilter) ([]*domain.ReportCard, string, error)
	UpdateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error
	DeleteReportCard(ctx context.Context, tenantID, id string) error
	ListTranscriptReportCards(ctx context.Context, tenantID, studentID string) ([]*domain.ReportCard, error)

	// Durable PDF generation queue. Enqueue atomically moves the card to
	// generating; claim is platform-scoped and lease-based so crashed workers
	// are recovered; completion/failure update job and card in one transaction.
	EnqueueReportGeneration(ctx context.Context, tenantID, reportCardID string) (*domain.ReportCard, error)
	ClaimReportGeneration(ctx context.Context, lease time.Duration) (*domain.GenerationJob, error)
	CompleteReportGeneration(ctx context.Context, job *domain.GenerationJob, storagePath string) (*domain.ReportCard, error)
	RetryReportGeneration(ctx context.Context, job *domain.GenerationJob, message string, maxAttempts int) (terminal bool, err error)

	// FindDraftReportCard returns the DRAFT report card for a student and period
	// (term). With a non-empty termID, cards whose term is NULL (period not yet
	// assigned) also match and an exact term match wins. With an empty termID
	// (events that carry no term, e.g. attendance.marked) every draft for the
	// student matches and the most recently created wins. It returns
	// domain.ErrNotFound when no draft exists.
	FindDraftReportCard(ctx context.Context, tenantID, studentID, termID string) (*domain.ReportCard, error)

	// Materialized entries (fed by assessment/attendance events). Upserts are
	// idempotent on their natural keys: (report_card_id, source_key) for scores
	// and (report_card_id, date) for attendance.
	UpsertScoreEntry(ctx context.Context, tenantID string, e *domain.ScoreEntry) error
	UpsertAttendanceEntry(ctx context.Context, tenantID string, e *domain.AttendanceEntry) error
	ListScoreEntries(ctx context.Context, tenantID, reportCardID string) ([]*domain.ScoreEntry, error)
	ListAttendanceEntries(ctx context.Context, tenantID, reportCardID string) ([]*domain.AttendanceEntry, error)
}

type LearnerScopeResolver interface {
	Resolve(context.Context, string, string, string) ([]string, error)
}

// ReportTemplateListFilter carries cursor pagination and optional equality filters.
type ReportTemplateListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	Status         string
}

// ReportCardListFilter carries cursor pagination and optional equality filters.
type ReportCardListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	Status         string
	StudentID      string
	StudentIDs     []string
	TemplateID     string
}
