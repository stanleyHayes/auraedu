package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type scopeRepo struct {
	filter ports.ListFilter
}

func (*scopeRepo) UpsertMetric(context.Context, string, *domain.Metric) error { return nil }
func (*scopeRepo) ApplyMetricEvent(context.Context, string, string, string, []*domain.Metric) error {
	return nil
}
func (*scopeRepo) ApplyAssessmentScoreEvent(context.Context, string, domain.AssessmentScoreEvent) error {
	return nil
}
func (r *scopeRepo) ListMetrics(_ context.Context, _ string, filter ports.ListFilter) ([]*domain.Metric, string, error) {
	r.filter = filter
	return []*domain.Metric{}, "", nil
}
func (*scopeRepo) ApplyGrowthEvent(context.Context, string, domain.GrowthEvent) error { return nil }
func (*scopeRepo) GrowthRollups(context.Context, string, string, string) ([]domain.GrowthRollup, error) {
	return nil, nil
}

type fixedScope struct {
	studentIDs []string
	err        error
}

func (s fixedScope) Resolve(context.Context, string, string, string) (ports.LearnerScope, error) {
	return ports.LearnerScope{StudentIDs: s.studentIDs}, s.err
}

func analyticsContext() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-a"})
}

func analyticsGate() *flags.StaticSnapshot {
	gate := flags.NewStaticSnapshot()
	gate.Set("school-a", FeatureAnalytics, true)
	return gate
}

func TestTeacherMetricListIsRestrictedToAuthoritativeRoster(t *testing.T) {
	repo := &scopeRepo{}
	svc := NewService(repo, WithFeatureGate(analyticsGate()), WithLearnerScopeResolver(fixedScope{studentIDs: []string{"student-a", "student-b"}}))
	teacher := auth.Actor{UserID: "teacher-a", TenantID: "school-a", Role: "teacher", Permissions: []string{PermRead}}
	if _, _, err := svc.List(analyticsContext(), teacher, ports.ListFilter{MetricName: "assessments.count"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.filter.StudentIDs, []string{"student-a", "student-b"}) {
		t.Fatalf("repository scope = %+v", repo.filter.StudentIDs)
	}
}

func TestTeacherMetricListFailsClosedWithoutRosterDependency(t *testing.T) {
	repo := &scopeRepo{}
	svc := NewService(repo, WithFeatureGate(analyticsGate()))
	teacher := auth.Actor{UserID: "teacher-a", TenantID: "school-a", Role: "teacher", Permissions: []string{PermRead}}
	if _, _, err := svc.List(analyticsContext(), teacher, ports.ListFilter{}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
	if repo.filter.Limit != 0 {
		t.Fatal("repository must not be queried when scope resolution is unavailable")
	}
}

func TestSchoolAdminMetricListRemainsTenantScopedWithoutLearnerFilter(t *testing.T) {
	repo := &scopeRepo{}
	svc := NewService(repo, WithFeatureGate(analyticsGate()))
	admin := auth.Actor{UserID: "admin-a", TenantID: "school-a", Role: "school_admin", Permissions: []string{PermRead}}
	if _, _, err := svc.List(analyticsContext(), admin, ports.ListFilter{}); err != nil {
		t.Fatal(err)
	}
	if repo.filter.StudentIDs != nil {
		t.Fatalf("school admin unexpectedly received learner scope: %+v", repo.filter.StudentIDs)
	}
}
