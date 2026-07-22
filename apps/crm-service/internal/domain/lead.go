// Package domain contains the Growth CRM aggregates and invariants.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// LeadStage is the controlled recruitment lifecycle from the CRM contract.
type LeadStage string

const (
	StageNew                  LeadStage = "new"
	StageContacted            LeadStage = "contacted"
	StageEngaged              LeadStage = "engaged"
	StageQualified            LeadStage = "qualified"
	StageApplicationStarted   LeadStage = "application_started"
	StageApplicationCompleted LeadStage = "application_completed"
	StageUnderReview          LeadStage = "under_review"
	StageAdmitted             LeadStage = "admitted"
	StageOfferAccepted        LeadStage = "offer_accepted"
	StageDepositPaid          LeadStage = "deposit_paid"
	StageEnrolled             LeadStage = "enrolled"
	StageLost                 LeadStage = "lost"
	StageDeferred             LeadStage = "deferred"
	StageWithdrawn            LeadStage = "withdrawn"
)

func ParseLeadStage(value string) (LeadStage, error) {
	stage := LeadStage(strings.TrimSpace(value))
	if !validLeadStage(stage) {
		return "", ErrValidation
	}
	return stage, nil
}

func validLeadStage(stage LeadStage) bool {
	switch stage {
	case StageNew, StageContacted, StageEngaged, StageQualified, StageApplicationStarted,
		StageApplicationCompleted, StageUnderReview, StageAdmitted, StageOfferAccepted,
		StageDepositPaid, StageEnrolled, StageLost, StageDeferred, StageWithdrawn:
		return true
	default:
		return false
	}
}

// Consent is the prospect's channel-specific permission snapshot.
type Consent struct {
	PrivacyNoticeVersion string `json:"privacy_notice_version"`
	Email                bool   `json:"email"`
	SMS                  bool   `json:"sms"`
	WhatsApp             bool   `json:"whatsapp"`
	Voice                bool   `json:"voice"`
}

// Lead is the tenant-owned recruitment aggregate. Email and phone are PII and
// must never be written to logs or events.
type Lead struct {
	ID                    string        `json:"id"`
	TenantID              string        `json:"tenant_id"`
	InstitutionID         *string       `json:"institution_id,omitempty"`
	FirstName             string        `json:"first_name"`
	LastName              string        `json:"last_name"`
	Email                 *string       `json:"email,omitempty"`
	Phone                 *string       `json:"phone,omitempty"`
	PreferredProgrammeIDs []string      `json:"preferred_programme_ids"`
	PreferredIntakeID     *string       `json:"preferred_intake_id,omitempty"`
	Source                string        `json:"source"`
	CampaignID            *string       `json:"campaign_id,omitempty"`
	Stage                 LeadStage     `json:"stage"`
	OwnerUserID           *string       `json:"owner_user_id,omitempty"`
	Score                 *int          `json:"score"`
	ScoreVersion          *string       `json:"score_version"`
	ScoreConfidence       *string       `json:"score_confidence"`
	ScorePositiveFactors  []ScoreFactor `json:"score_positive_factors"`
	ScoreNegativeFactors  []ScoreFactor `json:"score_negative_factors"`
	ScoredAt              *time.Time    `json:"scored_at"`
	Consent               Consent       `json:"consent"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

// NewLead normalizes contact data and enforces capture-time invariants.
func NewLead(tenantID, firstName, lastName string, email, phone *string, source string, consent Consent) (*Lead, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	firstName, lastName, source = strings.TrimSpace(firstName), strings.TrimSpace(lastName), strings.TrimSpace(source)
	if firstName == "" || lastName == "" || source == "" || strings.TrimSpace(consent.PrivacyNoticeVersion) == "" {
		return nil, ErrValidation
	}
	email = normalizeEmail(email)
	phone = normalizePhone(phone)
	if email == nil && phone == nil {
		return nil, ErrValidation
	}
	if email != nil {
		if _, err := mail.ParseAddress(*email); err != nil {
			return nil, ErrValidation
		}
	}
	now := time.Now().UTC()
	return &Lead{
		ID: uuid.NewString(), TenantID: tenantID, FirstName: firstName, LastName: lastName,
		Email: email, Phone: phone, Source: source, Stage: StageNew, Consent: consent,
		PreferredProgrammeIDs: []string{}, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// SetStage applies only contract-defined lifecycle values.
func (l *Lead) SetStage(stage LeadStage) error {
	if !validLeadStage(stage) {
		return ErrValidation
	}
	l.Stage = stage
	l.UpdatedAt = time.Now().UTC()
	return nil
}

func normalizeEmail(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	if normalized == "" {
		return nil
	}
	return &normalized
}

func normalizePhone(value *string) *string {
	if value == nil {
		return nil
	}
	var b strings.Builder
	for i, r := range strings.TrimSpace(*value) {
		if unicode.IsDigit(r) || (r == '+' && i == 0) {
			b.WriteRune(r)
		}
	}
	normalized := b.String()
	if len(strings.TrimPrefix(normalized, "+")) < 7 {
		return nil
	}
	return &normalized
}
