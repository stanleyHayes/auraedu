// Package ports defines the staff service repository boundary.
package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/staff-service/internal/domain"
)

const (
	StaffMutationCreate = "create"
	StaffMutationUpdate = "update"
	StaffMutationDelete = "delete"
)

// LifecycleRepository atomically persists a staff mutation and its domain event.
type LifecycleRepository interface {
	CommitStaffLifecycle(context.Context, string, *domain.Staff, string, string, map[string]any) error
}

// OutboxEvent is a pending durable staff lifecycle event.
type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
}

// OutboxRepository supports retry-safe staff lifecycle event delivery.
type OutboxRepository interface {
	ClaimPendingStaffEvents(context.Context, int) ([]OutboxEvent, error)
	MarkStaffEventPublished(context.Context, string) error
	MarkStaffEventFailed(context.Context, string, string) error
}

// StaffEventData returns the canonical payload shared by direct and outbox publishers.
func StaffEventData(staff *domain.Staff, meta map[string]any) map[string]any {
	data := map[string]any{
		"staff_id":   staff.ID,
		"staff_type": staff.StaffType,
		"name":       staff.FullName(),
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

// Repository persists Staff aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, s *domain.Staff) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Staff, error)
	GetByUserID(ctx context.Context, tenantID, userID string) (*domain.Staff, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Staff, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Staff) error
	Delete(ctx context.Context, tenantID, id string) error
}

// AssignmentRepository persists teacher-to-class/subject scope and commits the
// staff.assigned event in the same transaction as assignment creation.
type AssignmentRepository interface {
	CreateAssignment(context.Context, string, *domain.Assignment, map[string]any) error
	ListAssignments(context.Context, string, string, int, string) ([]*domain.Assignment, string, error)
	DeleteAssignment(context.Context, string, string, string) error
	ListAssignmentClassIDs(context.Context, string, string) ([]string, error)
	ListAssignmentSubjectIDs(context.Context, string, string) ([]string, error)
}

// AssignmentEventData returns the canonical staff.assigned.v1 payload.
func AssignmentEventData(assignment *domain.Assignment) map[string]any {
	return map[string]any{
		"staff_id":   assignment.StaffID,
		"class_id":   assignment.ClassID,
		"subject_id": assignment.SubjectID,
		"role":       assignment.Role,
	}
}
