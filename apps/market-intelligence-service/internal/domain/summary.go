//nolint:lll // Summary policy constructors keep bounded audit fields visible together.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type SummaryItem struct {
	SourceID               string     `json:"source_id"`
	Category               string     `json:"category"`
	ProgrammeID            *string    `json:"programme_id"`
	CampusID               *string    `json:"campus_id"`
	ChangeType             string     `json:"change_type"`
	LatestTitle            string     `json:"latest_title"`
	LatestExcerpt          string     `json:"latest_excerpt"`
	LatestEvidenceSHA256   string     `json:"latest_evidence_sha256"`
	LatestObservedAt       time.Time  `json:"latest_observed_at"`
	PreviousExcerpt        *string    `json:"previous_excerpt"`
	PreviousEvidenceSHA256 *string    `json:"previous_evidence_sha256"`
	PreviousObservedAt     *time.Time `json:"previous_observed_at"`
}
type CompetitorSummary struct {
	ID          string        `json:"id"`
	TenantID    string        `json:"tenant_id"`
	PeriodFrom  time.Time     `json:"period_from"`
	PeriodTo    time.Time     `json:"period_to"`
	Status      Status        `json:"status"`
	Items       []SummaryItem `json:"items"`
	ItemCount   int           `json:"item_count"`
	SourceCount int           `json:"source_count"`
	GeneratedBy string        `json:"generated_by"`
	ReviewedBy  *string       `json:"reviewed_by"`
	ReviewedAt  *time.Time    `json:"reviewed_at"`
	ReviewNote  *string       `json:"review_note"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

func boundedExcerpt(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 280 {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:277])) + "..."
}
func NewCompetitorSummary(tenant, actor string, from, to, now time.Time, items []SummaryItem) (CompetitorSummary, error) {
	tenant, actor = strings.TrimSpace(tenant), strings.TrimSpace(actor)
	if tenant == "" || actor == "" || from.IsZero() || to.IsZero() || !to.After(from) || to.Sub(from) > 366*24*time.Hour || to.After(now.Add(5*time.Minute)) {
		return CompetitorSummary{}, ErrValidation
	}
	sources := map[string]bool{}
	for i := range items {
		items[i].LatestExcerpt = boundedExcerpt(items[i].LatestExcerpt)
		if items[i].PreviousExcerpt != nil {
			v := boundedExcerpt(*items[i].PreviousExcerpt)
			items[i].PreviousExcerpt = &v
		}
		if items[i].ChangeType != "first_seen" && items[i].ChangeType != "changed" {
			return CompetitorSummary{}, ErrValidation
		}
		sources[items[i].SourceID] = true
	}
	now = now.UTC()
	return CompetitorSummary{ID: uuid.NewString(), TenantID: tenant, PeriodFrom: from.UTC(), PeriodTo: to.UTC(), Status: StatusPending, Items: items, ItemCount: len(items), SourceCount: len(sources), GeneratedBy: actor, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *CompetitorSummary) Review(actor, decision, note string, now time.Time) error {
	actor, decision, note = strings.TrimSpace(actor), strings.TrimSpace(decision), strings.TrimSpace(note)
	if s.Status != StatusPending || actor == "" || actor == s.GeneratedBy || (decision != "approved" && decision != "rejected") || len(note) < 3 || len(note) > 1000 {
		return ErrConflict
	}
	now = now.UTC()
	s.Status = Status(decision)
	s.ReviewedBy = &actor
	s.ReviewedAt = &now
	s.ReviewNote = &note
	s.UpdatedAt = now
	return nil
}
