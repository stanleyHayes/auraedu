// Package domain contains the CBT aggregates and value objects.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SubmissionStatus enumerates the lifecycle states of a submission.
type SubmissionStatus string

const (
	SubmissionStatusInProgress SubmissionStatus = "in_progress"
	SubmissionStatusSubmitted  SubmissionStatus = "submitted"
	SubmissionStatusGraded     SubmissionStatus = "graded"
)

// Submission is the aggregate root for a student's exam attempt.
// student_id is kept as an opaque UUID.
type Submission struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	ExamSessionID string            `json:"exam_session_id"`
	StudentID     string            `json:"student_id"`
	Answers       map[string]string `json:"answers"`
	Status        string            `json:"status"`
	Score         *int              `json:"score,omitempty"`
	MaxScore      int               `json:"max_score"`
	SubmittedAt   *time.Time        `json:"submitted_at,omitempty"`
	GradedAt      *time.Time        `json:"graded_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     *time.Time        `json:"-"`
}

// NewSubmission constructs a Submission in the in_progress state.
func NewSubmission(tenantID, examSessionID, studentID string) (*Submission, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(examSessionID) == "" {
		return nil, fmt.Errorf("%w: exam_session_id is required", ErrValidation)
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("submission: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Submission{
		ID:            id.String(),
		TenantID:      tenantID,
		ExamSessionID: strings.TrimSpace(examSessionID),
		StudentID:     strings.TrimSpace(studentID),
		Answers:       make(map[string]string),
		Status:        string(SubmissionStatusInProgress),
		MaxScore:      0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (s Submission) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.ExamSessionID) == "" {
		return fmt.Errorf("%w: exam_session_id is required", ErrValidation)
	}
	if strings.TrimSpace(s.StudentID) == "" {
		return fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if s.Answers == nil {
		return fmt.Errorf("%w: answers map is required", ErrValidation)
	}
	if !isValidSubmissionStatus(SubmissionStatus(s.Status)) {
		return fmt.Errorf("%w: status must be in_progress, submitted or graded", ErrValidation)
	}
	if s.Status == string(SubmissionStatusGraded) && s.Score == nil {
		return fmt.Errorf("%w: graded submissions require a score", ErrValidation)
	}
	if s.MaxScore < 0 {
		return fmt.Errorf("%w: max_score cannot be negative", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the submission with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a value is invalid.
func (s *Submission) ApplyUpdate(answers map[string]string, status *string, score, maxScore *int) ([]string, error) {
	var changed []string

	if answers != nil {
		s.Answers = make(map[string]string, len(answers))
		for k, v := range answers {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			s.Answers[k] = strings.TrimSpace(v)
		}
		changed = append(changed, "answers")
	}
	if maxScore != nil {
		if *maxScore < 0 {
			return nil, fmt.Errorf("%w: max_score cannot be negative", ErrValidation)
		}
		s.MaxScore = *maxScore
		changed = append(changed, "max_score")
	}
	if score != nil {
		if *score < 0 {
			return nil, fmt.Errorf("%w: score cannot be negative", ErrValidation)
		}
		if *score > s.MaxScore {
			return nil, fmt.Errorf("%w: score %d exceeds max_score %d", ErrValidation, *score, s.MaxScore)
		}
		s.Score = score
		changed = append(changed, "score")
	}
	if status != nil {
		st := SubmissionStatus(strings.TrimSpace(*status))
		if !isValidSubmissionStatus(st) {
			return nil, fmt.Errorf("%w: status must be in_progress, submitted or graded", ErrValidation)
		}
		if !isValidSubmissionStatusTransition(SubmissionStatus(s.Status), st) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, s.Status, *status)
		}
		s.Status = string(st)
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// Submit transitions the submission to submitted, storing the final answers.
func (s *Submission) Submit(answers map[string]string) error {
	now := time.Now().UTC()
	changed, err := s.ApplyUpdate(answers, strPtr(string(SubmissionStatusSubmitted)), nil, nil)
	if err != nil {
		return err
	}
	_ = changed
	s.SubmittedAt = &now
	return nil
}

// Grade transitions the submission to graded, setting score and max_score.
func (s *Submission) Grade(score, maxScore int) error {
	now := time.Now().UTC()
	changed, err := s.ApplyUpdate(nil, strPtr(string(SubmissionStatusGraded)), &score, &maxScore)
	if err != nil {
		return err
	}
	_ = changed
	s.GradedAt = &now
	return nil
}

func isValidSubmissionStatus(s SubmissionStatus) bool {
	switch s {
	case SubmissionStatusInProgress, SubmissionStatusSubmitted, SubmissionStatusGraded:
		return true
	}
	return false
}

func isValidSubmissionStatusTransition(from, to SubmissionStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case SubmissionStatusInProgress:
		return to == SubmissionStatusSubmitted
	case SubmissionStatusSubmitted:
		return to == SubmissionStatusGraded
	case SubmissionStatusGraded:
		return false
	}
	return false
}

func strPtr(s string) *string { return &s }
