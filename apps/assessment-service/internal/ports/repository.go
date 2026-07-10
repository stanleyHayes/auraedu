package ports

import (
	"context"

	"github.com/auraedu/assessment-service/internal/domain"
)

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
}

// AssessmentListFilter carries cursor pagination and optional equality filters.
type AssessmentListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	SubjectID      string
	Type           string
	Status         string
}

// ScoreListFilter carries cursor pagination and optional equality filters.
type ScoreListFilter struct {
	Limit     int
	Cursor    string
	StudentID string
}
