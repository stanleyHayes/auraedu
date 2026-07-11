// Package events adapts the platform eventbus to the CBT service publisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the CBT service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishQuestionBank emits a CloudEvent for the given question bank domain event.
func (p *Publisher) PublishQuestionBank(ctx context.Context, eventType string, q *domain.QuestionBank, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"question_id":      q.ID,
		"academic_year_id": q.AcademicYearID,
		"subject_id":       q.SubjectID,
		"question_type":    q.QuestionType,
		"marks":            q.Marks,
		"status":           q.Status,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", q.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build question event: %w", err)
	}
	event.Subject = q.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishExamSession emits a CloudEvent for the given exam session domain event.
func (p *Publisher) PublishExamSession(ctx context.Context, eventType string, e *domain.ExamSession, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"exam_id":          e.ID,
		"academic_year_id": e.AcademicYearID,
		"subject_id":       e.SubjectID,
		"title":            e.Title,
		"question_count":   len(e.QuestionIDs),
		"duration_minutes": e.DurationMinutes,
		"status":           e.Status,
	}
	if e.StartAt != nil {
		data["start_at"] = e.StartAt.Format(time.RFC3339)
	}
	if e.EndAt != nil {
		data["end_at"] = e.EndAt.Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", e.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build exam event: %w", err)
	}
	event.Subject = e.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishSubmission emits a CloudEvent for the given submission domain event.
func (p *Publisher) PublishSubmission(ctx context.Context, eventType string, s *domain.Submission, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"submission_id": s.ID,
		"exam_id":       s.ExamSessionID,
		"student_id":    s.StudentID,
		"status":        s.Status,
		"score":         s.Score,
		"max_score":     s.MaxScore,
	}
	if s.SubmittedAt != nil {
		data["submitted_at"] = s.SubmittedAt.Format(time.RFC3339)
	}
	if s.GradedAt != nil {
		data["graded_at"] = s.GradedAt.Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", s.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build submission event: %w", err)
	}
	event.Subject = s.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
