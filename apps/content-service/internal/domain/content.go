// Package domain defines governed content entities and lifecycle rules.
package domain

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrValidation  = errors.New("content validation failed")
	ErrForbidden   = errors.New("content access forbidden")
	ErrNotFound    = errors.New("content not found")
	ErrConflict    = errors.New("content lifecycle conflict")
	ErrUnavailable = errors.New("content generator unavailable")
)

type Status string

const (
	StatusDraft         Status = "draft"
	StatusPendingReview Status = "pending_review"
	StatusApproved      Status = "approved"
	StatusRejected      Status = "rejected"
	StatusExpired       Status = "expired"
)

type ComplianceStatus string

const (
	CompliancePass        ComplianceStatus = "pass"
	ComplianceNeedsReview ComplianceStatus = "needs_review"
	ComplianceFail        ComplianceStatus = "fail"
)

var localePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

func IsValidContentType(value string) bool {
	switch strings.TrimSpace(value) {
	case "social_post", "caption", "ad_copy", "video_script", "landing_page", "email", "sms",
		"whatsapp_sequence", "programme_description", "brochure", "prospectus", "event_invitation",
		"scholarship_announcement", "radio_script", "faq", "applicant_guide":
		return true
	default:
		return false
	}
}

func IsValidStatus(value string) bool {
	switch Status(strings.TrimSpace(value)) {
	case StatusDraft, StatusPendingReview, StatusApproved, StatusRejected, StatusExpired:
		return true
	default:
		return false
	}
}

type Fact struct {
	Label    string  `json:"label"`
	Value    string  `json:"value"`
	SourceID *string `json:"source_id"`
}

