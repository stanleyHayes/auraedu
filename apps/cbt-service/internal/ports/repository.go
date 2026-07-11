package ports

import (
	"context"

	"github.com/auraedu/cbt-service/internal/domain"
)

// Repository persists QuestionBank, ExamSession and Submission aggregates.
// Implementations MUST scope every query by tenantID (defense-in-depth with
// Postgres RLS, agent_plan §7).
type Repository interface {
	// Question banks.
	CreateQuestion(ctx context.Context, tenantID string, q *domain.QuestionBank) error
	GetQuestionByID(ctx context.Context, tenantID, id string) (*domain.QuestionBank, error)
	ListQuestions(ctx context.Context, tenantID string, filter QuestionListFilter) ([]*domain.QuestionBank, string, error)
	UpdateQuestion(ctx context.Context, tenantID string, q *domain.QuestionBank) error
	DeleteQuestion(ctx context.Context, tenantID, id string) error

	// Exam sessions.
	CreateExamSession(ctx context.Context, tenantID string, e *domain.ExamSession) error
	GetExamSessionByID(ctx context.Context, tenantID, id string) (*domain.ExamSession, error)
	ListExamSessions(ctx context.Context, tenantID string, filter ExamSessionListFilter) ([]*domain.ExamSession, string, error)
	UpdateExamSession(ctx context.Context, tenantID string, e *domain.ExamSession) error
	DeleteExamSession(ctx context.Context, tenantID, id string) error

	// Submissions.
	CreateSubmission(ctx context.Context, tenantID string, s *domain.Submission) error
	GetSubmissionByID(ctx context.Context, tenantID, id string) (*domain.Submission, error)
	ListSubmissions(ctx context.Context, tenantID string, filter SubmissionListFilter) ([]*domain.Submission, string, error)
	GetSubmissionByExamAndStudent(ctx context.Context, tenantID, examSessionID, studentID string) (*domain.Submission, error)
	UpdateSubmission(ctx context.Context, tenantID string, s *domain.Submission) error
	DeleteSubmission(ctx context.Context, tenantID, id string) error
}

// QuestionListFilter carries cursor pagination and optional equality filters.
type QuestionListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	SubjectID      string
	Status         string
}

// ExamSessionListFilter carries cursor pagination and optional equality filters.
type ExamSessionListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	SubjectID      string
	Status         string
}

// SubmissionListFilter carries cursor pagination and optional equality filters.
type SubmissionListFilter struct {
	Limit         int
	Cursor        string
	ExamSessionID string
	StudentID     string
	Status        string
}
