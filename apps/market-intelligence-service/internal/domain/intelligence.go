// Package domain defines market-intelligence entities and lifecycle rules.
//
//nolint:lll // Audit JSON mirrors and policy constructors keep related fields visible together.
package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrValidation = errors.New("market intelligence validation failed")
	ErrForbidden  = errors.New("market intelligence access forbidden")
	ErrNotFound   = errors.New("market intelligence record not found")
	ErrConflict   = errors.New("market intelligence lifecycle conflict")
)

type Kind string
type Status string

const (
	KindReputation Kind   = "reputation"
	KindCompetitor Kind   = "competitor"
	StatusPending  Status = "pending_review"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusResolved Status = "resolved"
)

type Source struct {
	ID, TenantID, Name, CanonicalURL, CollectionMethod, TermsReference, CreatedBy string
	Kind                                                                          Kind
	ComplianceStatus                                                              Status
	ReviewedBy, ReviewNote                                                        *string
	ReviewedAt                                                                    *time.Time
	CreatedAt, UpdatedAt                                                          time.Time
}

func (s Source) MarshalJSON() ([]byte, error) { return marshalSource(s) }

type sourceJSON struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	Kind             Kind       `json:"kind"`
	Name             string     `json:"name"`
	CanonicalURL     string     `json:"canonical_url"`
	CollectionMethod string     `json:"collection_method"`
	TermsReference   string     `json:"terms_reference"`
	ComplianceStatus Status     `json:"compliance_status"`
	CreatedBy        string     `json:"created_by"`
	ReviewedBy       *string    `json:"reviewed_by"`
	ReviewedAt       *time.Time `json:"reviewed_at"`
	ReviewNote       *string    `json:"review_note"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func marshalSource(s Source) ([]byte, error) {
	return json.Marshal(sourceJSON{s.ID, s.TenantID, s.Kind, s.Name, s.CanonicalURL, s.CollectionMethod, s.TermsReference, s.ComplianceStatus, s.CreatedBy, s.ReviewedBy, s.ReviewedAt, s.ReviewNote, s.CreatedAt, s.UpdatedAt})
}

type Observation struct {
	ID, TenantID, SourceID, Category, Title, EvidenceExcerpt, EvidenceSHA256, Sentiment, ResponseDraft, CreatedBy string
	Kind                                                                                                          Kind
	ProgrammeID, CampusID, ReviewedBy, ReviewNote, ResolutionNote, ResolvedBy                                     *string
	Status                                                                                                        Status
	ObservedAt                                                                                                    time.Time
	ReviewedAt, ResolvedAt                                                                                        *time.Time
	CreatedAt, UpdatedAt                                                                                          time.Time
}

type observationJSON struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	SourceID        string     `json:"source_id"`
	Kind            Kind       `json:"kind"`
	Category        string     `json:"category"`
	Title           string     `json:"title"`
	EvidenceExcerpt string     `json:"evidence_excerpt"`
	EvidenceSHA256  string     `json:"evidence_sha256"`
	Sentiment       string     `json:"sentiment"`
	ProgrammeID     *string    `json:"programme_id"`
	CampusID        *string    `json:"campus_id"`
	ResponseDraft   string     `json:"response_draft"`
	Status          Status     `json:"status"`
	CreatedBy       string     `json:"created_by"`
	ObservedAt      time.Time  `json:"observed_at"`
	ReviewedBy      *string    `json:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	ReviewNote      *string    `json:"review_note"`
	ResolutionNote  *string    `json:"resolution_note"`
	ResolvedBy      *string    `json:"resolved_by"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (o Observation) MarshalJSON() ([]byte, error) {
	return json.Marshal(observationJSON{o.ID, o.TenantID, o.SourceID, o.Kind, o.Category, o.Title, o.EvidenceExcerpt, o.EvidenceSHA256, o.Sentiment, o.ProgrammeID, o.CampusID, o.ResponseDraft, o.Status, o.CreatedBy, o.ObservedAt, o.ReviewedBy, o.ReviewedAt, o.ReviewNote, o.ResolutionNote, o.ResolvedBy, o.ResolvedAt, o.CreatedAt, o.UpdatedAt})
}

func NewSource(tenant string, kind Kind, name, canonicalURL, method, terms, actor string, now time.Time) (Source, error) {
	tenant, name, canonicalURL, method, terms, actor = strings.TrimSpace(tenant), strings.TrimSpace(name), strings.TrimSpace(canonicalURL), strings.TrimSpace(method), strings.TrimSpace(terms), strings.TrimSpace(actor)
	u, err := url.ParseRequestURI(canonicalURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (kind != KindReputation && kind != KindCompetitor) || (method != "manual" && method != "official_api") || tenant == "" || actor == "" || len(name) < 3 || len(name) > 160 || len(canonicalURL) > 2048 || len(terms) < 3 || len(terms) > 1000 {
		return Source{}, ErrValidation
	}
	now = now.UTC()
	return Source{ID: uuid.NewString(), TenantID: tenant, Kind: kind, Name: name, CanonicalURL: canonicalURL, CollectionMethod: method, TermsReference: terms, ComplianceStatus: StatusPending, CreatedBy: actor, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Source) Review(actor, decision, note string, now time.Time) error {
	actor, decision, note = strings.TrimSpace(actor), strings.TrimSpace(decision), strings.TrimSpace(note)
	if s.ComplianceStatus != StatusPending || actor == "" || actor == s.CreatedBy || (decision != "approved" && decision != "rejected") || len(note) < 3 || len(note) > 1000 {
		return ErrConflict
	}
	status := Status(decision)
	now = now.UTC()
	s.ComplianceStatus = status
	s.ReviewedBy = &actor
	s.ReviewedAt = &now
	s.ReviewNote = &note
	s.UpdatedAt = now
	return nil
}

func NewObservation(source Source, category, title, evidence, sentiment string, programmeID, campusID *string, responseDraft, actor string, observedAt, now time.Time) (Observation, error) {
	category, title, evidence, sentiment, responseDraft, actor = strings.TrimSpace(category), strings.TrimSpace(title), strings.TrimSpace(evidence), strings.TrimSpace(sentiment), strings.TrimSpace(responseDraft), strings.TrimSpace(actor)
	if sentiment == "" {
		sentiment = "unknown"
	}
	if source.ComplianceStatus != StatusApproved || categoryKind(category) != source.Kind || !validSentiment(sentiment) || actor == "" || len(title) < 3 || len(title) > 240 || len(evidence) < 3 || len(evidence) > 1000 || len(responseDraft) > 4000 || observedAt.IsZero() || observedAt.After(now.Add(5*time.Minute)) {
		return Observation{}, ErrValidation
	}
	for _, id := range []*string{programmeID, campusID} {
		if id != nil {
			v := strings.TrimSpace(*id)
			if _, err := uuid.Parse(v); err != nil {
				return Observation{}, ErrValidation
			}
			*id = v
		}
	}
	hash := sha256.Sum256([]byte(evidence))
	now = now.UTC()
	return Observation{ID: uuid.NewString(), TenantID: source.TenantID, SourceID: source.ID, Kind: source.Kind, Category: category, Title: title, EvidenceExcerpt: evidence, EvidenceSHA256: fmt.Sprintf("%x", hash), Sentiment: sentiment, ProgrammeID: programmeID, CampusID: campusID, ResponseDraft: responseDraft, Status: StatusPending, CreatedBy: actor, ObservedAt: observedAt.UTC(), CreatedAt: now, UpdatedAt: now}, nil
}

func categoryKind(category string) Kind {
	switch category {
	case "mention", "recurring_issue", "misinformation":
		return KindReputation
	case "programme", "fee", "scholarship", "deadline", "campaign":
		return KindCompetitor
	default:
		return ""
	}
}

func validSentiment(sentiment string) bool {
	switch sentiment {
	case "positive", "neutral", "negative", "unknown":
		return true
	default:
		return false
	}
}

func (o *Observation) Review(actor, decision, note string, now time.Time) error {
	actor, decision, note = strings.TrimSpace(actor), strings.TrimSpace(decision), strings.TrimSpace(note)
	if o.Status != StatusPending || actor == "" || actor == o.CreatedBy || (decision != "approved" && decision != "rejected") || len(note) < 3 || len(note) > 1000 {
		return ErrConflict
	}
	now = now.UTC()
	o.Status = Status(decision)
	o.ReviewedBy = &actor
	o.ReviewedAt = &now
	o.ReviewNote = &note
	o.UpdatedAt = now
	return nil
}
func (o *Observation) Resolve(actor, note string, now time.Time) error {
	actor, note = strings.TrimSpace(actor), strings.TrimSpace(note)
	if o.Kind != KindReputation || o.Status != StatusApproved || actor == "" || len(note) < 3 || len(note) > 2000 {
		return ErrConflict
	}
	now = now.UTC()
	o.Status = StatusResolved
	o.ResolutionNote = &note
	o.ResolvedBy = &actor
	o.ResolvedAt = &now
	o.UpdatedAt = now
	return nil
}
