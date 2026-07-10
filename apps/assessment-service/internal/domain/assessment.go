package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AssessmentType enumerates the kinds of assessments.
type AssessmentType string

const (
	TypeAssignment AssessmentType = "assignment"
	TypeTest       AssessmentType = "test"
	TypeExam       AssessmentType = "exam"
)

// AssessmentStatus enumerates the lifecycle states of an assessment.
type AssessmentStatus string

const (
	StatusDraft     AssessmentStatus = "draft"
	StatusPublished AssessmentStatus = "published"
	StatusArchived  AssessmentStatus = "archived"
)

// Assessment is the aggregate root for assignments, tests and exams.
// academic_year_id and subject_id are kept as opaque UUIDs to avoid coupling
// this service to academic-year/subject lifecycle details.
type Assessment struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	AcademicYearID string     `json:"academic_year_id"`
	SubjectID      string     `json:"subject_id"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Description    *string    `json:"description,omitempty"`
	MaxScore       int        `json:"max_score"`
	DueDate        *time.Time `json:"due_date,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// NewAssessment constructs an Assessment, enforcing invariants.
func NewAssessment(tenantID, academicYearID, subjectID, assessmentType, title, description string, maxScore int, dueDate *time.Time) (*Assessment, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(subjectID) == "" {
		return nil, fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if !isValidAssessmentType(AssessmentType(strings.TrimSpace(assessmentType))) {
		return nil, fmt.Errorf("%w: type must be assignment, test or exam", ErrValidation)
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if maxScore <= 0 {
		return nil, fmt.Errorf("%w: max_score must be greater than 0", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("assessment: generate id: %w", err)
	}
	now := time.Now().UTC()
	var desc *string
	if description != "" {
		d := strings.TrimSpace(description)
		desc = &d
	}
	return &Assessment{
		ID:             id.String(),
		TenantID:       tenantID,
		AcademicYearID: strings.TrimSpace(academicYearID),
		SubjectID:      strings.TrimSpace(subjectID),
		Type:           strings.TrimSpace(assessmentType),
		Title:          strings.TrimSpace(title),
		Description:    desc,
		MaxScore:       maxScore,
		DueDate:        dueDate,
		Status:         string(StatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (a Assessment) Validate() error {
	if a.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(a.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(a.SubjectID) == "" {
		return fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if !isValidAssessmentType(AssessmentType(a.Type)) {
		return fmt.Errorf("%w: type must be assignment, test or exam", ErrValidation)
	}
	if strings.TrimSpace(a.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if a.MaxScore <= 0 {
		return fmt.Errorf("%w: max_score must be greater than 0", ErrValidation)
	}
	if !isValidAssessmentStatus(AssessmentStatus(a.Status)) {
		return fmt.Errorf("%w: status must be draft, published or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the assessment with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
// The caller can inspect the returned changed fields to decide whether to emit
// an assessment.published.v1 event when "status" changed to "published".
func (a *Assessment) ApplyUpdate(title, description, assessmentType *string, maxScore *int, dueDate *time.Time, status *string) ([]string, error) {
	var changed []string

	if title != nil {
		if strings.TrimSpace(*title) == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrValidation)
		}
		a.Title = strings.TrimSpace(*title)
		changed = append(changed, "title")
	}
	if description != nil {
		if strings.TrimSpace(*description) == "" {
			a.Description = nil
		} else {
			d := strings.TrimSpace(*description)
			a.Description = &d
		}
		changed = append(changed, "description")
	}
	if assessmentType != nil {
		t := AssessmentType(strings.TrimSpace(*assessmentType))
		if !isValidAssessmentType(t) {
			return nil, fmt.Errorf("%w: type must be assignment, test or exam", ErrValidation)
		}
		a.Type = string(t)
		changed = append(changed, "type")
	}
	if maxScore != nil {
		if *maxScore <= 0 {
			return nil, fmt.Errorf("%w: max_score must be greater than 0", ErrValidation)
		}
		a.MaxScore = *maxScore
		changed = append(changed, "max_score")
	}
	if dueDate != nil {
		a.DueDate = dueDate
		changed = append(changed, "due_date")
	}
	if status != nil {
		s := AssessmentStatus(strings.TrimSpace(*status))
		if !isValidAssessmentStatus(s) {
			return nil, fmt.Errorf("%w: status must be draft, published or archived", ErrValidation)
		}
		if !isValidStatusTransition(AssessmentStatus(a.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, a.Status, *status)
		}
		a.Status = string(s)
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		a.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidAssessmentType(t AssessmentType) bool {
	switch t {
	case TypeAssignment, TypeTest, TypeExam:
		return true
	}
	return false
}

func isValidAssessmentStatus(s AssessmentStatus) bool {
	switch s {
	case StatusDraft, StatusPublished, StatusArchived:
		return true
	}
	return false
}

// isValidStatusTransition defines the allowed assessment lifecycle transitions.
// draft may be published or archived; published may be archived or moved back to
// draft; archived may be moved back to draft. The only disallowed transition is
// an unknown status (handled above).
func isValidStatusTransition(from, to AssessmentStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusDraft:
		return to == StatusPublished || to == StatusArchived
	case StatusPublished:
		return to == StatusArchived || to == StatusDraft
	case StatusArchived:
		return to == StatusDraft
	}
	return false
}
