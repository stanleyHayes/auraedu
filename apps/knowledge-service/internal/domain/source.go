// Package domain defines governed knowledge entities and invariants.
package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusApproved Status = "approved"
	StatusRetired  Status = "retired"
)

type Source struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	SourceType      string     `json:"source_type"`
	Title           string     `json:"title"`
	Owner           string     `json:"owner"`
	Content         string     `json:"content"`
	Status          Status     `json:"status"`
	Confidentiality string     `json:"confidentiality"`
	Locale          string     `json:"locale"`
	Version         int        `json:"version"`
	EffectiveAt     time.Time  `json:"effective_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Programme       *string    `json:"programme"`
	Campus          *string    `json:"campus"`
	Intake          *string    `json:"intake"`
	ApprovedBy      *string    `json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
	ReviewNote      *string    `json:"review_note"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CreateInput struct {
	TenantID, SourceType, Title, Owner, Content, Confidentiality, Locale string
	EffectiveAt                                                          time.Time
	ExpiresAt                                                            *time.Time
	Programme, Campus, Intake                                            *string
}

var localePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

func NormalizeLocale(locale string) (string, bool) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "en", true
	}
	language := strings.SplitN(locale, "-", 2)[0]
	if !localePattern.MatchString(locale) || (language != "en" && language != "fr") {
		return "", false
	}
	return locale, true
}

func SameLanguage(left, right string) bool {
	left, leftOK := NormalizeLocale(left)
	right, rightOK := NormalizeLocale(right)
	if !leftOK || !rightOK {
		return false
	}
	return strings.SplitN(left, "-", 2)[0] == strings.SplitN(right, "-", 2)[0]
}

func NewSource(input CreateInput, now time.Time) (Source, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	input.Title = strings.TrimSpace(input.Title)
	input.Owner = strings.TrimSpace(input.Owner)
	input.Content = strings.TrimSpace(input.Content)
	input.Confidentiality = strings.ToLower(strings.TrimSpace(input.Confidentiality))
	locale, localeOK := NormalizeLocale(input.Locale)
	if input.TenantID == "" || !validSourceType(input.SourceType) || len(input.Title) < 3 || len(input.Title) > 200 ||
		len(input.Owner) < 2 || len(input.Owner) > 120 || len(input.Content) < 20 || len(input.Content) > 100000 {
		return Source{}, ErrValidation
	}
	if (input.Confidentiality != "public" && input.Confidentiality != "internal") || !localeOK {
		return Source{}, ErrValidation
	}
	if input.EffectiveAt.IsZero() || (input.ExpiresAt != nil && !input.ExpiresAt.After(input.EffectiveAt)) {
		return Source{}, ErrValidation
	}
	now = now.UTC()
	return Source{ID: uuid.NewString(), TenantID: input.TenantID, SourceType: input.SourceType,
		Title: input.Title, Owner: input.Owner, Content: input.Content, Status: StatusDraft,
		Confidentiality: input.Confidentiality, Locale: locale, Version: 1, EffectiveAt: input.EffectiveAt.UTC(),
		ExpiresAt: input.ExpiresAt, Programme: cleanOptional(input.Programme), Campus: cleanOptional(input.Campus),
		Intake: cleanOptional(input.Intake), CreatedAt: now, UpdatedAt: now}, nil
}

func validSourceType(value string) bool {
	switch value {
	case "programme", "admissions", "fees", "scholarship", "calendar", "policy", "campus",
		"accommodation", "faq", "announcement", "marketing", "support":
		return true
	default:
		return false
	}
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	if clean == "" {
		return nil
	}
	return &clean
}

func (s Source) IsRetrievable(at time.Time) bool {
	return s.Status == StatusApproved && s.Confidentiality == "public" && !s.EffectiveAt.After(at) &&
		(s.ExpiresAt == nil || s.ExpiresAt.After(at))
}

type SearchResult struct {
	SourceID    string     `json:"source_id"`
	Title       string     `json:"title"`
	Passage     string     `json:"passage"`
	SourceType  string     `json:"source_type"`
	Locale      string     `json:"locale"`
	Version     int        `json:"version"`
	Score       float64    `json:"score"`
	EffectiveAt time.Time  `json:"effective_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}
