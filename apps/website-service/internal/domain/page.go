// Package domain holds the website-service business rules and aggregate models.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PageStatus enumerates the lifecycle states of a page.
type PageStatus string

const (
	PageStatusDraft     PageStatus = "draft"
	PageStatusPublished PageStatus = "published"
	PageStatusArchived  PageStatus = "archived"
)

// PageLayout enumerates the supported page layouts.
type PageLayout string

const (
	PageLayoutDefault PageLayout = "default"
	PageLayoutLanding PageLayout = "landing"
	PageLayoutContact PageLayout = "contact"
)

// Page is the aggregate root for website pages. Every record is tenant-scoped.
type Page struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	MetaDescription *string    `json:"meta_description,omitempty"`
	Layout          string     `json:"layout"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

// NewPage constructs a Page, enforcing invariants.
func NewPage(tenantID, slug, title string) (*Page, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	slug = NormalizeSlug(slug)
	if slug == "" {
		return nil, ErrValidation
	}
	if strings.TrimSpace(title) == "" {
		return nil, ErrValidation
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("page: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Page{
		ID:        id.String(),
		TenantID:  tenantID,
		Slug:      slug,
		Title:     strings.TrimSpace(title),
		Status:    string(PageStatusDraft),
		Layout:    string(PageLayoutDefault),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the page aggregate is well-formed.
func (p Page) Validate() error {
	if p.TenantID == "" {
		return ErrMissingTenant
	}
	if NormalizeSlug(p.Slug) == "" {
		return ErrValidation
	}
	if strings.TrimSpace(p.Title) == "" {
		return ErrValidation
	}
	if !isValidPageStatus(p.Status) {
		return ErrValidation
	}
	if !isValidPageLayout(p.Layout) {
		return ErrValidation
	}
	return nil
}

// ApplyUpdate mutates the page with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (p *Page) ApplyUpdate(slug, title, status, metaDescription, layout *string) ([]string, error) {
	var changed []string
	if slug != nil {
		normalized := NormalizeSlug(*slug)
		if normalized == "" {
			return nil, ErrValidation
		}
		if p.Slug != normalized {
			p.Slug = normalized
			changed = append(changed, "slug")
		}
	}
	if title != nil {
		if strings.TrimSpace(*title) == "" {
			return nil, ErrValidation
		}
		if p.Title != strings.TrimSpace(*title) {
			p.Title = strings.TrimSpace(*title)
			changed = append(changed, "title")
		}
	}
	if status != nil {
		if !isValidPageStatus(*status) {
			return nil, ErrValidation
		}
		if p.Status != *status {
			p.Status = *status
			changed = append(changed, "status")
			if p.Status == string(PageStatusPublished) && p.PublishedAt == nil {
				now := time.Now().UTC()
				p.PublishedAt = &now
			}
		}
	}
	if metaDescription != nil {
		if p.MetaDescription == nil || *p.MetaDescription != *metaDescription {
			p.MetaDescription = metaDescription
			changed = append(changed, "meta_description")
		}
	}
	if layout != nil {
		if !isValidPageLayout(*layout) {
			return nil, ErrValidation
		}
		if p.Layout != *layout {
			p.Layout = *layout
			changed = append(changed, "layout")
		}
	}
	if len(changed) > 0 {
		p.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// IsPublished reports whether the page is currently published.
func (p Page) IsPublished() bool { return p.Status == string(PageStatusPublished) }

func NormalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), "-")
}

func isValidPageStatus(v string) bool {
	switch PageStatus(v) {
	case PageStatusDraft, PageStatusPublished, PageStatusArchived:
		return true
	}
	return false
}

func isValidPageLayout(v string) bool {
	switch PageLayout(v) {
	case PageLayoutDefault, PageLayoutLanding, PageLayoutContact:
		return true
	}
	return false
}
