package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	DefaultAlertThreshold  = 3
	DefaultAlertWindowDays = 30
)

type AlertRule struct {
	TenantID   string    `json:"tenant_id"`
	Threshold  int       `json:"threshold"`
	WindowDays int       `json:"window_days"`
	UpdatedBy  string    `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NewAlertRule(tenant string, threshold, windowDays int, actor string, now time.Time) (AlertRule, error) {
	tenant, actor = strings.TrimSpace(tenant), strings.TrimSpace(actor)
	if tenant == "" || actor == "" || threshold < 2 || threshold > 20 || windowDays < 1 || windowDays > 90 {
		return AlertRule{}, ErrValidation
	}
	return AlertRule{TenantID: tenant, Threshold: threshold, WindowDays: windowDays, UpdatedBy: actor, UpdatedAt: now.UTC()}, nil
}

type Alert struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	Fingerprint         string     `json:"fingerprint"`
	Category            string     `json:"category"`
	ProgrammeID         *string    `json:"programme_id"`
	CampusID            *string    `json:"campus_id"`
	ObservationCount    int        `json:"observation_count"`
	Threshold           int        `json:"threshold"`
	WindowDays          int        `json:"window_days"`
	FirstObservedAt     time.Time  `json:"first_observed_at"`
	LastObservedAt      time.Time  `json:"last_observed_at"`
	Reason              string     `json:"reason"`
	Status              string     `json:"status"`
	AcknowledgedBy      *string    `json:"acknowledged_by"`
	AcknowledgedAt      *time.Time `json:"acknowledged_at"`
	AcknowledgementNote *string    `json:"acknowledgement_note"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func AlertFingerprint(category string, programmeID, campusID *string) string {
	programme, campus := "all", "all"
	if programmeID != nil {
		programme = *programmeID
	}
	if campusID != nil {
		campus = *campusID
	}
	return category + "|" + programme + "|" + campus
}
func AlertReason(category string, count, threshold, window int) string {
	return fmt.Sprintf("%d approved %s observations reached the threshold of %d within %d days", count, strings.ReplaceAll(category, "_", " "), threshold, window)
}
func (a *Alert) Acknowledge(actor, note string, now time.Time) error {
	actor, note = strings.TrimSpace(actor), strings.TrimSpace(note)
	if a.Status != "open" || actor == "" || len(note) < 3 || len(note) > 1000 {
		return ErrConflict
	}
	now = now.UTC()
	a.Status = "acknowledged"
	a.AcknowledgedBy = &actor
	a.AcknowledgedAt = &now
	a.AcknowledgementNote = &note
	a.UpdatedAt = now
	return nil
}