type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type BrandProfile struct {
	TenantID            string    `json:"tenant_id"`
	ToneOfVoice         string    `json:"tone_of_voice"`
	ApprovedTerms       []string  `json:"approved_terms"`
	ProhibitedClaims    []string  `json:"prohibited_claims"`
	RequiredDisclaimers []string  `json:"required_disclaimers"`
	Locale              string    `json:"locale"`
	Version             int       `json:"version"`
	UpdatedBy           string    `json:"updated_by"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BrandProfileInput struct {
	TenantID, ToneOfVoice, Locale, UpdatedBy string
	ApprovedTerms, ProhibitedClaims          []string
	RequiredDisclaimers                      []string
	ExpectedVersion                          int
}

func NewBrandProfile(input BrandProfileInput, now time.Time) (BrandProfile, error) {
	profile := BrandProfile{
		TenantID: strings.TrimSpace(input.TenantID), ToneOfVoice: strings.TrimSpace(input.ToneOfVoice),
		ApprovedTerms: normalizeList(input.ApprovedTerms), ProhibitedClaims: normalizeList(input.ProhibitedClaims),
		RequiredDisclaimers: normalizeList(input.RequiredDisclaimers), Locale: strings.TrimSpace(input.Locale),
		Version: input.ExpectedVersion + 1, UpdatedBy: strings.TrimSpace(input.UpdatedBy), UpdatedAt: now.UTC(),
	}
	if !profileIsValid(profile, input.ExpectedVersion) {
		return BrandProfile{}, ErrValidation
	}
	return profile, nil
}

func profileIsValid(profile BrandProfile, expectedVersion int) bool {
	return expectedVersion >= 0 && profile.TenantID != "" && profile.UpdatedBy != "" &&
		len(profile.ToneOfVoice) >= 3 && len(profile.ToneOfVoice) <= 1000 &&
		localePattern.MatchString(profile.Locale) && len(profile.ApprovedTerms) <= 100 &&
		len(profile.ProhibitedClaims) <= 100 && len(profile.RequiredDisclaimers) <= 25 &&
		listLengths(profile.ApprovedTerms, 120) && listLengths(profile.ProhibitedClaims, 240) &&
		listLengths(profile.RequiredDisclaimers, 500)
}

type Draft struct {
	ID                  string           `json:"id"`
	TenantID            string           `json:"tenant_id"`
	CampaignID          *string          `json:"campaign_id"`
	ContentType         string           `json:"content_type"`
	Title               string           `json:"title"`
	Brief               string           `json:"brief"`
	Audience            string           `json:"audience"`
	Locale              string           `json:"locale"`
	KeyMessages         []string         `json:"key_messages"`
	Facts               []Fact           `json:"facts"`
	Content             string           `json:"content"`
	Status              Status           `json:"status"`
	Version             int              `json:"version"`
	ComplianceStatus    ComplianceStatus `json:"compliance_status"`
	ComplianceFindings  []Finding        `json:"compliance_findings"`
	Generator           string           `json:"generator"`
	BrandProfileVersion int              `json:"brand_profile_version"`
	CreatedBy           string           `json:"created_by"`
	SubmittedBy         *string          `json:"submitted_by"`
	SubmittedAt         *time.Time       `json:"submitted_at"`
	ReviewedBy          *string          `json:"reviewed_by"`
	ReviewedAt          *time.Time       `json:"reviewed_at"`
	ReviewNote          *string          `json:"review_note"`
	ExpiresAt           *time.Time       `json:"expires_at"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

type DraftInput struct {
	TenantID, ContentType, Title, Brief, Audience, Locale, Content, Generator, CreatedBy string
	CampaignID                                                                           *string
	KeyMessages                                                                          []string
	Facts                                                                                []Fact
	BrandProfileVersion                                                                  int
	ExpiresAt                                                                            *time.Time
}

func NewDraft(input DraftInput, profile BrandProfile, now time.Time) (Draft, error) {
	now = now.UTC()
	draft := Draft{
		ID: uuid.NewString(), TenantID: strings.TrimSpace(input.TenantID), CampaignID: input.CampaignID,
		ContentType: strings.TrimSpace(input.ContentType), Title: strings.TrimSpace(input.Title), Brief: strings.TrimSpace(input.Brief),
		Audience: strings.TrimSpace(input.Audience), Locale: strings.TrimSpace(input.Locale), KeyMessages: normalizeList(input.KeyMessages),
		Facts: normalizeFacts(input.Facts), Content: strings.TrimSpace(input.Content), Status: StatusDraft, Version: 1,
		Generator: strings.TrimSpace(input.Generator), BrandProfileVersion: input.BrandProfileVersion, CreatedBy: strings.TrimSpace(input.CreatedBy),
		ExpiresAt: normalizeTime(input.ExpiresAt), CreatedAt: now, UpdatedAt: now,
	}
	profileMatches := profile.TenantID == draft.TenantID && profile.Version == draft.BrandProfileVersion &&
		profile.Locale == draft.Locale
	if err := draft.validate(); err != nil || !profileMatches {
		return Draft{}, ErrValidation
	}
	draft.ComplianceStatus, draft.ComplianceFindings = EvaluateCompliance(draft.Content, profile)
	return draft, nil
}

func (d Draft) validate() error {
	if !d.hasValidRequiredFields() || !IsValidContentType(d.ContentType) {
		return ErrValidation
	}
	if d.CampaignID != nil {
		if _, err := uuid.Parse(*d.CampaignID); err != nil {
			return ErrValidation
		}
	}
	if d.ExpiresAt != nil && !d.ExpiresAt.After(d.CreatedAt) {
		return ErrValidation
	}
	if !factsAreValid(d.Facts) {
		return ErrValidation
	}
	return nil
}

func (d Draft) hasValidRequiredFields() bool {
	return d.TenantID != "" && d.CreatedBy != "" && d.Generator != "" && d.BrandProfileVersion >= 1 &&
		len(d.Title) >= 3 && len(d.Title) <= 160 && len(d.Brief) >= 20 && len(d.Brief) <= 5000 &&
		len(d.Audience) >= 3 && len(d.Audience) <= 1000 && len(d.Content) >= 1 && len(d.Content) <= 50000 &&
		localePattern.MatchString(d.Locale) && len(d.KeyMessages) >= 1 && len(d.KeyMessages) <= 20 &&
		len(d.Facts) >= 1 && len(d.Facts) <= 30
}

func factsAreValid(facts []Fact) bool {
	for _, fact := range facts {
		if len(fact.Label) < 1 || len(fact.Label) > 120 || len(fact.Value) < 1 || len(fact.Value) > 1000 {
			return false
		}
		if fact.SourceID != nil {
			if _, err := uuid.Parse(*fact.SourceID); err != nil {
				return false
			}
		}
	}
	return true
}

type Version struct {
	ContentID           string           `json:"content_id"`
	Version             int              `json:"version"`
	Content             string           `json:"content"`
	Status              Status           `json:"status"`
	ComplianceStatus    ComplianceStatus `json:"compliance_status"`
	ComplianceFindings  []Finding        `json:"compliance_findings"`
	Generator           string           `json:"generator"`
	BrandProfileVersion int              `json:"brand_profile_version"`
	CreatedBy           string           `json:"created_by"`
	ChangeNote          string           `json:"change_note"`
	CreatedAt           time.Time        `json:"created_at"`
}

func (d Draft) Snapshot(changeNote string) Version {
	return Version{ContentID: d.ID, Version: d.Version, Content: d.Content, Status: d.Status,
		ComplianceStatus: d.ComplianceStatus, ComplianceFindings: slices.Clone(d.ComplianceFindings), Generator: d.Generator,
		BrandProfileVersion: d.BrandProfileVersion, CreatedBy: d.CreatedBy, ChangeNote: strings.TrimSpace(changeNote), CreatedAt: d.UpdatedAt}
}

func (d *Draft) Revise(actor, content, note string, expectedVersion int, expiresAt *time.Time, profile BrandProfile, now time.Time) error {
	content, note, actor = strings.TrimSpace(content), strings.TrimSpace(note), strings.TrimSpace(actor)
	if !d.canRevise(actor, content, note, expectedVersion, profile) {
		return ErrConflict
	}
	d.Content, d.Generator, d.BrandProfileVersion = content, "human-revision", profile.Version
	d.Version++
	d.Status, d.CreatedBy = StatusDraft, actor
	d.SubmittedBy, d.SubmittedAt, d.ReviewedBy, d.ReviewedAt, d.ReviewNote = nil, nil, nil, nil, nil
	d.ExpiresAt, d.UpdatedAt = normalizeTime(expiresAt), now.UTC()
	if d.ExpiresAt != nil && !d.ExpiresAt.After(d.UpdatedAt) {
		return ErrValidation
	}
	d.ComplianceStatus, d.ComplianceFindings = EvaluateCompliance(d.Content, profile)
	return nil
}

func (d Draft) canRevise(actor, content, note string, expectedVersion int, profile BrandProfile) bool {
	return actor != "" && expectedVersion == d.Version &&
		(d.Status == StatusDraft || d.Status == StatusRejected) && len(content) >= 1 && len(content) <= 50000 &&
		len(note) >= 3 && len(note) <= 500 && profile.TenantID == d.TenantID && profile.Version >= 1 &&
		profile.Locale == d.Locale
}

func (d *Draft) Submit(actor string, expectedVersion int, now time.Time) error {
	actor = strings.TrimSpace(actor)
	if actor == "" || expectedVersion != d.Version || d.Status != StatusDraft || d.ComplianceStatus != CompliancePass || d.IsExpired(now) {
		return ErrConflict
	}
	now = now.UTC()
	d.Status, d.SubmittedBy, d.SubmittedAt, d.UpdatedAt = StatusPendingReview, &actor, &now, now
	return nil
}

func (d *Draft) Review(actor, note string, expectedVersion int, approve bool, now time.Time) error {
	actor, note = strings.TrimSpace(actor), strings.TrimSpace(note)
	if !d.canReview(actor, note, expectedVersion, approve, now) {
		return ErrConflict
	}
	now = now.UTC()
	if approve {
		d.Status = StatusApproved
	} else {
		d.Status = StatusRejected
	}
	d.ReviewedBy, d.ReviewedAt, d.ReviewNote, d.UpdatedAt = &actor, &now, &note, now
	return nil
}

func (d Draft) canReview(actor, note string, expectedVersion int, approve bool, now time.Time) bool {
	return actor != "" && len(note) >= 3 && len(note) <= 1000 && expectedVersion == d.Version &&
		d.Status == StatusPendingReview && d.SubmittedBy != nil && actor != *d.SubmittedBy && !d.IsExpired(now) &&
		(!approve || d.ComplianceStatus == CompliancePass)
}

func (d Draft) IsExpired(now time.Time) bool { return d.ExpiresAt != nil && !d.ExpiresAt.After(now) }

func EvaluateCompliance(content string, profile BrandProfile) (ComplianceStatus, []Finding) {
	lower := strings.ToLower(content)
	findings := []Finding{}
	status := CompliancePass
	for _, claim := range profile.ProhibitedClaims {
		if strings.Contains(lower, strings.ToLower(claim)) {
			status = ComplianceFail
			findings = append(findings, Finding{Rule: "prohibited_claim", Severity: "error", Message: "Remove a prohibited institutional claim before review."})
		}
	}
	for _, disclaimer := range profile.RequiredDisclaimers {
		if !strings.Contains(lower, strings.ToLower(disclaimer)) {
			if status != ComplianceFail {
				status = ComplianceNeedsReview
			}
			findings = append(findings, Finding{Rule: "required_disclaimer", Severity: "warning", Message: "Add the required institution disclaimer before review."})
		}
	}
	return status, findings
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeFacts(values []Fact) []Fact {
	out := make([]Fact, 0, len(values))
	for _, fact := range values {
		fact.Label, fact.Value = strings.TrimSpace(fact.Label), strings.TrimSpace(fact.Value)
		out = append(out, fact)
	}
	return out
}

func listLengths(values []string, maximum int) bool {
	for _, value := range values {
		if len(value) < 1 || len(value) > maximum {
			return false
		}
	}
	return true
}

func normalizeTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}
