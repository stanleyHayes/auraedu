// Package application coordinates governed market-intelligence workflows.
package application

import (
	"context"
	"strings"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/market-intelligence-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

const (
	PermRead   = "intelligence.read"
	PermManage = "intelligence.manage"
	PermReview = "intelligence.review"
)

type Service struct {
	repo ports.Repository
	gate flags.Gate
	now  func() time.Time
}
type Option func(*Service)

func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gate = g } }
func WithClock(n func() time.Time) Option { return func(s *Service) { s.now = n } }
func NewService(r ports.Repository, opts ...Option) *Service {
	s := &Service{repo: r, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}
func feature(k domain.Kind) string {
	if k == domain.KindReputation {
		return "growth_reputation_monitor"
	}
	return "growth_competitor_monitor"
}
func validKind(k domain.Kind) bool { return k == domain.KindReputation || k == domain.KindCompetitor }
func isAI(a auth.Actor) bool {
	v := strings.ToLower(a.Role)
	return strings.Contains(v, "ai") || strings.Contains(v, "service_account")
}
func (s *Service) allowed(ctx context.Context, a auth.Actor, p string, k domain.Kind) bool {
	return validKind(k) && a.Authenticated() && a.TenantID != "" && a.Has(p) && (s.gate == nil || s.gate.IsEnabled(ctx, a.TenantID, feature(k)))
}
func limit(v int) int {
	if v < 1 || v > 100 {
		return 50
	}
	return v
}

type CreateSourceInput struct {
	Kind             domain.Kind `json:"kind"`
	Name             string      `json:"name"`
	CanonicalURL     string      `json:"canonical_url"`
	CollectionMethod string      `json:"collection_method"`
	TermsReference   string      `json:"terms_reference"`
}

func (s *Service) CreateSource(ctx context.Context, a auth.Actor, in CreateSourceInput) (domain.Source, error) {
	if !s.allowed(ctx, a, PermManage, in.Kind) {
		return domain.Source{}, domain.ErrForbidden
	}
	item, e := domain.NewSource(a.TenantID, in.Kind, in.Name, in.CanonicalURL, in.CollectionMethod, in.TermsReference, a.UserID, s.now())
	if e != nil {
		return domain.Source{}, e
	}
	e = s.repo.CreateSource(ctx, item, "intelligence.source.created.v1", ports.LifecycleEventData(item.ID, item.Kind, a.UserID, item.CreatedAt))
	return item, e
}
func (s *Service) ListSources(ctx context.Context, a auth.Actor, k domain.Kind, n int) ([]domain.Source, error) {
	if !s.allowed(ctx, a, PermRead, k) {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListSources(ctx, a.TenantID, k, limit(n))
}
func (s *Service) ReviewSource(ctx context.Context, a auth.Actor, id, decision, note string) (domain.Source, error) {
	item, e := s.repo.GetSource(ctx, a.TenantID, id)
	if e != nil {
		return domain.Source{}, e
	}
	if !s.allowed(ctx, a, PermReview, item.Kind) || isAI(a) {
		return domain.Source{}, domain.ErrForbidden
	}
	expected := item.ComplianceStatus
	if e = item.Review(a.UserID, decision, note, s.now()); e != nil {
		return domain.Source{}, e
	}
	e = s.repo.UpdateSource(ctx, item, expected, "intelligence.source.reviewed.v1", ports.LifecycleEventData(item.ID, item.Kind, a.UserID, item.UpdatedAt))
	return item, e
}

type CreateObservationInput struct {
	SourceID        string    `json:"source_id"`
	Category        string    `json:"category"`
	Title           string    `json:"title"`
	EvidenceExcerpt string    `json:"evidence_excerpt"`
	Sentiment       string    `json:"sentiment"`
	ProgrammeID     *string   `json:"programme_id"`
	CampusID        *string   `json:"campus_id"`
	ResponseDraft   string    `json:"response_draft"`
	ObservedAt      time.Time `json:"observed_at"`
}

func (s *Service) CreateObservation(ctx context.Context, a auth.Actor, in CreateObservationInput) (domain.Observation, error) {
	source, e := s.repo.GetSource(ctx, a.TenantID, in.SourceID)
	if e != nil {
		return domain.Observation{}, e
	}
	if !s.allowed(ctx, a, PermManage, source.Kind) {
		return domain.Observation{}, domain.ErrForbidden
	}
	item, e := domain.NewObservation(
		source, in.Category, in.Title, in.EvidenceExcerpt, in.Sentiment,
		in.ProgrammeID, in.CampusID, in.ResponseDraft, a.UserID, in.ObservedAt, s.now(),
	)
	if e != nil {
		return domain.Observation{}, e
	}
	e = s.repo.CreateObservation(ctx, item, "intelligence.observation.created.v1", ports.LifecycleEventData(item.ID, item.Kind, a.UserID, item.CreatedAt))
	return item, e
}
func (s *Service) ListObservations(ctx context.Context, a auth.Actor, k domain.Kind, status domain.Status, n int) ([]domain.Observation, error) {
	if !s.allowed(ctx, a, PermRead, k) {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListObservations(ctx, a.TenantID, k, status, limit(n))
}
func (s *Service) ReviewObservation(ctx context.Context, a auth.Actor, id, decision, note string) (domain.Observation, error) {
	item, e := s.repo.GetObservation(ctx, a.TenantID, id)
	if e != nil {
		return domain.Observation{}, e
	}
	if !s.allowed(ctx, a, PermReview, item.Kind) || isAI(a) {
		return domain.Observation{}, domain.ErrForbidden
	}
	expected := item.Status
	if e = item.Review(a.UserID, decision, note, s.now()); e != nil {
		return domain.Observation{}, e
	}
	payload := ports.LifecycleEventData(item.ID, item.Kind, a.UserID, item.UpdatedAt)
	e = s.repo.UpdateObservation(ctx, item, expected, "intelligence.observation.reviewed.v1", payload)
	return item, e
}
func (s *Service) ResolveObservation(ctx context.Context, a auth.Actor, id, note string) (domain.Observation, error) {
	item, e := s.repo.GetObservation(ctx, a.TenantID, id)
	if e != nil {
		return domain.Observation{}, e
	}
	if !s.allowed(ctx, a, PermManage, item.Kind) || isAI(a) {
		return domain.Observation{}, domain.ErrForbidden
	}
	expected := item.Status
	if e = item.Resolve(a.UserID, note, s.now()); e != nil {
		return domain.Observation{}, e
	}
	payload := ports.LifecycleEventData(item.ID, item.Kind, a.UserID, item.UpdatedAt)
	e = s.repo.UpdateObservation(ctx, item, expected, "intelligence.observation.resolved.v1", payload)
	return item, e
}

func (s *Service) GetAlertRule(ctx context.Context, a auth.Actor) (domain.AlertRule, error) {
	if !s.allowed(ctx, a, PermRead, domain.KindReputation) {
		return domain.AlertRule{}, domain.ErrForbidden
	}
	return s.repo.GetAlertRule(ctx, a.TenantID)
}
func (s *Service) UpdateAlertRule(ctx context.Context, a auth.Actor, threshold, windowDays int) (domain.AlertRule, error) {
	if !s.allowed(ctx, a, PermManage, domain.KindReputation) || isAI(a) {
		return domain.AlertRule{}, domain.ErrForbidden
	}
	rule, e := domain.NewAlertRule(a.TenantID, threshold, windowDays, a.UserID, s.now())
	if e != nil {
		return domain.AlertRule{}, e
	}
	payload := ports.LifecycleEventData(a.TenantID, domain.KindReputation, a.UserID, rule.UpdatedAt)
	e = s.repo.UpsertAlertRule(ctx, rule, "intelligence.alert_rule.updated.v1", payload)
	return rule, e
}
func (s *Service) ListAlerts(ctx context.Context, a auth.Actor, status string, n int) ([]domain.Alert, error) {
	if !s.allowed(ctx, a, PermRead, domain.KindReputation) {
		return nil, domain.ErrForbidden
	}
	if status != "" && status != "open" && status != "acknowledged" {
		return nil, domain.ErrValidation
	}
	return s.repo.ListAlerts(ctx, a.TenantID, status, limit(n))
}
func (s *Service) AcknowledgeAlert(ctx context.Context, a auth.Actor, id, note string) (domain.Alert, error) {
	if !s.allowed(ctx, a, PermManage, domain.KindReputation) || isAI(a) {
		return domain.Alert{}, domain.ErrForbidden
	}
	alert, e := s.repo.GetAlert(ctx, a.TenantID, id)
	if e != nil {
		return domain.Alert{}, e
	}
	if e = alert.Acknowledge(a.UserID, note, s.now()); e != nil {
		return domain.Alert{}, e
	}
	payload := ports.LifecycleEventData(alert.ID, domain.KindReputation, a.UserID, alert.UpdatedAt)
	e = s.repo.AcknowledgeAlert(ctx, alert, "intelligence.alert.acknowledged.v1", payload)
	return alert, e
}

func (s *Service) GenerateSummary(ctx context.Context, a auth.Actor, from, to time.Time) (domain.CompetitorSummary, error) {
	if !s.allowed(ctx, a, PermManage, domain.KindCompetitor) || isAI(a) {
		return domain.CompetitorSummary{}, domain.ErrForbidden
	}
	items, e := s.repo.BuildSummaryItems(ctx, a.TenantID, from, to)
	if e != nil {
		return domain.CompetitorSummary{}, e
	}
	summary, e := domain.NewCompetitorSummary(a.TenantID, a.UserID, from, to, s.now(), items)
	if e != nil {
		return domain.CompetitorSummary{}, e
	}
	event := ports.LifecycleEventData(summary.ID, domain.KindCompetitor, a.UserID, summary.CreatedAt)
	event["item_count"] = summary.ItemCount
	event["source_count"] = summary.SourceCount
	e = s.repo.CreateSummary(ctx, summary, "intelligence.competitor_summary.created.v1", event)
	return summary, e
}
func (s *Service) ListSummaries(ctx context.Context, a auth.Actor, status domain.Status, n int) ([]domain.CompetitorSummary, error) {
	if !s.allowed(ctx, a, PermRead, domain.KindCompetitor) {
		return nil, domain.ErrForbidden
	}
	if status != "" && status != domain.StatusPending && status != domain.StatusApproved && status != domain.StatusRejected {
		return nil, domain.ErrValidation
	}
	return s.repo.ListSummaries(ctx, a.TenantID, status, limit(n))
}
func (s *Service) ReviewSummary(ctx context.Context, a auth.Actor, id, decision, note string) (domain.CompetitorSummary, error) {
	summary, e := s.repo.GetSummary(ctx, a.TenantID, id)
	if e != nil {
		return domain.CompetitorSummary{}, e
	}
	if !s.allowed(ctx, a, PermReview, domain.KindCompetitor) || isAI(a) {
		return domain.CompetitorSummary{}, domain.ErrForbidden
	}
	expected := summary.Status
	if e = summary.Review(a.UserID, decision, note, s.now()); e != nil {
		return domain.CompetitorSummary{}, e
	}
	payload := ports.LifecycleEventData(summary.ID, domain.KindCompetitor, a.UserID, summary.UpdatedAt)
	e = s.repo.UpdateSummary(ctx, summary, expected, "intelligence.competitor_summary.reviewed.v1", payload)
	return summary, e
}
