package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/assessment-service/internal/domain"
)

const (
	AssessmentMutationCreate      = "assessment_create"
	AssessmentMutationUpdate      = "assessment_update"
	AssessmentMutationDelete      = "assessment_delete"
	AssessmentMutationScoreCreate = "score_create"
	AssessmentMutationScoreUpdate = "score_update"
	AssessmentMutationScoreDelete = "score_delete"
)

type LifecycleEvent struct {
	EventType string
	Payload   map[string]any
}
type LifecycleMutation struct {
	Kind       string
	Assessment *domain.Assessment
	Score      *domain.Score
}
type LifecycleRepository interface {
	CommitAssessmentLifecycle(context.Context, string, LifecycleMutation, []LifecycleEvent) error
}
type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
}
type OutboxRepository interface {
	ClaimPendingAssessmentEvents(context.Context, int) ([]OutboxEvent, error)
	MarkAssessmentEventPublished(context.Context, string) error
	MarkAssessmentEventFailed(context.Context, string, string) error
}

func AssessmentEventData(a *domain.Assessment, meta map[string]any) map[string]any {
	data := map[string]any{
		"assessment_id":    a.ID,
		"academic_year_id": a.AcademicYearID,
		"subject_id":       a.SubjectID,
		"type":             a.Type,
		"title":            a.Title,
		"max_score":        a.MaxScore,
		"status":           a.Status,
	}
	if a.Description != nil {
		data["description"] = *a.Description
	}
	if a.DueDate != nil {
		data["due_date"] = a.DueDate.Format("2006-01-02T15:04:05Z07:00")
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
func AssignmentEventData(a *domain.Assessment, meta map[string]any) map[string]any {
	data := map[string]any{"assignment_id": a.ID, "subject_id": a.SubjectID}
	if len(a.ClassIDs) > 0 {
		data["class_ids"] = a.ClassIDs
	}
	if a.DueDate != nil {
		data["due_date"] = a.DueDate.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
func ScoreEventData(s *domain.Score, meta map[string]any) map[string]any {
	data := map[string]any{"score_id": s.ID, "assessment_id": s.AssessmentID, "student_id": s.StudentID, "score": s.Score, "recorded_by": s.RecordedBy}
	if s.Notes != nil {
		data["notes"] = *s.Notes
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}

// Repository persists Assessment and Score aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	// Assessments.
	CreateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error
	GetAssessmentByID(ctx context.Context, tenantID, id string) (*domain.Assessment, error)
	ListAssessments(ctx context.Context, tenantID string, filter AssessmentListFilter) ([]*domain.Assessment, string, error)
	UpdateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error
	DeleteAssessment(ctx context.Context, tenantID, id string) error

	// Scores.
	CreateScore(ctx context.Context, tenantID string, s *domain.Score) error
	GetScoreByID(ctx context.Context, tenantID, assessmentID, scoreID string) (*domain.Score, error)
	ListScores(ctx context.Context, tenantID, assessmentID string, filter ScoreListFilter) ([]*domain.Score, string, error)
	UpdateScore(ctx context.Context, tenantID string, s *domain.Score) error
	DeleteScore(ctx context.Context, tenantID, assessmentID, scoreID string) error

	// Assignments (assessments with type='assignment').
	ListAssignments(ctx context.Context, tenantID string, filter AssignmentListFilter) ([]*domain.Assessment, string, error)

	// Gradebook.
	GradebookScores(ctx context.Context, tenantID string, filter GradebookFilter) ([]domain.GradeRow, error)
}
type LearnerScopeResolver interface {
	Resolve(context.Context, string, string, string) (LearnerScope, error)
}

type LearnerScope struct {
	StudentIDs []string
	ClassIDs   []string
}

// AssessmentListFilter carries cursor pagination and optional equality filters.
type AssessmentListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	SubjectID      string
	Type           string
	Status         string
	ClassIDs       []string
}

// ScoreListFilter carries cursor pagination and optional equality filters.
type ScoreListFilter struct {
	Limit      int
	Cursor     string
	StudentID  string
	StudentIDs []string
}

// AssignmentListFilter carries cursor pagination and optional equality filters.
// StudentID restricts to assignments that have a recorded score for that student.
type AssignmentListFilter struct {
	Limit     int
	Cursor    string
	SubjectID string
	ClassID   string
	ClassIDs  []string
	StudentID string
	Status    string
}

// GradebookFilter scopes a gradebook query. At least one of StudentID or
// ClassID must be set.
type GradebookFilter struct {
	StudentID      string
	ClassID        string
	AcademicYearID string
	SubjectID      string
}
