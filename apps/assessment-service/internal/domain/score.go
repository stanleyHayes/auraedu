package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Score is the aggregate root for a student's score on an assessment.
// student_id is kept as an opaque UUID to avoid coupling this service to the
// student lifecycle.
type Score struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id"`
	AssessmentID string     `json:"assessment_id"`
	StudentID    string     `json:"student_id"`
	Score        int        `json:"score"`
	RecordedBy   string     `json:"recorded_by"`
	Notes        *string    `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"-"`
}

// NewScore constructs a Score, enforcing invariants.
func NewScore(tenantID, assessmentID, studentID string, score int, recordedBy, notes string, maxScore int) (*Score, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(assessmentID) == "" {
		return nil, fmt.Errorf("%w: assessment_id is required", ErrValidation)
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(recordedBy) == "" {
		return nil, fmt.Errorf("%w: recorded_by is required", ErrValidation)
	}
	if score < 0 {
		return nil, fmt.Errorf("%w: score cannot be negative", ErrValidation)
	}
	if score > maxScore {
		return nil, fmt.Errorf("%w: score %d exceeds assessment max_score %d", ErrValidation, score, maxScore)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("score: generate id: %w", err)
	}
	now := time.Now().UTC()
	var n *string
	if notes != "" {
		nn := strings.TrimSpace(notes)
		n = &nn
	}
	return &Score{
		ID:           id.String(),
		TenantID:     tenantID,
		AssessmentID: strings.TrimSpace(assessmentID),
		StudentID:    strings.TrimSpace(studentID),
		Score:        score,
		RecordedBy:   strings.TrimSpace(recordedBy),
		Notes:        n,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (s Score) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.AssessmentID) == "" {
		return fmt.Errorf("%w: assessment_id is required", ErrValidation)
	}
	if strings.TrimSpace(s.StudentID) == "" {
		return fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(s.RecordedBy) == "" {
		return fmt.Errorf("%w: recorded_by is required", ErrValidation)
	}
	if s.Score < 0 {
		return fmt.Errorf("%w: score cannot be negative", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the score with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
// maxScore is the current assessment max_score and is used to validate a new score.
func (s *Score) ApplyUpdate(score *int, notes *string, maxScore int) ([]string, error) {
	var changed []string

	if score != nil {
		if *score < 0 {
			return nil, fmt.Errorf("%w: score cannot be negative", ErrValidation)
		}
		if *score > maxScore {
			return nil, fmt.Errorf("%w: score %d exceeds assessment max_score %d", ErrValidation, *score, maxScore)
		}
		s.Score = *score
		changed = append(changed, "score")
	}
	if notes != nil {
		if strings.TrimSpace(*notes) == "" {
			s.Notes = nil
		} else {
			nn := strings.TrimSpace(*notes)
			s.Notes = &nn
		}
		changed = append(changed, "notes")
	}

	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}
