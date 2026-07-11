package ports

import (
	"context"

	"github.com/auraedu/cbt-service/internal/domain"
)

// EventPublisher emits CBT domain events.
type EventPublisher interface {
	// PublishQuestionBank emits an event about a QuestionBank aggregate.
	PublishQuestionBank(ctx context.Context, eventType string, q *domain.QuestionBank, meta map[string]any) error
	// PublishExamSession emits an event about an ExamSession aggregate.
	PublishExamSession(ctx context.Context, eventType string, e *domain.ExamSession, meta map[string]any) error
	// PublishSubmission emits an event about a Submission aggregate.
	PublishSubmission(ctx context.Context, eventType string, s *domain.Submission, meta map[string]any) error
}
