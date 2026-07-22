package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/file-service/internal/domain"
)

const (
	FileMutationCreate = "create"
	FileMutationUpdate = "update"
	FileMutationDelete = "delete"
)

type LifecycleRepository interface {
	CommitFileLifecycle(context.Context, string, *domain.FileUpload, string, string, map[string]any) error
}

type OutboxEvent struct {
	ID          string
	TenantID    string
	EventType   string
	Payload     json.RawMessage
	CleanupPath string
}

type OutboxRepository interface {
	ClaimPendingFileEvents(context.Context, int) ([]OutboxEvent, error)
	MarkFileEventPublished(context.Context, string) error
	MarkFileEventFailed(context.Context, string, string) error
}

// UsageRecord is a per-day, per-tenant file-storage aggregate.
type UsageRecord struct {
	TenantID       string `json:"tenant_id"`
	Date           string `json:"date"`
	BytesStored    int64  `json:"bytes_stored"`
	BytesDelivered int64  `json:"bytes_delivered"`
}

// Repository persists FileUpload aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, f *domain.FileUpload) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.FileUpload, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.FileUpload, string, error)
	Update(ctx context.Context, tenantID string, f *domain.FileUpload) error
	Delete(ctx context.Context, tenantID, id string) error

	RecordStorage(ctx context.Context, tenantID string, bytes int64) error
	RecordDelivery(ctx context.Context, tenantID string, bytes int64) error
	GetUsage(ctx context.Context, tenantID string, limit int) ([]*UsageRecord, error)
}
