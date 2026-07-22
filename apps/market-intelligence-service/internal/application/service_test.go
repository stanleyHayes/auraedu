package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/application"
	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type memoryRepo struct {
	sources      map[string]domain.Source
	observations map[string]domain.Observation
	rule         domain.AlertRule
	alerts       map[string]domain.Alert
	summaries    map[string]domain.CompetitorSummary
}

func memory() *memoryRepo {
	return &memoryRepo{sources: map[string]domain.Source{}, observations: map[string]domain.Observation{}, alerts: map[string]domain.Alert{}, summaries: map[string]domain.CompetitorSummary{}}
}
func (r *memoryRepo) CreateSource(_ context.Context, v domain.Source, _ string, _ map[string]any) error {
	r.sources[v.ID] = v
	return nil
}
func (r *memoryRepo) GetSource(_ context.Context, t, id string) (domain.Source, error) {
	v, ok := r.sources[id]
	if !ok || v.TenantID != t {
		return domain.Source{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *memoryRepo) ListSources(_ context.Context, t string, k domain.Kind, _ int) ([]domain.Source, error) {
	out := []domain.Source{}
	for _, v := range r.sources {
		if v.TenantID == t && v.Kind == k {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) UpdateSource(_ context.Context, v domain.Source, expected domain.Status, _ string, _ map[string]any) error {
	current, ok := r.sources[v.ID]
	if !ok || current.ComplianceStatus != expected {
		return domain.ErrConflict
	}
	r.sources[v.ID] = v
	return nil
}
func (r *memoryRepo) CreateObservation(_ context.Context, v domain.Observation, _ string, _ map[string]any) error {
	r.observations[v.ID] = v
	return nil
}
func (r *memoryRepo) GetObservation(_ context.Context, t, id string) (domain.Observation, error) {
	v, ok := r.observations[id]
	if !ok || v.TenantID != t {
		return domain.Observation{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *memoryRepo) ListObservations(_ context.Context, t string, k domain.Kind, s domain.Status, _ int) ([]domain.Observation, error) {
	out := []domain.Observation{}
	for _, v := range r.observations {
		if v.TenantID == t && v.Kind == k && (s == "" || v.Status == s) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) UpdateObservation(_ context.Context, v domain.Observation, expected domain.Status, _ string, _ map[string]any) error {
	current, ok := r.observations[v.ID]
	if !ok || current.Status != expected {
		return domain.ErrConflict
	}
	r.observations[v.ID] = v
	return nil
}
func (r *memoryRepo) GetAlertRule(_ context.Context, tenant string) (domain.AlertRule, error) {
	if r.rule.TenantID == "" {
		return domain.AlertRule{TenantID: tenant, Threshold: 3, WindowDays: 30, UpdatedBy: "system-default"}, nil
	}
	return r.rule, nil
}
func (r *memoryRepo) UpsertAlertRule(_ context.Context, value domain.AlertRule, _ string, _ map[string]any) error {
	r.rule = value
	return nil
}
func (r *memoryRepo) ListAlerts(_ context.Context, tenant, status string, _ int) ([]domain.Alert, error) {
	out := []domain.Alert{}
	for _, value := range r.alerts {
		if value.TenantID == tenant && (status == "" || value.Status == status) {
			out = append(out, value)
		}
	}
	return out, nil
}
func (r *memoryRepo) GetAlert(_ context.Context, tenant, id string) (domain.Alert, error) {
	value, ok := r.alerts[id]
	if !ok || value.TenantID != tenant {
		return domain.Alert{}, domain.ErrNotFound
	}
	return value, nil
}
func (r *memoryRepo) AcknowledgeAlert(_ context.Context, value domain.Alert, _ string, _ map[string]any) error {
	r.alerts[value.ID] = value
	return nil
}
func (r *memoryRepo) BuildSummaryItems(context.Context, string, time.Time, time.Time) ([]domain.SummaryItem, error) {
	return []domain.SummaryItem{}, nil
}
func (r *memoryRepo) CreateSummary(_ context.Context, v domain.CompetitorSummary, _ string, _ map[string]any) error {
	r.summaries[v.ID] = v
	return nil
}
func (r *memoryRepo) GetSummary(_ context.Context, t, id string) (domain.CompetitorSummary, error) {
	v, ok := r.summaries[id]
	if !ok || v.TenantID != t {
		return domain.CompetitorSummary{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *memoryRepo) ListSummaries(_ context.Context, t string, s domain.Status, _ int) ([]domain.CompetitorSummary, error) {
	out := []domain.CompetitorSummary{}
	for _, v := range r.summaries {
		if v.TenantID == t && (s == "" || v.Status == s) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) UpdateSummary(_ context.Context, v domain.CompetitorSummary, expected domain.Status, _ string, _ map[string]any) error {
	current, ok := r.summaries[v.ID]
	if !ok || current.Status != expected {
		return domain.ErrConflict
	}
	r.summaries[v.ID] = v
	return nil
}
func actor(id, role string, permissions ...string) auth.Actor {
	return auth.Actor{UserID: id, TenantID: "school-one", Role: role, Permissions: permissions}
}

func TestServicePreventsAIAgentApprovalAndExposesNoPublishOperation(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repo := memory()
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	creator := actor("researcher", "growth_analyst", application.PermManage, application.PermRead)
	source, err := svc.CreateSource(context.Background(), creator, application.CreateSourceInput{Kind: domain.KindReputation, Name: "Official review page", CanonicalURL: "https://reviews.example.edu/school", CollectionMethod: "manual", TermsReference: "Manual use approved"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ReviewSource(context.Background(), actor("agent", "ai_agent", application.PermReview), source.ID, "approved", "Verified by AI"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AI approved source: %v", err)
	}
	source, err = svc.ReviewSource(context.Background(), actor("legal", "compliance_officer", application.PermReview), source.ID, "approved", "Manual collection authority verified")
	if err != nil {
		t.Fatal(err)
	}
	observation, err := svc.CreateObservation(context.Background(), creator, application.CreateObservationInput{SourceID: source.ID, Category: "mention", Title: "Application deadline confusion", EvidenceExcerpt: "Public question about the application deadline.", Sentiment: "neutral", ResponseDraft: "The official deadline is available on our programme page.", ObservedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ReviewObservation(context.Background(), actor("agent", "service_account", application.PermReview), observation.ID, "approved", "Looks good"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("service account approved observation: %v", err)
	}
	approved, err := svc.ReviewObservation(context.Background(), actor("editor", "communications_lead", application.PermReview), observation.ID, "approved", "Evidence verified; response remains an internal draft")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != domain.StatusApproved || approved.ResponseDraft == "" {
		t.Fatalf("unexpected approved observation: %+v", approved)
	}
}
