package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/attendance-service/internal/domain"
)

const (
	AttendanceMutationCreate     = "create"
	AttendanceMutationBulkUpsert = "bulk_upsert"
	AttendanceMutationUpdate     = "update"
	AttendanceMutationDelete     = "delete"
)

type LifecycleRepository interface {
	CommitAttendanceLifecycle(context.Context, string, string, []*domain.AttendanceRecord, string, []map[string]any) error
}
type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
}
type OutboxRepository interface {
	ClaimPendingAttendanceEvents(context.Context, int) ([]OutboxEvent, error)
	MarkAttendanceEventPublished(context.Context, string) error
	MarkAttendanceEventFailed(context.Context, string, string) error
}

func AttendanceEventData(r *domain.AttendanceRecord, meta map[string]any) map[string]any {
	data := map[string]any{
		"attendance_id":    r.ID,
		"student_id":       r.StudentID,
		"academic_year_id": r.AcademicYearID,
		"date":             r.Date.String(),
		"status":           r.Status,
		"marked_by":        r.MarkedBy,
	}
	if r.ClassID != nil {
		data["class_id"] = *r.ClassID
	}
	if r.SubjectID != nil {
		data["subject_id"] = *r.SubjectID
	}
	if r.Reason != nil {
		data["reason"] = *r.Reason
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}

// Repository persists AttendanceRecord aggregates. Implementations MUST scope every
// query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, r *domain.AttendanceRecord) error
	UpsertMany(ctx context.Context, tenantID string, records []*domain.AttendanceRecord) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.AttendanceRecord, error)
	List(ctx context.Context, tenantID string, filter ListFilter) ([]*domain.AttendanceRecord, string, error)
	Update(ctx context.Context, tenantID string, r *domain.AttendanceRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

type LearnerScopeResolver interface {
	Resolve(ctx context.Context, tenantID, userID, role string) (LearnerScope, error)
}

type LearnerScope struct {
	StudentIDs []string
	ClassIDs   []string
}

// ListFilter carries cursor pagination and optional equality filters for listing.
type ListFilter struct {
	Limit          int
	Cursor         string
	StudentID      string
	StudentIDs     []string
	AcademicYearID string
	Date           string
	Status         string
}
