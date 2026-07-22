package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type growthRepoStub struct{ rows []domain.GrowthRollup }

func (r growthRepoStub) UpsertMetric(context.Context, string, *domain.Metric) error { return nil }
func (r growthRepoStub) ApplyMetricEvent(context.Context, string, string, string, []*domain.Metric) error {
	return nil
}
func (r growthRepoStub) ApplyAssessmentScoreEvent(context.Context, string, domain.AssessmentScoreEvent) error {
	return nil
}
func (r growthRepoStub) ListMetrics(context.Context, string, ports.ListFilter) ([]*domain.Metric, string, error) {
	return nil, "", nil
}
func (r growthRepoStub) ApplyGrowthEvent(context.Context, string, domain.GrowthEvent) error {
	return nil
}
func (r growthRepoStub) GrowthRollups(context.Context, string, string, string) ([]domain.GrowthRollup, error) {
	return r.rows, nil
}

func TestBuildGrowthExecutiveCalculatesExplainableFunnel(t *testing.T) {
	rows := []domain.GrowthRollup{
		{Stage: domain.GrowthLeads, Source: "website", Value: 100},
		{Stage: domain.GrowthApplicationsStarted, Source: "website", ProgrammeID: "programme-a", Value: 40},
		{Stage: domain.GrowthApplicationsDone, Source: "website", ProgrammeID: "programme-a", Value: 30},
		{Stage: domain.GrowthAdmitted, Source: "website", ProgrammeID: "programme-a", Value: 20},
		{Stage: domain.GrowthOffersIssued, Source: "website", ProgrammeID: "programme-a", Value: 18},
		{Stage: domain.GrowthOffersAccepted, Source: "website", ProgrammeID: "programme-a", Value: 15},
	}
	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	report := buildGrowthExecutive(rows, from, to, to.Add(12*time.Hour))
	if len(report.Funnel) != 6 || report.Funnel[1].ConversionFromLead == nil || *report.Funnel[1].ConversionFromLead != 40 {
		t.Fatalf("unexpected funnel: %+v", report.Funnel)
	}
	if report.Forecast.ProjectedOfferAcceptances != 15 || report.Forecast.Confidence != "medium" {
		t.Fatalf("unexpected forecast: %+v", report.Forecast)
	}
	if len(report.ByProgramme) != 1 || report.ByProgramme[0].OffersAccepted != 15 {
		t.Fatalf("unexpected programme breakdown: %+v", report.ByProgramme)
	}
}

func TestNormalizeGrowthRangeRejectsInvalidAndOversizedRanges(t *testing.T) {
	now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	if _, _, err := normalizeGrowthRange("2026-07-20", "2026-07-19", now); err == nil {
		t.Fatal("expected reversed range error")
	}
	if _, _, err := normalizeGrowthRange("2024-01-01", "2026-07-19", now); err == nil {
		t.Fatal("expected oversized range error")
	}
}

func TestGrowthExecutiveRequiresDedicatedPermissionAndFeature(t *testing.T) {
	gate := flags.NewStaticSnapshot()
	gate.Set("school-a", FeatureAnalytics, true)
	svc := NewService(growthRepoStub{}, WithFeatureGate(gate))
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
	viewer := auth.Actor{UserID: "viewer", TenantID: "school-a", Permissions: []string{PermRead}}
	if _, err := svc.GrowthExecutive(ctx, viewer, GrowthQuery{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("ordinary analytics viewer must be denied, got %v", err)
	}
	executive := auth.Actor{UserID: "executive", TenantID: "school-a", Permissions: []string{PermExecutiveRead}}
	if _, err := svc.GrowthExecutive(ctx, executive, GrowthQuery{}); err != nil {
		t.Fatalf("authorized executive denied: %v", err)
	}
	gate.Set("school-a", FeatureAnalytics, false)
	if _, err := svc.GrowthExecutive(ctx, executive, GrowthQuery{}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("disabled analytics feature must deny access, got %v", err)
	}
}

func TestExecutiveQuestionAlwaysReturnsSourcesRangeAndCalculation(t *testing.T) {
	gate := flags.NewStaticSnapshot()
	gate.Set("school-a", FeatureAnalytics, true)
	repo := growthRepoStub{rows: []domain.GrowthRollup{{Stage: domain.GrowthLeads, Source: "website", Value: 20}, {Stage: domain.GrowthOffersAccepted, Source: "website", ProgrammeID: "programme-a", Value: 4}}}
	svc := NewService(repo, WithFeatureGate(gate))
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
	actor := auth.Actor{UserID: "exec", TenantID: "school-a", Permissions: []string{PermExecutiveRead}}
	answer, err := svc.AskExecutive(ctx, actor, ExecutiveQuery{Question: "Which channel generated accepted offers?", From: "2026-07-01", To: "2026-07-19"})
	if err != nil {
		t.Fatal(err)
	}
	if answer.Confidence != "high" || len(answer.SourceDatasets) == 0 || len(answer.CalculationNotes) == 0 || answer.From != "2026-07-01" || answer.DashboardURL == "" {
		t.Fatalf("incomplete grounded answer: %+v", answer)
	}
}
