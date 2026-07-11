package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QuestionType enumerates the supported question formats.
type QuestionType string

const (
	TypeMultipleChoice QuestionType = "multiple_choice"
	TypeTrueFalse      QuestionType = "true_false"
	TypeShortAnswer    QuestionType = "short_answer"
)

// QuestionStatus enumerates the lifecycle states of a question.
type QuestionStatus string

const (
	QuestionStatusDraft     QuestionStatus = "draft"
	QuestionStatusPublished QuestionStatus = "published"
	QuestionStatusArchived  QuestionStatus = "archived"
)

// QuestionBank is the aggregate root for a single exam question.
// academic_year_id and subject_id are kept as opaque UUIDs to avoid coupling
// this service to academic-year/subject lifecycle details.
type QuestionBank struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	AcademicYearID string     `json:"academic_year_id"`
	SubjectID      string     `json:"subject_id"`
	QuestionText   string     `json:"question_text"`
	QuestionType   string     `json:"question_type"`
	Options        []string   `json:"options,omitempty"`
	CorrectAnswer  string     `json:"correct_answer"`
	Marks          int        `json:"marks"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// NewQuestionBank constructs a QuestionBank, enforcing invariants.
func NewQuestionBank(tenantID, academicYearID, subjectID, questionText, questionType, correctAnswer string, marks int, options []string) (*QuestionBank, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(subjectID) == "" {
		return nil, fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if strings.TrimSpace(questionText) == "" {
		return nil, fmt.Errorf("%w: question_text is required", ErrValidation)
	}
	qType := QuestionType(strings.TrimSpace(questionType))
	if !isValidQuestionType(qType) {
		return nil, fmt.Errorf("%w: question_type must be multiple_choice, true_false or short_answer", ErrValidation)
	}
	if strings.TrimSpace(correctAnswer) == "" {
		return nil, fmt.Errorf("%w: correct_answer is required", ErrValidation)
	}
	if marks <= 0 {
		return nil, fmt.Errorf("%w: marks must be greater than 0", ErrValidation)
	}
	if err := validateOptionsForType(qType, options); err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("question_bank: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &QuestionBank{
		ID:             id.String(),
		TenantID:       tenantID,
		AcademicYearID: strings.TrimSpace(academicYearID),
		SubjectID:      strings.TrimSpace(subjectID),
		QuestionText:   strings.TrimSpace(questionText),
		QuestionType:   string(qType),
		Options:        normalizeOptions(options),
		CorrectAnswer:  strings.TrimSpace(correctAnswer),
		Marks:          marks,
		Status:         string(QuestionStatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (q QuestionBank) Validate() error {
	if q.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(q.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(q.SubjectID) == "" {
		return fmt.Errorf("%w: subject_id is required", ErrValidation)
	}
	if strings.TrimSpace(q.QuestionText) == "" {
		return fmt.Errorf("%w: question_text is required", ErrValidation)
	}
	qType := QuestionType(q.QuestionType)
	if !isValidQuestionType(qType) {
		return fmt.Errorf("%w: question_type must be multiple_choice, true_false or short_answer", ErrValidation)
	}
	if strings.TrimSpace(q.CorrectAnswer) == "" {
		return fmt.Errorf("%w: correct_answer is required", ErrValidation)
	}
	if q.Marks <= 0 {
		return fmt.Errorf("%w: marks must be greater than 0", ErrValidation)
	}
	if !isValidQuestionStatus(QuestionStatus(q.Status)) {
		return fmt.Errorf("%w: status must be draft, published or archived", ErrValidation)
	}
	if err := validateOptionsForType(qType, q.Options); err != nil {
		return err
	}
	return nil
}

// ApplyUpdate mutates the question with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (q *QuestionBank) ApplyUpdate(questionText, questionType, correctAnswer *string, marks *int, options []string, status *string) ([]string, error) {
	var changed []string

	if questionText != nil {
		if strings.TrimSpace(*questionText) == "" {
			return nil, fmt.Errorf("%w: question_text cannot be empty", ErrValidation)
		}
		q.QuestionText = strings.TrimSpace(*questionText)
		changed = append(changed, "question_text")
	}
	if questionType != nil {
		qType := QuestionType(strings.TrimSpace(*questionType))
		if !isValidQuestionType(qType) {
			return nil, fmt.Errorf("%w: question_type must be multiple_choice, true_false or short_answer", ErrValidation)
		}
		if err := validateOptionsForType(qType, options); err != nil && len(options) > 0 {
			return nil, err
		}
		q.QuestionType = string(qType)
		changed = append(changed, "question_type")
	}
	if options != nil {
		if err := validateOptionsForType(QuestionType(q.QuestionType), options); err != nil {
			return nil, err
		}
		q.Options = normalizeOptions(options)
		changed = append(changed, "options")
	}
	if correctAnswer != nil {
		if strings.TrimSpace(*correctAnswer) == "" {
			return nil, fmt.Errorf("%w: correct_answer cannot be empty", ErrValidation)
		}
		q.CorrectAnswer = strings.TrimSpace(*correctAnswer)
		changed = append(changed, "correct_answer")
	}
	if marks != nil {
		if *marks <= 0 {
			return nil, fmt.Errorf("%w: marks must be greater than 0", ErrValidation)
		}
		q.Marks = *marks
		changed = append(changed, "marks")
	}
	if status != nil {
		s := QuestionStatus(strings.TrimSpace(*status))
		if !isValidQuestionStatus(s) {
			return nil, fmt.Errorf("%w: status must be draft, published or archived", ErrValidation)
		}
		if !isValidQuestionStatusTransition(QuestionStatus(q.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, q.Status, *status)
		}
		q.Status = string(s)
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		q.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidQuestionType(t QuestionType) bool {
	switch t {
	case TypeMultipleChoice, TypeTrueFalse, TypeShortAnswer:
		return true
	}
	return false
}

func isValidQuestionStatus(s QuestionStatus) bool {
	switch s {
	case QuestionStatusDraft, QuestionStatusPublished, QuestionStatusArchived:
		return true
	}
	return false
}

func isValidQuestionStatusTransition(from, to QuestionStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case QuestionStatusDraft:
		return to == QuestionStatusPublished || to == QuestionStatusArchived
	case QuestionStatusPublished:
		return to == QuestionStatusArchived || to == QuestionStatusDraft
	case QuestionStatusArchived:
		return to == QuestionStatusDraft
	}
	return false
}

func validateOptionsForType(t QuestionType, options []string) error {
	switch t {
	case TypeMultipleChoice:
		if len(options) < 2 {
			return fmt.Errorf("%w: multiple_choice questions require at least 2 options", ErrValidation)
		}
	case TypeTrueFalse:
		if len(options) != 2 {
			return fmt.Errorf("%w: true_false questions require exactly 2 options", ErrValidation)
		}
	case TypeShortAnswer:
		if len(options) != 0 {
			return fmt.Errorf("%w: short_answer questions must not have options", ErrValidation)
		}
	}
	return nil
}

func normalizeOptions(opts []string) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, strings.TrimSpace(o))
	}
	return out
}
