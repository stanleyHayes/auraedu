package ports

import (
	"context"

	"github.com/auraedu/attendance-service/internal/domain"
)

// Repository persists AttendanceRecord aggregates. Implementations MUST scope every
// query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, r *domain.AttendanceRecord) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.AttendanceRecord, error)
	List(ctx context.Context, tenantID string, filter ListFilter) ([]*domain.AttendanceRecord, string, error)
	Update(ctx context.Context, tenantID string, r *domain.AttendanceRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

// ListFilter carries cursor pagination and optional equality filters for listing.
type ListFilter struct {
	Limit          int
	Cursor         string
	StudentID      string
	AcademicYearID string
	Date           string
	Status         string
}
