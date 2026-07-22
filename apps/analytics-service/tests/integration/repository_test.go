package integration

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/analytics-service/internal/adapters/postgres"
	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

func newRepo(t *testing.T) ports.Repository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB)
}

func TestRepository_GrowthProjectionIsAtomicIdempotentAndTenantSafe(t *testing.T) {
	repo := newRepo(t)
	ctx := withTenant(context.Background(), tenantA)
	events := []domain.GrowthEvent{
		{EventID: "growth-app", EventType: "application.started.v1", Stage: domain.GrowthApplicationsStarted, BucketDate: "2025-09-01", LeadID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ApplicationID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", ProgrammeID: "cccccccc-cccc-cccc-cccc-cccccccccccc", IntakeID: "dddddddd-dddd-dddd-dddd-dddddddddddd", OccurredAt: time.Date(2025, 9, 1, 11, 0, 0, 0, time.UTC)},
		{EventID: "growth-lead", EventType: "lead.created.v1", Stage: domain.GrowthLeads, BucketDate: "2025-09-01", LeadID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Source: "website", CampaignID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", OccurredAt: time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)},
	}
	for _, event := range events {
		if err := repo.ApplyGrowthEvent(ctx, tenantA, event); err != nil {
			t.Fatalf("apply %s: %v", event.Stage, err)
		}
		if err := repo.ApplyGrowthEvent(ctx, tenantA, event); err != nil {
			t.Fatalf("redeliver %s: %v", event.Stage, err)
		}
	}
	rows, err := repo.GrowthRollups(ctx, tenantA, "2025-09-01", "2025-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two stage rows, got %+v", rows)
	}
	for _, row := range rows {
		if row.Value != 1 || row.Source != "website" {
			t.Fatalf("unexpected rollup: %+v", row)
		}
	}
	other, err := repo.GrowthRollups(withTenant(context.Background(), tenantB), tenantB, "2025-09-01", "2025-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("tenant B saw tenant A growth rows: %+v", other)
	}
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustMetric(t *testing.T, name, bucketDate string, value float64, unit domain.Unit, dims domain.Dimensions) *domain.Metric {
	t.Helper()
	m, err := domain.NewMetric(tenantA, name, bucketDate, value, unit, dims)
	if err != nil {
		t.Fatalf("new metric: %v", err)
	}
	return m
}

func TestRepository_UpsertAndList(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	m := mustMetric(t, "students.count", "2025-09-01", 1, domain.UnitCount, nil)
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

func TestRepository_MetricEventIsAtomicIdempotentAndTenantSafe(t *testing.T) {
	repo := newRepo(t)
	aCtx := withTenant(context.Background(), tenantA)
	metrics := []*domain.Metric{
		mustMetric(t, "assessments.count", "2025-09-01", 1, domain.UnitCount, domain.Dimensions{"subject_id": "subject-a"}),
		mustMetric(t, "assessments.avg_percentage", "2025-09-01", 72, domain.UnitAverage, domain.Dimensions{"subject_id": "subject-a"}),
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := repo.ApplyMetricEvent(aCtx, tenantA, "score-event-1", "assessment.score_recorded.v1", metrics); err != nil {
			t.Fatalf("apply metric event attempt %d: %v", attempt, err)
		}
	}
	rows, _, err := repo.ListMetrics(aCtx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two metric rows, got %+v", rows)
	}
	for _, row := range rows {
		if row.MetricName == "assessments.count" && row.Value != 1 {
			t.Fatalf("redelivery inflated count: %+v", row)
		}
		if row.Unit == domain.UnitAverage && (row.SampleCount == nil || *row.SampleCount != 1) {
			t.Fatalf("redelivery inflated average sample count: %+v", row)
		}
	}
	bRows, _, err := repo.ListMetrics(withTenant(context.Background(), tenantB), tenantB, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(bRows) != 0 {
		t.Fatalf("tenant B saw tenant A metrics: %+v", bRows)
	}
}

func TestRepository_ScoreLifecycleRecomputesCurrentState(t *testing.T) {
	repo := newRepo(t)
	ctx := withTenant(context.Background(), tenantA)
	recordedAt := time.Date(2025, 9, 1, 9, 0, 0, 0, time.UTC)
	base := domain.AssessmentScoreEvent{
		EventID: "score-record-1", EventType: "assessment.score_recorded.v1", Operation: domain.ScoreRecorded,
		ScoreID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", AssessmentID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		StudentID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", SubjectID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		AcademicYearID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", Score: 80, MaxScore: 100,
		RecordedAt: recordedAt, OccurredAt: recordedAt,
	}
	if err := repo.ApplyAssessmentScoreEvent(ctx, tenantA, base); err != nil {
		t.Fatal(err)
	}
	// Redelivery cannot add another fact or sample.
	if err := repo.ApplyAssessmentScoreEvent(ctx, tenantA, base); err != nil {
		t.Fatal(err)
	}
	second := base
	second.EventID = "score-record-2"
	second.ScoreID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	second.Score = 100
	if err := repo.ApplyAssessmentScoreEvent(ctx, tenantA, second); err != nil {
		t.Fatal(err)
	}
	assertScoreRollup(ctx, t, repo, 2, 180, 90, 90)

	updated := base
	updated.EventID = "score-update-1"
	updated.EventType = "assessment.score_updated.v1"
	updated.Operation = domain.ScoreUpdated
	updated.Score = 40
	updated.MaxScore = 80
	updated.OccurredAt = recordedAt.Add(time.Hour)
	if err := repo.ApplyAssessmentScoreEvent(ctx, tenantA, updated); err != nil {
		t.Fatal(err)
	}
	assertScoreRollup(ctx, t, repo, 2, 140, 70, 75)

	deleted := second
	deleted.EventID = "score-delete-2"
	deleted.EventType = "assessment.score_deleted.v1"
	deleted.Operation = domain.ScoreDeleted
	deleted.OccurredAt = recordedAt.Add(2 * time.Hour)
	if err := repo.ApplyAssessmentScoreEvent(ctx, tenantA, deleted); err != nil {
		t.Fatal(err)
	}
	assertScoreRollup(ctx, t, repo, 1, 40, 40, 50)

	other, _, err := repo.ListMetrics(withTenant(context.Background(), tenantB), tenantB, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("tenant B saw tenant A score rollup: %+v", other)
	}
}

func assertScoreRollup(ctx context.Context, t *testing.T, repo ports.Repository, count, sum, average, percentage float64) {
	t.Helper()
	rows, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, StudentIDs: []string{"cccccccc-cccc-4ccc-8ccc-cccccccccccc"}})
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]*domain.Metric, len(rows))
	for _, row := range rows {
		byName[row.MetricName] = row
	}
	for name, want := range map[string]float64{
		"assessments.count": count, "assessments.sum_score": sum,
		"assessments.avg_score": average, "assessments.avg_percentage": percentage,
	} {
		if byName[name] == nil || byName[name].Value != want {
			t.Fatalf("%s = %+v, want %v (all=%+v)", name, byName[name], want, byName)
		}
	}
	for _, name := range []string{"assessments.avg_score", "assessments.avg_percentage"} {
		if byName[name].SampleCount == nil || float64(*byName[name].SampleCount) != count {
			t.Fatalf("%s sample_count = %+v, want %v", name, byName[name].SampleCount, count)
		}
	}
}

