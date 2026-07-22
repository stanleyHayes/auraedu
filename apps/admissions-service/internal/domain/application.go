// Package domain defines Admissions entities and invariants.
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrValidation  = errors.New("admissions validation failed")
	ErrForbidden   = errors.New("admissions access forbidden")
	ErrNotFound    = errors.New("application not found")
	ErrConflict    = errors.New("application lifecycle conflict")
	ErrUnavailable = errors.New("admissions dependency unavailable")
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusSubmitted Status = "submitted"
	StatusAdmitted  Status = "admitted"
	StatusRejected  Status = "rejected"
	StatusWithdrawn Status = "withdrawn"
)

type Document struct {
	ID           string    `json:"id"`
	FileID       string    `json:"file_id"`
	DocumentType string    `json:"document_type"`
	FileName     string    `json:"file_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type Application struct {
	ID                   string         `json:"id"`
	TenantID             string         `json:"tenant_id"`
	ApplicantUserID      string         `json:"applicant_user_id"`
	LeadID               *string        `json:"lead_id"`
	ProgrammeID          string         `json:"programme_id"`
	IntakeID             string         `json:"intake_id"`
	ProgrammeName        string         `json:"programme_name"`
	IntakeName           string         `json:"intake_name"`
	LegalName            string         `json:"legal_name"`
	Email                string         `json:"email"`
	Phone                string         `json:"phone"`
	Answers              map[string]any `json:"answers"`
	Documents            []Document     `json:"documents"`
	Status               Status         `json:"status"`
	CompletionPercentage int            `json:"completion_percentage"`
	MissingRequirements  []string       `json:"missing_requirements"`
	SubmittedAt          *time.Time     `json:"submitted_at"`
	ReviewedBy           *string        `json:"reviewed_by"`
	ReviewedAt           *time.Time     `json:"reviewed_at"`
	ReviewNote           *string        `json:"review_note"`
	OfferStatus          string         `json:"offer_status"`
	OfferConditions      *string        `json:"offer_conditions"`
	OfferExpiresAt       *time.Time     `json:"offer_expires_at"`
	OfferIssuedBy        *string        `json:"offer_issued_by"`
	OfferAcceptedAt      *time.Time     `json:"offer_accepted_at"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

func New(tenantID, applicantID string, leadID *string, programmeID, intakeID, programmeName, intakeName string, now time.Time) (Application, error) {
	tenantID, applicantID = strings.TrimSpace(tenantID), strings.TrimSpace(applicantID)
	programmeName, intakeName = strings.TrimSpace(programmeName), strings.TrimSpace(intakeName)
	if tenantID == "" || applicantID == "" || uuid.Validate(programmeID) != nil || uuid.Validate(intakeID) != nil || programmeName == "" || intakeName == "" {
		return Application{}, ErrValidation
	}
	if leadID != nil && uuid.Validate(*leadID) != nil {
		return Application{}, ErrValidation
	}
	now = now.UTC()
	a := Application{
		ID: uuid.NewString(), TenantID: tenantID, ApplicantUserID: applicantID,
		LeadID: leadID, ProgrammeID: programmeID, IntakeID: intakeID,
		ProgrammeName: programmeName, IntakeName: intakeName,
		Answers: map[string]any{}, Documents: []Document{}, Status: StatusDraft,
		OfferStatus: "none", CreatedAt: now, UpdatedAt: now,
	}
	a.RefreshChecklist()
	return a, nil
}

func (a *Application) RefreshChecklist() {
	missing := []string{}
	if len(strings.TrimSpace(a.LegalName)) < 2 {
		missing = append(missing, "legal_name")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(a.Email)); err != nil {
		missing = append(missing, "email")
	}
	if len(strings.TrimSpace(a.Phone)) < 7 {
		missing = append(missing, "phone")
	}
	hasTranscript := false
	for _, document := range a.Documents {
		if document.DocumentType == "transcript" {
			hasTranscript = true
		}
	}
	if !hasTranscript {
		missing = append(missing, "transcript")
	}
	a.MissingRequirements = missing
	a.CompletionPercentage = (4 - len(missing)) * 25
}

func (a *Application) AttachDocument(fileID, documentType, fileName string, now time.Time) error {
	valid := map[string]bool{"transcript": true, "certificate": true, "identity": true, "passport_photo": true, "recommendation": true, "other": true}
	if a.Status != StatusDraft || uuid.Validate(fileID) != nil || !valid[documentType] || strings.TrimSpace(fileName) == "" || len(fileName) > 255 {
		return ErrValidation
	}
	for _, document := range a.Documents {
		if document.FileID == fileID {
			return ErrConflict
		}
	}
	a.Documents = append(a.Documents, Document{
		ID: uuid.NewString(), FileID: fileID, DocumentType: documentType,
		FileName: strings.TrimSpace(fileName), CreatedAt: now.UTC(),
	})
	a.UpdatedAt = now.UTC()
	a.RefreshChecklist()
	return nil
}

func (a *Application) Submit(now time.Time) error {
	if a.Status != StatusDraft {
		return ErrConflict
	}
	a.RefreshChecklist()
	if len(a.MissingRequirements) != 0 {
		return ErrValidation
	}
	now = now.UTC()
	a.Status = StatusSubmitted
	a.SubmittedAt = &now
	a.UpdatedAt = now
	return nil
}

func (a *Application) Review(decision, reviewer, note string, now time.Time) error {
	note = strings.TrimSpace(note)
	if a.Status != StatusSubmitted || reviewer == "" || len(note) < 3 || len(note) > 2000 {
		return ErrConflict
	}
	switch decision {
	case "admitted":
		a.Status = StatusAdmitted
	case "rejected":
		a.Status = StatusRejected
	default:
		return ErrValidation
	}
	now = now.UTC()
	a.ReviewedBy = &reviewer
	a.ReviewedAt = &now
	a.ReviewNote = &note
	a.UpdatedAt = now
	return nil
}

func (a *Application) IssueOffer(issuer, conditions string, expiresAt, now time.Time) error {
	conditions = strings.TrimSpace(conditions)
	if a.Status != StatusAdmitted || a.OfferStatus != "none" || issuer == "" || len(conditions) < 3 || len(conditions) > 5000 || !expiresAt.After(now) {
		return ErrConflict
	}
	expiresAt = expiresAt.UTC()
	now = now.UTC()
	a.OfferStatus = "issued"
	a.OfferConditions = &conditions
	a.OfferExpiresAt = &expiresAt
	a.OfferIssuedBy = &issuer
	a.UpdatedAt = now
	return nil
}

func (a *Application) AcceptOffer(actor string, now time.Time) error {
	if actor != a.ApplicantUserID || a.OfferStatus != "issued" || a.OfferExpiresAt == nil || !a.OfferExpiresAt.After(now) {
		return ErrConflict
	}
	now = now.UTC()
	a.OfferStatus = "accepted"
	a.OfferAcceptedAt = &now
	a.UpdatedAt = now
	return nil
}
