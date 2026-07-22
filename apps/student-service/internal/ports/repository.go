// Package ports defines the student service repository boundary.
package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/student-service/internal/domain"
)

const (
	MutationStudentCreate    = "student_create"
	MutationStudentUpdate    = "student_update"
	MutationStudentDelete    = "student_delete"
	MutationEnrollmentCreate = "enrollment_create"
	MutationGuardianCreate   = "guardian_create"
	MutationGuardianUpdate   = "guardian_update"
	MutationGuardianDelete   = "guardian_delete"
	MutationGuardianLink     = "guardian_link"
	MutationGuardianUnlink   = "guardian_unlink"
)

// LifecycleMutation describes one state transition committed with one event.
type LifecycleMutation struct {
	Kind       string
	Student    *domain.Student
	Enrollment *domain.Enrollment
	Guardian   *domain.Guardian
	Link       *domain.StudentGuardian
	StudentID  string
	GuardianID string
}

type LifecycleRepository interface {
	CommitStudentLifecycle(context.Context, string, LifecycleMutation, string, map[string]any) error
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
}

type OutboxRepository interface {
	ClaimPendingStudentEvents(context.Context, int) ([]OutboxEvent, error)
	MarkStudentEventPublished(context.Context, string) error
	MarkStudentEventFailed(context.Context, string, string) error
}

func StudentEventData(student *domain.Student, meta map[string]any) map[string]any {
	data := map[string]any{}
	if student != nil {
		data["student_id"] = student.ID
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

// Repository persists Student aggregates and Guardian links. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	// Students
	Create(ctx context.Context, tenantID string, s *domain.Student) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error)
	// GetStudentByUserID returns the student linked to the given identity user
	// (AURA-10.12), or domain.ErrNotFound when no record is linked.
	GetStudentByUserID(ctx context.Context, tenantID, userID string) (*domain.Student, error)
	// List returns a page of students. When classID is non-nil, only students
	// assigned to that class are returned (roster view, AURA-10.11).
	List(ctx context.Context, tenantID string, classID *string, limit int, cursor string) ([]*domain.Student, string, error)
	ListStudentIDsByClassIDs(ctx context.Context, tenantID string, classIDs []string) ([]string, error)
	Update(ctx context.Context, tenantID string, s *domain.Student) error
	Delete(ctx context.Context, tenantID, id string) error
	CreateEnrollment(ctx context.Context, tenantID string, enrollment *domain.Enrollment) error
	ListEnrollments(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Enrollment, string, error)

	// Guardians
	CreateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error
	GetGuardianByID(ctx context.Context, tenantID, id string) (*domain.Guardian, error)
	// GetGuardianByUserID returns the guardian linked to the given identity user
	// (AURA-10.12), or domain.ErrNotFound when no record is linked.
	GetGuardianByUserID(ctx context.Context, tenantID, userID string) (*domain.Guardian, error)
	// ListStudentsByGuardian returns the students linked to a guardian through the
	// student_guardians link table (a parent's children, AURA-10.12).
	ListStudentsByGuardian(ctx context.Context, tenantID, guardianID string) ([]*domain.Student, error)
	ListGuardiansByStudent(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Guardian, string, error)
	UpdateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error
	DeleteGuardian(ctx context.Context, tenantID, id string) error

	// Student ↔ Guardian links
	LinkGuardianToStudent(ctx context.Context, tenantID string, link *domain.StudentGuardian) error
	UnlinkGuardianFromStudent(ctx context.Context, tenantID, studentID, guardianID string) error
}

// TeacherClassScopeResolver resolves the classes assigned to a teacher identity.
type TeacherClassScopeResolver interface {
	ResolveTeacherClasses(context.Context, string, string) ([]string, error)
}
