package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AnnouncementAudience enumerates the audiences an announcement can target.
type AnnouncementAudience string

const (
	AudienceAll       AnnouncementAudience = "all"
	AudienceStudents  AnnouncementAudience = "students"
	AudienceGuardians AnnouncementAudience = "guardians"
	AudienceStaff     AnnouncementAudience = "staff"
)

// Announcement is the aggregate root for a tenant-wide announcement.
// Creating an announcement also publishes an in-app message to the tenant inbox.
type Announcement struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Audience  string    `json:"audience"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewAnnouncement constructs an Announcement, enforcing invariants.
func NewAnnouncement(tenantID, title, body, audience string) (*Announcement, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%w: body is required", ErrValidation)
	}
	audience = strings.TrimSpace(strings.ToLower(audience))
	if audience == "" {
		audience = string(AudienceAll)
	}
	if !isValidAudience(AnnouncementAudience(audience)) {
		return nil, fmt.Errorf("%w: audience must be all, students, guardians or staff", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("notifications: generate announcement id: %w", err)
	}
	now := time.Now().UTC()
	return &Announcement{
		ID:        id.String(),
		TenantID:  tenantID,
		Title:     strings.TrimSpace(title),
		Body:      body,
		Audience:  audience,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (a Announcement) Validate() error {
	if a.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(a.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(a.Body) == "" {
		return fmt.Errorf("%w: body is required", ErrValidation)
	}
	if !isValidAudience(AnnouncementAudience(a.Audience)) {
		return fmt.Errorf("%w: audience must be all, students, guardians or staff", ErrValidation)
	}
	return nil
}

func isValidAudience(a AnnouncementAudience) bool {
	switch a {
	case AudienceAll, AudienceStudents, AudienceGuardians, AudienceStaff:
		return true
	}
	return false
}
