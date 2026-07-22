// Package domain defines Campaign aggregates and lifecycle rules.
package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("campaign validation failed")
	ErrForbidden  = errors.New("campaign access forbidden")
	ErrNotFound   = errors.New("campaign not found")
	ErrConflict   = errors.New("campaign lifecycle conflict")
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPending   Status = "pending_approval"
	StatusApproved  Status = "approved"
	StatusScheduled Status = "scheduled"
	StatusActive    Status = "active"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type Campaign struct {
	ID                    string     `json:"id"`
	TenantID              string     `json:"tenant_id"`
	Name                  string     `json:"name"`
	Objective             string     `json:"objective"`
	Status                Status     `json:"status"`
	Channel               string     `json:"channel"`
	AudienceDefinition    string     `json:"audience_definition"`
	ProgrammeIDs          []string   `json:"programme_ids"`
	Budget                float64    `json:"budget"`
	Currency              string     `json:"currency"`
	StartAt               time.Time  `json:"start_at"`
	EndAt                 time.Time  `json:"end_at"`
	ApprovalStatus        string     `json:"approval_status"`
	OwnerUserID           string     `json:"owner_user_id"`
	SubmittedBy           *string    `json:"submitted_by"`
	SubmittedAt           *time.Time `json:"submitted_at"`
	ApprovedBy            *string    `json:"approved_by"`
	ApprovedAt            *time.Time `json:"approved_at"`
	ReviewNote            *string    `json:"review_note"`
	TrackingURLParameters string     `json:"tracking_url_parameters"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type CreateInput struct {
	TenantID, Name, Objective, Channel, AudienceDefinition, Currency, OwnerUserID string
	ProgrammeIDs                                                                  []string
	Budget                                                                        float64
	StartAt, EndAt                                                                time.Time
}

func NewCampaign(input CreateInput, now time.Time) (Campaign, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Objective = strings.TrimSpace(input.Objective)
	input.Channel = strings.ToLower(strings.TrimSpace(input.Channel))
	input.AudienceDefinition = strings.TrimSpace(input.AudienceDefinition)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	id := uuid.NewString()
	now = now.UTC()
	if input.ProgrammeIDs == nil {
		input.ProgrammeIDs = []string{}
	}
	tracking := url.Values{
		"utm_source":   {"auraedu"},
		"utm_medium":   {input.Channel},
		"utm_campaign": {slug(input.Name) + "-" + id[:8]},
	}.Encode()
	campaign := Campaign{
		ID: id, TenantID: input.TenantID, Name: input.Name, Objective: input.Objective,
		Channel: input.Channel, AudienceDefinition: input.AudienceDefinition,
		ProgrammeIDs: input.ProgrammeIDs, Budget: input.Budget, Currency: input.Currency,
		StartAt: input.StartAt.UTC(), EndAt: input.EndAt.UTC(), Status: StatusDraft,
		ApprovalStatus: "not_submitted", OwnerUserID: input.OwnerUserID,
		TrackingURLParameters: tracking, CreatedAt: now, UpdatedAt: now,
	}
	if err := campaign.ValidateDraft(); err != nil {
		return Campaign{}, err
	}
	return campaign, nil
}

func (c Campaign) ValidateDraft() error {
	valid := c.TenantID != "" && c.OwnerUserID != "" &&
		len(c.Name) >= 3 && len(c.Name) <= 160 &&
		len(c.Objective) >= 3 && len(c.Objective) <= 500 &&
		validChannel(c.Channel) && len(c.AudienceDefinition) >= 3 && len(c.AudienceDefinition) <= 2000 &&
		c.Budget >= 0 && c.Budget <= 100000000 && currencyPattern.MatchString(c.Currency) &&
		!c.StartAt.IsZero() && c.EndAt.After(c.StartAt) && len(c.ProgrammeIDs) <= 30
	if !valid {
		return ErrValidation
	}
	for _, id := range c.ProgrammeIDs {
		if _, err := uuid.Parse(id); err != nil {
			return ErrValidation
		}
	}
	return nil
}

func validChannel(channel string) bool {
	switch channel {
	case "website", "email", "sms", "whatsapp", "facebook", "instagram",
		"tiktok", "youtube", "linkedin", "radio", "event", "referral",
		"school_visit", "agent", "affiliate":
		return true
	default:
		return false
	}
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func (c *Campaign) Submit(actor string, now time.Time) error {
	if c.Status != StatusDraft || actor == "" {
		return ErrConflict
	}
	c.Status, c.ApprovalStatus, c.SubmittedBy = StatusPending, "pending", &actor
	now = now.UTC()
	c.SubmittedAt, c.UpdatedAt = &now, now
	return nil
}

func (c *Campaign) Approve(actor, note string, now time.Time) error {
	note = strings.TrimSpace(note)
	if c.Status != StatusPending || actor == "" || c.SubmittedBy == nil || actor == *c.SubmittedBy || len(note) < 3 || len(note) > 500 {
		return ErrConflict
	}
	c.Status, c.ApprovalStatus, c.ApprovedBy, c.ReviewNote = StatusApproved, "approved", &actor, &note
	now = now.UTC()
	c.ApprovedAt, c.UpdatedAt = &now, now
	return nil
}

func (c *Campaign) Publish(now time.Time) error {
	if c.Status != StatusApproved && c.Status != StatusPaused {
		return ErrConflict
	}
	if !c.EndAt.After(now) {
		return ErrConflict
	}
	if c.StartAt.After(now) {
		c.Status = StatusScheduled
	} else {
		c.Status = StatusActive
	}
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *Campaign) Pause(now time.Time) error {
	if c.Status != StatusActive && c.Status != StatusScheduled {
		return ErrConflict
	}
	c.Status, c.UpdatedAt = StatusPaused, now.UTC()
	return nil
}
