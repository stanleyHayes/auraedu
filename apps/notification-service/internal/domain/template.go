package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TemplateStatus enumerates the lifecycle states of a notification template.
type TemplateStatus string

const (
	TemplateStatusActive   TemplateStatus = "active"
	TemplateStatusArchived TemplateStatus = "archived"
)

// Template is the aggregate root for a reusable notification template.
type Template struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	Name            string    `json:"name"`
	Channel         string    `json:"channel"`
	SubjectTemplate string    `json:"subject_template"`
	BodyTemplate    string    `json:"body_template"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NewTemplate constructs a Template, enforcing invariants.
func NewTemplate(tenantID, name, channel, subjectTemplate, bodyTemplate string) (*Template, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("%w: channel is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(channel)) {
		return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp or in_app", ErrValidation)
	}
	if strings.TrimSpace(subjectTemplate) == "" {
		return nil, fmt.Errorf("%w: subject_template is required", ErrValidation)
	}
	if strings.TrimSpace(bodyTemplate) == "" {
		return nil, fmt.Errorf("%w: body_template is required", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("notifications: generate template id: %w", err)
	}
	now := time.Now().UTC()
	return &Template{
		ID:              id.String(),
		TenantID:        tenantID,
		Name:            strings.TrimSpace(name),
		Channel:         strings.TrimSpace(strings.ToLower(channel)),
		SubjectTemplate: strings.TrimSpace(subjectTemplate),
		BodyTemplate:    bodyTemplate,
		Status:          string(TemplateStatusActive),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (t Template) Validate() error {
	if t.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(t.Channel)) {
		return fmt.Errorf("%w: channel must be email, sms, whatsapp or in_app", ErrValidation)
	}
	if strings.TrimSpace(t.SubjectTemplate) == "" {
		return fmt.Errorf("%w: subject_template is required", ErrValidation)
	}
	if strings.TrimSpace(t.BodyTemplate) == "" {
		return fmt.Errorf("%w: body_template is required", ErrValidation)
	}
	if !isValidTemplateStatus(TemplateStatus(t.Status)) {
		return fmt.Errorf("%w: status must be active or archived", ErrValidation)
	}
	return nil
}

// TemplatePatch carries optional update fields.
type TemplatePatch struct {
	Name            *string
	Channel         *string
	SubjectTemplate *string
	BodyTemplate    *string
	Status          *string
}

// ApplyUpdate mutates the template with non-nil patch fields.
func (t *Template) ApplyUpdate(patch TemplatePatch) ([]string, error) {
	var changed []string

	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		t.Name = strings.TrimSpace(*patch.Name)
		changed = append(changed, "name")
	}
	if patch.Channel != nil {
		if !isValidChannel(NotificationChannel(*patch.Channel)) {
			return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp or in_app", ErrValidation)
		}
		t.Channel = strings.TrimSpace(strings.ToLower(*patch.Channel))
		changed = append(changed, "channel")
	}
	if patch.SubjectTemplate != nil {
		if strings.TrimSpace(*patch.SubjectTemplate) == "" {
			return nil, fmt.Errorf("%w: subject_template cannot be empty", ErrValidation)
		}
		t.SubjectTemplate = strings.TrimSpace(*patch.SubjectTemplate)
		changed = append(changed, "subject_template")
	}
	if patch.BodyTemplate != nil {
		if strings.TrimSpace(*patch.BodyTemplate) == "" {
			return nil, fmt.Errorf("%w: body_template cannot be empty", ErrValidation)
		}
		t.BodyTemplate = *patch.BodyTemplate
		changed = append(changed, "body_template")
	}
	if patch.Status != nil {
		if !isValidTemplateStatus(TemplateStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be active or archived", ErrValidation)
		}
		t.Status = *patch.Status
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		t.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidTemplateStatus(s TemplateStatus) bool {
	switch s {
	case TemplateStatusActive, TemplateStatusArchived:
		return true
	}
	return false
}
