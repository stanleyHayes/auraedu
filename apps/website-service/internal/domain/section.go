package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SectionType enumerates the supported section types.
type SectionType string

const (
	SectionTypeHero     SectionType = "hero"
	SectionTypeText     SectionType = "text"
	SectionTypeFeatures SectionType = "features"
	SectionTypeGallery  SectionType = "gallery"
	SectionTypeCTA      SectionType = "cta"
	SectionTypeContact  SectionType = "contact"
)

// SectionStatus enumerates the lifecycle states of a section.
type SectionStatus string

const (
	SectionStatusDraft     SectionStatus = "draft"
	SectionStatusPublished SectionStatus = "published"
)

// Content is a JSONB-backed key/value map.
type Content map[string]any

// Section is the aggregate for page sections. Every record is tenant-scoped.
type Section struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	PageID    string    `json:"page_id"`
	Type      string    `json:"type"`
	Content   Content   `json:"content,omitempty"`
	SortOrder int       `json:"sort_order"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSection constructs a Section, enforcing invariants.
func NewSection(tenantID, pageID string, sectionType SectionType, content Content, sortOrder int) (*Section, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if pageID == "" {
		return nil, ErrValidation
	}
	if !isValidSectionType(string(sectionType)) {
		return nil, ErrValidation
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("section: generate id: %w", err)
	}
	now := time.Now().UTC()
	if content == nil {
		content = Content{}
	}
	return &Section{
		ID:        id.String(),
		TenantID:  tenantID,
		PageID:    pageID,
		Type:      string(sectionType),
		Content:   content,
		SortOrder: sortOrder,
		Status:    string(SectionStatusDraft),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the section aggregate is well-formed.
func (s Section) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if s.PageID == "" {
		return ErrValidation
	}
	if !isValidSectionType(s.Type) {
		return ErrValidation
	}
	if !isValidSectionStatus(s.Status) {
		return ErrValidation
	}
	if s.SortOrder < 0 {
		return ErrValidation
	}
	return nil
}

// ApplyUpdate mutates the section with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (s *Section) ApplyUpdate(sectionType *SectionType, content *Content, sortOrder *int, status *string) ([]string, error) {
	var changed []string
	if sectionType != nil {
		if !isValidSectionType(string(*sectionType)) {
			return nil, ErrValidation
		}
		if s.Type != string(*sectionType) {
			s.Type = string(*sectionType)
			changed = append(changed, "type")
		}
	}
	if content != nil {
		s.Content = *content
		if s.Content == nil {
			s.Content = Content{}
		}
		changed = append(changed, "content")
	}
	if sortOrder != nil {
		if *sortOrder < 0 {
			return nil, ErrValidation
		}
		if s.SortOrder != *sortOrder {
			s.SortOrder = *sortOrder
			changed = append(changed, "sort_order")
		}
	}
	if status != nil {
		if !isValidSectionStatus(*status) {
			return nil, ErrValidation
		}
		if s.Status != *status {
			s.Status = *status
			changed = append(changed, "status")
		}
	}
	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidSectionType(v string) bool {
	switch SectionType(v) {
	case SectionTypeHero, SectionTypeText, SectionTypeFeatures, SectionTypeGallery, SectionTypeCTA, SectionTypeContact:
		return true
	}
	return false
}

func isValidSectionStatus(v string) bool {
	switch SectionStatus(v) {
	case SectionStatusDraft, SectionStatusPublished:
		return true
	}
	return false
}
