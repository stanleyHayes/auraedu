// Package ports defines the inbound and outbound ports for the CBT service.
package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/cbt-service/internal/domain"
)

const (
	CBTMutationQuestionCreate   = "question_create"
	CBTMutationQuestionUpdate   = "question_update"
	CBTMutationQuestionDelete   = "question_delete"
	CBTMutationExamCreate       = "exam_create"
	CBTMutationExamUpdate       = "exam_update"
	CBTMutationExamDelete       = "exam_delete"
	CBTMutationSubmissionUpdate = "submission_update"
)

type LifecycleMutation struct {
	Kind       string
	Question   *domain.QuestionBank
	Exam       *domain.ExamSession
	Submission *domain.Submission
}

type LifecycleEvent struct {
	EventType string
	Payload   map[string]any
}

type LifecycleRepository interface {
	CommitCBTLifecycle(context.Context, string, LifecycleMutation, []LifecycleEvent) error
}

type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
}

type OutboxRepository interface {
	ClaimPendingCBTEvents(context.Context, int) ([]OutboxEvent, error)
	MarkCBTEventPublished(context.Context, string) error
	MarkCBTEventFailed(context.Context, string, string) error
}

func QuestionEventData(q *domain.QuestionBank, meta map[string]any) map[string]any {
	data := map[string]any{
		"question_id": q.ID, "academic_year_id": q.AcademicYearID,
		"subject_id": q.SubjectID, "question_type": q.QuestionType,
		"marks": q.Marks, "status": q.Status,
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

func ExamEventData(exam *domain.ExamSession, meta map[string]any) map[string]any {
	data := map[string]any{
		"exam_id": exam.ID, "academic_year_id": exam.AcademicYearID,
		"subject_id": exam.SubjectID, "title": exam.Title,
		"question_count":   len(exam.QuestionIDs),
		"duration_minutes": exam.DurationMinutes, "status": exam.Status,
	}
	if exam.StartAt != nil {
		data["start_at"] = exam.StartAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if exam.EndAt != nil {
		data["end_at"] = exam.EndAt.Format("2006-01-02T15:04:05Z07:00")
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

func SubmissionEventData(eventType string, submission *domain.Submission) map[string]any {
	if eventType == "cbt.graded.v1" {
		return map[string]any{
			"submission_id": submission.ID,
			"score":         submission.Score,
			"max_score":     submission.MaxScore,
		}
	}
	data := map[string]any{
		"submission_id": submission.ID,
		"exam_id":       submission.ExamSessionID,
		"student_id":    submission.StudentID,
	}
	if submission.SubmittedAt != nil {
		data["submitted_at"] = submission.SubmittedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return data
}

type LearnerScopeResolver interface {
	ResolveStudentIDs(ctx context.Context, tenantID, userID, role string) ([]string, error)
}

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
