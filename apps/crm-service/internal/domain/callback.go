package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type CallbackStatus string

const (
	CallbackRequested CallbackStatus = "requested"
	CallbackConfirmed CallbackStatus = "confirmed"
	CallbackCompleted CallbackStatus = "completed"
	CallbackCancelled CallbackStatus = "cancelled"
)

type CallbackRequest struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	LeadID      string         `json:"lead_id"`
	PreferredAt time.Time      `json:"preferred_at"`
	Timezone    string         `json:"timezone"`
	Locale      string         `json:"locale"`
	Status      CallbackStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func NewCallbackRequest(tenantID, leadID string, preferredAt time.Time, timezone, locale string, now time.Time) (*CallbackRequest, error) {
	tenantID, leadID = strings.TrimSpace(tenantID), strings.TrimSpace(leadID)
	timezone, locale = strings.TrimSpace(timezone), strings.TrimSpace(locale)
	if tenantID == "" || uuid.Validate(leadID) != nil || preferredAt.IsZero() || timezone == "" {
		return nil, ErrValidation
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrValidation
	}
	if locale == "" {
		locale = "en"
	}
	parts := strings.SplitN(locale, "-", 2)
	if (parts[0] != "en" && parts[0] != "fr") || (len(parts) == 2 && parts[1] != "GH") {
		return nil, ErrValidation
	}
	now, preferredAt = now.UTC(), preferredAt.UTC()
	if preferredAt.Before(now.Add(15*time.Minute)) || preferredAt.After(now.Add(90*24*time.Hour)) {
		return nil, ErrValidation
	}
	return &CallbackRequest{
		ID: uuid.NewString(), TenantID: tenantID, LeadID: leadID, PreferredAt: preferredAt,
		Timezone: timezone, Locale: locale, Status: CallbackRequested, CreatedAt: now, UpdatedAt: now,
	}, nil
}
