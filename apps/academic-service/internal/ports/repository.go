package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/academic-service/internal/domain"
)

const (
	AcademicMutationYearCreate    = "year_create"
	AcademicMutationYearUpdate    = "year_update"
	AcademicMutationYearDelete    = "year_delete"
	AcademicMutationTermUpdate    = "term_update"
	AcademicMutationTermDelete    = "term_delete"
	AcademicMutationClassCreate   = "class_create"
	AcademicMutationClassUpdate   = "class_update"
	AcademicMutationClassDelete   = "class_delete"
	AcademicMutationSubjectCreate = "subject_create"
	AcademicMutationSubjectUpdate = "subject_update"
	AcademicMutationSubjectDelete = "subject_delete"
)

type AcademicMutation struct {
	Kind    string
	Year    *domain.AcademicYear
	Term    *domain.Term
	Class   *domain.Class
	Subject *domain.Subject
}
type LifecycleRepository interface {
	CommitAcademicLifecycle(context.Context, string, AcademicMutation, string, map[string]any) error
}
type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
}
type OutboxRepository interface {
	ClaimPendingAcademicEvents(context.Context, int) ([]OutboxEvent, error)
	MarkAcademicEventPublished(context.Context, string) error
	MarkAcademicEventFailed(context.Context, string, string) error
}

func YearEventData(y *domain.AcademicYear, meta map[string]any) map[string]any {
	data := map[string]any{"year_id": y.ID, "name": y.Name, "start_date": y.StartDate, "end_date": y.EndDate}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
func TermEventData(t *domain.Term, meta map[string]any) map[string]any {
	data := map[string]any{"term_id": t.ID, "academic_year_id": t.AcademicYearID, "name": t.Name, "start_date": t.StartDate, "end_date": t.EndDate}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
func ClassEventData(c *domain.Class, meta map[string]any) map[string]any {
	data := map[string]any{"class_id": c.ID, "name": c.Name, "academic_year_id": c.AcademicYearID}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
func SubjectEventData(s *domain.Subject, meta map[string]any) map[string]any {
	data := map[string]any{"subject_id": s.ID, "name": s.Name, "code": s.Code}
	for k, v := range meta {
		data[k] = v
	}
	return data
}

// AcademicYearRepository persists AcademicYear aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type AcademicYearRepository interface {
	Create(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.AcademicYear, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AcademicYear, string, error)
	Update(ctx context.Context, tenantID string, y *domain.AcademicYear) error
	Delete(ctx context.Context, tenantID, id string) error
}

// TermRepository persists Term aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type TermRepository interface {
	Create(ctx context.Context, tenantID string, t *domain.Term) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Term, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Term, string, error)
	Update(ctx context.Context, tenantID string, t *domain.Term) error
	Delete(ctx context.Context, tenantID, id string) error
}

// ClassRepository persists Class aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type ClassRepository interface {
	Create(ctx context.Context, tenantID string, c *domain.Class) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Class, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Class, string, error)
	ListIDsByTeacher(ctx context.Context, tenantID, staffID string) ([]string, error)
	Update(ctx context.Context, tenantID string, c *domain.Class) error
	Delete(ctx context.Context, tenantID, id string) error
}

// TeacherIdentityResolver maps an identity user to an active teacher staff record.
type TeacherIdentityResolver interface {
	ResolveTeacher(context.Context, string, string) (string, error)
}

// TeacherAssignmentResolver exposes explicit staff-owned class assignments.
// It is optional so in-process test resolvers and legacy adapters remain valid.
type TeacherAssignmentResolver interface {
	ResolveTeacherAssignments(context.Context, string, string) (string, []string, error)
}

// SubjectRepository persists Subject aggregates. Implementations MUST scope every query
// by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type SubjectRepository interface {
	Create(ctx context.Context, tenantID string, s *domain.Subject) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Subject, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Subject, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Subject) error
	Delete(ctx context.Context, tenantID, id string) error
}

// GradingScaleRepository persists tenant-owned grading policies.
type GradingScaleRepository interface {
	Create(context.Context, string, *domain.GradingScale) error
	GetByID(context.Context, string, string) (*domain.GradingScale, error)
	List(context.Context, string, int, string) ([]*domain.GradingScale, string, error)
	Update(context.Context, string, *domain.GradingScale) error
	Delete(context.Context, string, string) error
}

type TimetableFilter struct {
	ClassIDs []string
	TermID   string
	Weekday  int
	Status   string
	Limit    int
}

// TimetableRepository persists conflict-protected lesson periods.
type TimetableRepository interface {
	Create(context.Context, string, *domain.TimetableEntry) error
	GetByID(context.Context, string, string) (*domain.TimetableEntry, error)
	List(context.Context, string, TimetableFilter) ([]*domain.TimetableEntry, error)
	Update(context.Context, string, *domain.TimetableEntry) error
	Delete(context.Context, string, string) error
}

// LearnerScopeResolver maps a student or parent identity to current class IDs.
type LearnerScopeResolver interface {
	Resolve(context.Context, string, string, string) ([]string, error)
}
