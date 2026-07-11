package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExamStatus enumerates the lifecycle states of an exam session.
type ExamStatus string

const (
	ExamStatusDraft     ExamStatus = "draft"
	ExamStatusPublished ExamStatus = "published"
	ExamStatusActive    ExamStatus = "active"
	ExamStatusClosed    ExamStatus = "closed"
	ExamStatusArchived  ExamStatus = "archived"
)

// ExamSession is the aggregate root for a scheduled CBT exam.
// academic_year_id and subject_id are kept as opaque UUIDs.
type ExamSession struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	Title           string     `json:"title"`
	AcademicYearID  string     `json:"academic_year_id"`
	SubjectID       string     `json:"subject_id"`
	QuestionIDs     []string   `json:"question_ids"`
	DurationMinutes int        `json:"duration_minutes"`
	StartAt         *time.Time `json:"start_at,omitempty"`
	EndAt           *time.Time `json:"end_at,omitempty"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`
}

// NewExamSession constructs an ExamSession, enforcing invariants.
func NewExamSession(tenantID, title, academicYearID, subjectID string, questionIDs []string, durationMinutes int, startAt, endAt *time.Time) (*ExamSession, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(subjectID) == "" {
		return nil, fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if len(questionIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one question_id is required", ErrValidation)
	}
	if durationMinutes <= 0 {
		return nil, fmt.Errorf("%w: duration_minutes must be greater than 0", ErrValidation)
	}
	if startAt != nil && endAt != nil && !endAt.After(*startAt) {
		return nil, fmt.Errorf("%w: end_at must be after start_at", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("exam_session: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &ExamSession{
		ID:              id.String(),
		TenantID:        tenantID,
		Title:           strings.TrimSpace(title),
		AcademicYearID:  strings.TrimSpace(academicYearID),
		SubjectID:       strings.TrimSpace(subjectID),
		QuestionIDs:     normalizeIDs(questionIDs),
		DurationMinutes: durationMinutes,
		StartAt:         startAt,
		EndAt:           endAt,
		Status:          string(ExamStatusDraft),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (e ExamSession) Validate() error {
	if e.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(e.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(e.SubjectID) == "" {
		return fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if len(e.QuestionIDs) == 0 {
		return fmt.Errorf("%w: at least one question_id is required", ErrValidation)
	}
	if e.DurationMinutes <= 0 {
		return fmt.Errorf("%w: duration_minutes must be greater than 0", ErrValidation)
	}
	if e.StartAt != nil && e.EndAt != nil && !e.EndAt.After(*e.StartAt) {
		return fmt.Errorf("%w: end_at must be after start_at", ErrValidation)
	}
	if !isValidExamStatus(ExamStatus(e.Status)) {
		return fmt.Errorf("%w: status must be draft, published, active, closed or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the exam session with non-empty patch fields. It returns
// the names of fields that changed, or ErrValidation if a value is invalid.
func (e *ExamSession) ApplyUpdate(title *string, questionIDs []string, durationMinutes *int, startAt, endAt *time.Time, status *string) ([]string, error) {
	var changed []string

	if title != nil {
		if strings.TrimSpace(*title) == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrValidation)
		}
		e.Title = strings.TrimSpace(*title)
		changed = append(changed, "title")
	}
	if questionIDs != nil {
		if len(questionIDs) == 0 {
			return nil, fmt.Errorf("%w: at least one question_id is required", ErrValidation)
		}
		e.QuestionIDs = normalizeIDs(questionIDs)
		changed = append(changed, "question_ids")
	}
	if durationMinutes != nil {
		if *durationMinutes <= 0 {
			return nil, fmt.Errorf("%w: duration_minutes must be greater than 0", ErrValidation)
		}
		e.DurationMinutes = *durationMinutes
		changed = append(changed, "duration_minutes")
	}
	if startAt != nil {
		e.StartAt = startAt
		changed = append(changed, "start_at")
	}
	if endAt != nil {
		e.EndAt = endAt
		changed = append(changed, "end_at")
	}
	if status != nil {
		s := ExamStatus(strings.TrimSpace(*status))
		if !isValidExamStatus(s) {
			return nil, fmt.Errorf("%w: status must be draft, published, active, closed or archived", ErrValidation)
		}
		if !isValidExamStatusTransition(ExamStatus(e.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, e.Status, *status)
		}
		e.Status = string(s)
		changed = append(changed, "status")
	}

	if e.StartAt != nil && e.EndAt != nil && !e.EndAt.After(*e.StartAt) {
		return nil, fmt.Errorf("%w: end_at must be after start_at", ErrValidation)
	}

	if len(changed) > 0 {
		e.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// IsActive reports whether the exam session is currently active: status is active
// and, if window boundaries are set, the current time falls within them.
func (e ExamSession) IsActive(now time.Time) bool {
	if ExamStatus(e.Status) != ExamStatusActive {
		return false
	}
	if e.StartAt != nil && now.Before(*e.StartAt) {
		return false
	}
	if e.EndAt != nil && now.After(*e.EndAt) {
		return false
	}
	return true
}

func isValidExamStatus(s ExamStatus) bool {
	switch s {
	case ExamStatusDraft, ExamStatusPublished, ExamStatusActive, ExamStatusClosed, ExamStatusArchived:
		return true
	}
	return false
}

func isValidExamStatusTransition(from, to ExamStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case ExamStatusDraft:
		return to == ExamStatusPublished || to == ExamStatusActive || to == ExamStatusArchived
	case ExamStatusPublished:
		return to == ExamStatusActive || to == ExamStatusArchived || to == ExamStatusDraft
	case ExamStatusActive:
		return to == ExamStatusClosed || to == ExamStatusPublished
	case ExamStatusClosed:
		return to == ExamStatusPublished || to == ExamStatusArchived || to == ExamStatusDraft
	case ExamStatusArchived:
		return to == ExamStatusDraft
	}
	return false
}

func normalizeIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
