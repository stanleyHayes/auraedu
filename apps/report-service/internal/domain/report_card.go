package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReportCardStatus enumerates the lifecycle states of a report card.
type ReportCardStatus string

const (
	ReportCardStatusDraft      ReportCardStatus = "draft"
	ReportCardStatusGenerating ReportCardStatus = "generating"
	ReportCardStatusPublished  ReportCardStatus = "published"
	ReportCardStatusArchived   ReportCardStatus = "archived"
)

// ReportCard is the aggregate root for a student report card.
// student_id, academic_year_id, term_id and template_id are kept as opaque UUIDs
// to avoid coupling this service to student/academic-year/template lifecycles.
// AcademicYearID, TermID and TemplateID may be empty on DRAFT cards auto-created
// by the event worker before a year/template is assigned through the API.
type ReportCard struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	StudentID      string     `json:"student_id"`
	AcademicYearID string     `json:"academic_year_id,omitempty"`
	TermID         string     `json:"term_id,omitempty"`
	TemplateID     string     `json:"template_id,omitempty"`
	Status         string     `json:"status"`
	PDFPath        *string    `json:"-"` // internal storage key; never expose through REST
	GeneratedAt    *time.Time `json:"generated_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// NewReportCard constructs a ReportCard, enforcing invariants.
func NewReportCard(tenantID, studentID, academicYearID, templateID string) (*ReportCard, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(templateID) == "" {
		return nil, fmt.Errorf("%w: template_id is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("report: generate report card id: %w", err)
	}
	now := time.Now().UTC()
	return &ReportCard{
		ID:             id.String(),
		TenantID:       tenantID,
		StudentID:      strings.TrimSpace(studentID),
		AcademicYearID: strings.TrimSpace(academicYearID),
		TemplateID:     strings.TrimSpace(templateID),
		Status:         string(ReportCardStatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// NewEventDraftReportCard constructs a DRAFT report card from event data, where
// the academic year and template may not be known yet. Used by the event worker
// when a score/attendance event arrives for a student with no draft card.
func NewEventDraftReportCard(tenantID, studentID, academicYearID, termID string) (*ReportCard, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("report: generate report card id: %w", err)
	}
	now := time.Now().UTC()
	return &ReportCard{
		ID:             id.String(),
		TenantID:       tenantID,
		StudentID:      strings.TrimSpace(studentID),
		AcademicYearID: strings.TrimSpace(academicYearID),
		TermID:         strings.TrimSpace(termID),
		Status:         string(ReportCardStatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed. academic_year_id and
// template_id are optional: event-created drafts legitimately lack them until
// they are assigned through the API.
func (c ReportCard) Validate() error {
	if c.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(c.StudentID) == "" {
		return fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if !isValidReportCardStatus(ReportCardStatus(c.Status)) {
		return fmt.Errorf("%w: status must be draft, generating, published or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the report card with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (c *ReportCard) ApplyUpdate(studentID, academicYearID, templateID, status *string) ([]string, error) {
	var changed []string

	if studentID != nil {
		if strings.TrimSpace(*studentID) == "" {
			return nil, fmt.Errorf("%w: student_id cannot be empty", ErrValidation)
		}
		c.StudentID = strings.TrimSpace(*studentID)
		changed = append(changed, "student_id")
	}
	if academicYearID != nil {
		if strings.TrimSpace(*academicYearID) == "" {
			return nil, fmt.Errorf("%w: academic_year_id cannot be empty", ErrValidation)
		}
		c.AcademicYearID = strings.TrimSpace(*academicYearID)
		changed = append(changed, "academic_year_id")
	}
	if templateID != nil {
		if strings.TrimSpace(*templateID) == "" {
			return nil, fmt.Errorf("%w: template_id cannot be empty", ErrValidation)
		}
		c.TemplateID = strings.TrimSpace(*templateID)
		changed = append(changed, "template_id")
	}
	if status != nil {
		s := ReportCardStatus(strings.TrimSpace(*status))
		if !isValidReportCardStatus(s) {
			return nil, fmt.Errorf("%w: status must be draft, generating, published or archived", ErrValidation)
		}
		if !isValidReportCardStatusTransition(ReportCardStatus(c.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, c.Status, *status)
		}
		c.Status = string(s)
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		c.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// SetPublished marks the report card as published with a PDF path and timestamp.
func (c *ReportCard) SetPublished(pdfPath string) {
	now := time.Now().UTC()
	c.PDFPath = &pdfPath
	c.GeneratedAt = &now
	c.Status = string(ReportCardStatusPublished)
	c.UpdatedAt = now
}

// SetGenerating marks the report card as generating.
func (c *ReportCard) SetGenerating() {
	c.Status = string(ReportCardStatusGenerating)
	c.UpdatedAt = time.Now().UTC()
}

func isValidReportCardStatus(s ReportCardStatus) bool {
	switch s {
	case ReportCardStatusDraft, ReportCardStatusGenerating, ReportCardStatusPublished, ReportCardStatusArchived:
		return true
	}
	return false
}

// isValidReportCardStatusTransition defines the allowed report card lifecycle transitions.
func isValidReportCardStatusTransition(from, to ReportCardStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case ReportCardStatusDraft:
		return to == ReportCardStatusGenerating || to == ReportCardStatusArchived
	case ReportCardStatusGenerating:
		return to == ReportCardStatusPublished || to == ReportCardStatusDraft
	case ReportCardStatusPublished:
		return to == ReportCardStatusArchived || to == ReportCardStatusDraft
	case ReportCardStatusArchived:
		return to == ReportCardStatusDraft
	}
	return false
}
