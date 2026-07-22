package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OnboardingPending            = "pending_review"
	OnboardingApproved           = "approved"
	OnboardingRejected           = "rejected"
	OnboardingProvisioningFailed = "provisioning_failed"
)

// OnboardingRequest is a platform-owned intake record. It is deliberately not
// tenant-scoped because no tenant exists until a platform administrator approves it.
type OnboardingRequest struct {
	ID                   string     `json:"request_id"`
	SchoolName           string     `json:"school_name"`
	AdministratorName    string     `json:"administrator_name"`
	Email                string     `json:"email"`
	Phone                *string    `json:"phone"`
	CountryCode          string     `json:"country_code"`
	Plan                 string     `json:"plan"`
	Priorities           *string    `json:"priorities"`
	PrivacyNoticeVersion string     `json:"-"`
	Status               string     `json:"status"`
	TenantCode           *string    `json:"tenant_code"`
	DecisionReason       *string    `json:"decision_reason"`
	SubmittedAt          time.Time  `json:"submitted_at"`
	DecidedAt            *time.Time `json:"decided_at"`
}

type OnboardingRequestInput struct {
	SchoolName           string
	AdministratorName    string
	Email                string
	Phone                *string
	CountryCode          string
	Plan                 string
	Priorities           *string
	PrivacyNoticeVersion string
	AcceptedTerms        bool
	Website              string
}

func NewOnboardingRequest(input OnboardingRequestInput) (*OnboardingRequest, error) {
	request := &OnboardingRequest{
		ID:                   uuid.Must(uuid.NewV7()).String(),
		SchoolName:           strings.TrimSpace(input.SchoolName),
		AdministratorName:    strings.TrimSpace(input.AdministratorName),
		Email:                strings.ToLower(strings.TrimSpace(input.Email)),
		Phone:                trimOptional(input.Phone),
		CountryCode:          strings.ToUpper(strings.TrimSpace(input.CountryCode)),
		Plan:                 strings.ToLower(strings.TrimSpace(input.Plan)),
		Priorities:           trimOptional(input.Priorities),
		PrivacyNoticeVersion: strings.TrimSpace(input.PrivacyNoticeVersion),
		Status:               OnboardingPending,
		SubmittedAt:          time.Now().UTC(),
	}
	if !input.AcceptedTerms || strings.TrimSpace(input.Website) != "" {
		return nil, ErrValidation
	}
	if len(request.SchoolName) < 2 || len(request.SchoolName) > 200 || len(request.AdministratorName) < 2 || len(request.AdministratorName) > 160 {
		return nil, ErrValidation
	}
	if parsed, err := mail.ParseAddress(request.Email); err != nil || !strings.EqualFold(parsed.Address, request.Email) || len(request.Email) > 320 {
		return nil, ErrValidation
	}
	if len(request.CountryCode) != 2 || !asciiLetters(request.CountryCode) || !inSlice(request.Plan, ValidPlans()) || request.Plan == "core" {
		return nil, ErrValidation
	}
	invalidOptionalFields := request.PrivacyNoticeVersion == "" || len(request.PrivacyNoticeVersion) > 40 ||
		optionalLength(request.Phone) > 40 || optionalLength(request.Priorities) > 2000
	if invalidOptionalFields {
		return nil, ErrValidation
	}
	return request, nil
}

func (r OnboardingRequest) Tenant(code string) (Tenant, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return Tenant{}, ErrValidation
	}
	for _, ch := range code {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return Tenant{}, ErrValidation
		}
	}
	if r.Status != OnboardingPending {
		return Tenant{}, fmt.Errorf("%w: onboarding request is %s", ErrConflict, r.Status)
	}
	short := r.SchoolName
	if len(short) > 80 {
		short = short[:80]
	}
	return Tenant{Code: code, Name: r.SchoolName, Short: short, Status: "onboarding", Plan: r.Plan}, nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalLength(value *string) int {
	if value == nil {
		return 0
	}
	return len(*value)
}

func asciiLetters(value string) bool {
	for _, ch := range value {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}
	return true
}
