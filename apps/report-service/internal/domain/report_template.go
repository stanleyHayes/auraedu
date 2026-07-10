package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TemplateStatus enumerates the lifecycle states of a report template.
type TemplateStatus string

const (
	TemplateStatusDraft    TemplateStatus = "draft"
	TemplateStatusActive   TemplateStatus = "active"
	TemplateStatusArchived TemplateStatus = "archived"
)

// ReportTemplate is the aggregate root for a reusable report card template.
// academic_year_id is kept as an opaque UUID to avoid coupling this service
// to the academic-year lifecycle.
type ReportTemplate struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Name           string     `json:"name"`
	AcademicYearID string     `json:"academic_year_id"`
	BodyTemplate   string     `json:"body_template"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// NewReportTemplate constructs a ReportTemplate, enforcing invariants.
func NewReportTemplate(tenantID, name, academicYearID, bodyTemplate string) (*ReportTemplate, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	bodyTemplate = strings.TrimSpace(bodyTemplate)
	if bodyTemplate == "" {
		return nil, fmt.Errorf("%w: body_template is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("report: generate template id: %w", err)
	}
	now := time.Now().UTC()
	return &ReportTemplate{
		ID:             id.String(),
		TenantID:       tenantID,
		Name:           strings.TrimSpace(name),
		AcademicYearID: strings.TrimSpace(academicYearID),
		BodyTemplate:   bodyTemplate,
		Status:         string(TemplateStatusDraft),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (t ReportTemplate) Validate() error {
	if t.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(t.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(t.BodyTemplate) == "" {
		return fmt.Errorf("%w: body_template is required", ErrValidation)
	}
	if !isValidTemplateStatus(TemplateStatus(t.Status)) {
		return fmt.Errorf("%w: status must be draft, active or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the template with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (t *ReportTemplate) ApplyUpdate(name, academicYearID, bodyTemplate, status *string) ([]string, error) {
	var changed []string

	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		t.Name = strings.TrimSpace(*name)
		changed = append(changed, "name")
	}
	if academicYearID != nil {
		if strings.TrimSpace(*academicYearID) == "" {
			return nil, fmt.Errorf("%w: academic_year_id cannot be empty", ErrValidation)
		}
		t.AcademicYearID = strings.TrimSpace(*academicYearID)
		changed = append(changed, "academic_year_id")
	}
	if bodyTemplate != nil {
		if strings.TrimSpace(*bodyTemplate) == "" {
			return nil, fmt.Errorf("%w: body_template cannot be empty", ErrValidation)
		}
		t.BodyTemplate = strings.TrimSpace(*bodyTemplate)
		changed = append(changed, "body_template")
	}
	if status != nil {
		s := TemplateStatus(strings.TrimSpace(*status))
		if !isValidTemplateStatus(s) {
			return nil, fmt.Errorf("%w: status must be draft, active or archived", ErrValidation)
		}
		if !isValidTemplateStatusTransition(TemplateStatus(t.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, t.Status, *status)
		}
		t.Status = string(s)
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		t.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidTemplateStatus(s TemplateStatus) bool {
	switch s {
	case TemplateStatusDraft, TemplateStatusActive, TemplateStatusArchived:
		return true
	}
	return false
}

// isValidTemplateStatusTransition defines the allowed template lifecycle transitions.
func isValidTemplateStatusTransition(from, to TemplateStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case TemplateStatusDraft:
		return to == TemplateStatusActive || to == TemplateStatusArchived
	case TemplateStatusActive:
		return to == TemplateStatusArchived || to == TemplateStatusDraft
	case TemplateStatusArchived:
		return to == TemplateStatusDraft
	}
	return false
}