func TestRepository_UpsertIncrementsCount(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	for i := 0; i < 3; i++ {
		m := mustMetric(t, "students.count", "2025-09-01", 1, domain.UnitCount, nil)
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
	repo := newRepo(t)

	for _, amount := range []float64{10, 20, 30} {
		m := mustMetric(t, "payments.total", "2025-09-01", amount, domain.UnitSum, nil)
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
	repo := newRepo(t)

	dims := domain.Dimensions{"subject_id": "sub-1", "academic_year_id": "ay-1"}
	for _, score := range []float64{80, 100} {
		m := mustMetric(t, "assessments.avg_score", "2025-09-01", score, domain.UnitAverage, dims)
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
	repo := newRepo(t)

	dims := domain.Dimensions{"subject_id": "sub-1", "academic_year_id": "ay-1"}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, "students.count", "2025-09-01", 1, domain.UnitCount, nil)); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, "students.count", "2025-09-02", 1, domain.UnitCount, nil)); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}
	if err := repo.UpsertMetric(ctx, tenantA, mustMetric(t, "assessments.avg_score", "2025-09-01", 80, domain.UnitAverage, dims)); err != nil {
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

func TestRepository_ListRestrictsMetricsToAuthorisedStudents(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	for _, studentID := range []string{"student-a", "student-b"} {
		metric := mustMetric(t, "assessments.count", "2025-09-01", 1, domain.UnitCount, domain.Dimensions{
			"student_id":       studentID,
			"subject_id":       "subject-a",
			"academic_year_id": "year-a",
		})
		if err := repo.UpsertMetric(ctx, tenantA, metric); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, StudentIDs: []string{"student-a"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Dimensions["student_id"] != "student-a" {
		t.Fatalf("unexpected scoped metrics: %+v", rows)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	for _, date := range []string{"2025-09-01", "2025-09-02"} {
		m := mustMetric(t, "students.count", date, 1, domain.UnitCount, nil)
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
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	if err := repo.UpsertMetric(aCtx, tenantA, mustMetric(t, "students.count", "2025-09-01", 1, domain.UnitCount, nil)); err != nil {
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
