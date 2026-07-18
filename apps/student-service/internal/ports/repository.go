// Package ports defines the student service repository boundary.
package ports

import (
	"context"

	"github.com/auraedu/student-service/internal/domain"
)

// Repository persists Student aggregates and Guardian links. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	// Students
	Create(ctx context.Context, tenantID string, s *domain.Student) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error)
	// List returns a page of students. When classID is non-nil, only students
	// assigned to that class are returned (roster view, AURA-10.11).
	List(ctx context.Context, tenantID string, classID *string, limit int, cursor string) ([]*domain.Student, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Student) error
	Delete(ctx context.Context, tenantID, id string) error

	// Guardians
	CreateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error
	GetGuardianByID(ctx context.Context, tenantID, id string) (*domain.Guardian, error)
	ListGuardiansByStudent(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Guardian, string, error)
	UpdateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error
	DeleteGuardian(ctx context.Context, tenantID, id string) error

	// Student ↔ Guardian links
	LinkGuardianToStudent(ctx context.Context, tenantID string, link *domain.StudentGuardian) error
	UnlinkGuardianFromStudent(ctx context.Context, tenantID, studentID, guardianID string) error
}
