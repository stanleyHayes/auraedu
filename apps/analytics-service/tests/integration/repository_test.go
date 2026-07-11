package integration

import (
	"context"
	"testing"

	"github.com/auraedu/analytics-service/internal/adapters/postgres"
	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

func newRepo(t *testing.T) (ports.Repository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustMetric(t *testing.T, tenantID, name, bucketDate string, value float64, unit domain.Unit, dims domain.Dimensions) *domain.Metric {
	t.Helper()
	m, err := domain.NewMetric(tenantID, name, bucketDate, value, unit, dims)
	if err != nil {
		t.Fatalf("new metric: %v", err)
	}
	return m
}

func TestRepository_UpsertAndList(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	m := mustMetric(t, tenantA, "students.count", "2025-09-01", 1, domain.UnitCount, nil)
	if err := repo.UpsertMetric(ctx, tenantA, m); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 1 {
		t.Fatalf("expected value 1, got %v", metrics[0].Value)
	}
}

func TestRepository_UpsertIncrementsCount(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	for i := 0; i < 3; i++ {
		m := mustMetric(t, tenantA, "students.count", "2025-09-01", 1, domain.UnitCount, nil)
		if err := repo.UpsertMetric(ctx, tenantA, m); err != nil {
			t.Fatalf("upsert metric %d: %v", i, err)
		}
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 3 {
		t.Fatalf("expected value 3, got %v", metrics[0].Value)
	}
}

func TestRepository_UpsertAccumulatesSum(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	for _, amount := range []float64{10, 20, 30} {
		m := mustMetric(t, tenantA, "payments.total", "2025-09-01", amount, domain.UnitSum, nil)
		if err := repo.UpsertMetric(ctx, tenantA, m); err != nil {
			t.Fatalf("upsert sum: %v", err)
		}
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, MetricName: "payments.total"})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 60 {
		t.Fatalf("expected value 60, got %v", metrics[0].Value)
	}
}

func TestRepository_UpsertAverage(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	dims := domain.Dimensions{"subject_id": "sub-1", "academic_year_id": "ay-1"}
	for _, score := range []float64{80, 100} {
		m := mustMetric(t, tenantA, "assessments.avg_score", "2025-09-01", score, domain.UnitAverage, dims)
		if err := repo.UpsertMetric(ctx, tenantA, m); err != nil {
			t.Fatalf("upsert average: %v", err)
		}
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, MetricName: "assessments.avg_score"})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 90 {
		t.Fatalf("expected average 90, got %v", metrics[0].Value)
	}
	if metrics[0].SampleCount == nil || *metrics[0].SampleCount != 2 {
		t.Fatalf("expected sample_count 2, got %v", metrics[0].SampleCount)
	}
}

func TestRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	dims := domain.Dimensions{"subject_id": "sub-1", "academic_year_id": "ay-1"}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, tenantA, "students.count", "2025-09-01", 1, domain.UnitCount, nil)); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, tenantA, "students.count", "2025-09-02", 1, domain.UnitCount, nil)); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, tenantA, "assessments.avg_score", "2025-09-01", 80, domain.UnitAverage, dims)); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}

	cases := []struct {
		name   string
		filter ports.ListFilter
		want   int
	}{
		{"by metric_name", ports.ListFilter{Limit: 10, MetricName: "students.count"}, 2},
		{"by bucket_date_from", ports.ListFilter{Limit: 10, BucketDateFrom: "2025-09-02"}, 1},
		{"by bucket_date_to", ports.ListFilter{Limit: 10, BucketDateTo: "2025-09-01"}, 2},
		{"by dimension", ports.ListFilter{Limit: 10, DimensionKey: "subject_id", DimensionValue: "sub-1"}, 1},
		{"combined", ports.ListFilter{Limit: 10, MetricName: "students.count", BucketDateFrom: "2025-09-02"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListMetrics(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d metrics, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	for _, date := range []string{"2025-09-01", "2025-09-02"} {
		m := mustMetric(t, tenantA, "students.count", date, 1, domain.UnitCount, nil)
		if err := repo.UpsertMetric(ctx, tenantA, m); err != nil {
			t.Fatalf("upsert metric: %v", err)
		}
	}

	page, next, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 item on second page, got %d", len(page2))
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	if err := repo.UpsertMetric(aCtx, tenantA, mustMetric(t, tenantA, "students.count", "2025-09-01", 1, domain.UnitCount, nil)); err != nil {
		t.Fatalf("upsert tenant A metric: %v", err)
	}

	bCtx := withTenant(ctx, tenantB)
	metrics, _, err := repo.ListMetrics(bCtx, tenantB, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(metrics) != 0 {
		t.Fatalf("tenant B should see 0 metrics, got %d", len(metrics))
	}
}
