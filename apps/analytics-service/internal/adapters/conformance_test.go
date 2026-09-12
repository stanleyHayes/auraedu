// Package adapters_test runs one contract against every analytics repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them.
package adapters_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/analytics-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/analytics-service/internal/adapters/postgres"
	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

// tenantCtx carries the tenant the way the application layer does: the Postgres
// adapter derives app.tenant_id from it for RLS, and the Mongo adapter's Scope
// reads the same context. Both drivers see one calling convention.
func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func runAnalyticsContract(t *testing.T, repo ports.Repository) {
	t.Run("upsert and retrieve a metric within the tenant", func(t *testing.T) {
		tenantID := "upshs"
		ctx := tenantCtx(tenantID)

		metric, err := domain.NewMetric(tenantID, "test.metric", "2025-09-12", 42.5, domain.UnitCount, nil)
		if err != nil {
			t.Fatalf("new metric: %v", err)
		}

		if err := repo.UpsertMetric(ctx, tenantID, metric); err != nil {
			t.Fatalf("upsert metric: %v", err)
		}

		// List and verify
		filter := ports.ListFilter{
			Limit:      10,
			MetricName: "test.metric",
		}
		metrics, _, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list metrics: %v", err)
		}
		if len(metrics) != 1 || metrics[0].Value != 42.5 {
			t.Fatalf("metric not found or incorrect: %+v", metrics)
		}
	})

	t.Run("another tenant cannot read the metric", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		metric, err := domain.NewMetric("upshs", "isolated.metric", "2025-09-12", 10, domain.UnitCount, nil)
		if err != nil {
			t.Fatalf("new metric: %v", err)
		}
		if err := repo.UpsertMetric(ctx, "upshs", metric); err != nil {
			t.Fatalf("upsert metric: %v", err)
		}

		// Read as the other tenant, in that tenant's own context. Asking for
		// "aboom" while still carrying an "upshs" context would pass for the
		// wrong reason: RLS would refuse the mismatch rather than the scope
		// proving anything.
		filter := ports.ListFilter{
			Limit:      100,
			MetricName: "isolated.metric",
		}
		metrics, _, err := repo.ListMetrics(tenantCtx("aboom"), "aboom", filter)
		if err != nil {
			t.Fatalf("list metrics: %v", err)
		}
		if len(metrics) != 0 {
			t.Fatalf("metric leaked to another tenant: %+v", metrics)
		}
	})

	t.Run("apply metric event deduplicates by event id", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		tenantID := "upshs"
		eventID := uuid.NewString()

		metric1, err := domain.NewMetric(tenantID, "event.metric", "2025-09-12", 5, domain.UnitCount, nil)
		if err != nil {
			t.Fatalf("new metric 1: %v", err)
		}

		// Apply first time
		if err := repo.ApplyMetricEvent(ctx, tenantID, eventID, "test.event.v1", []*domain.Metric{metric1}); err != nil {
			t.Fatalf("apply metric event 1: %v", err)
		}

		// Apply again with different value
		metric2, err := domain.NewMetric(tenantID, "event.metric", "2025-09-12", 10, domain.UnitCount, nil)
		if err != nil {
			t.Fatalf("new metric 2: %v", err)
		}
		if err := repo.ApplyMetricEvent(ctx, tenantID, eventID, "test.event.v1", []*domain.Metric{metric2}); err != nil {
			t.Fatalf("apply metric event 2: %v", err)
		}

		// Verify the first value is unchanged (deduplication worked)
		filter := ports.ListFilter{
			Limit:      100,
			MetricName: "event.metric",
		}
		metrics, _, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list metrics: %v", err)
		}
		if len(metrics) != 1 || metrics[0].Value != 5 {
			t.Fatalf("deduplication failed or value incorrect: %+v", metrics)
		}
	})

	t.Run("apply assessment score event and recompute aggregates", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		tenantID := "upshs"
		studentID := uuid.NewString()
		subjectID := uuid.NewString()
		academicYearID := uuid.NewString()
		assessmentID := uuid.NewString()

		scoreID := uuid.NewString()
		recordedAt := time.Now().UTC()

		event := domain.AssessmentScoreEvent{
			EventID:        uuid.NewString(),
			EventType:      "assessment.score.recorded.v1",
			Operation:      domain.ScoreRecorded,
			ScoreID:        scoreID,
			AssessmentID:   assessmentID,
			StudentID:      studentID,
			SubjectID:      subjectID,
			AcademicYearID: academicYearID,
			Score:          75,
			MaxScore:       100,
			RecordedAt:     recordedAt,
			OccurredAt:     recordedAt,
		}

		if err := repo.ApplyAssessmentScoreEvent(ctx, tenantID, event); err != nil {
			t.Fatalf("apply assessment score event: %v", err)
		}

		// Verify metrics were created
		filter := ports.ListFilter{
			Limit:      100,
			MetricName: "assessments.count",
		}
		metrics, _, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list metrics: %v", err)
		}

		// Should have count, sum, avg_score, avg_percentage
		if len(metrics) < 1 {
			t.Fatalf("no assessment metrics created")
		}

		foundCount := false
		for _, m := range metrics {
			if m.MetricName == "assessments.count" && m.Value == 1 {
				foundCount = true
				break
			}
		}
		if !foundCount {
			t.Fatalf("assessment count metric not found or incorrect: %+v", metrics)
		}
	})

	t.Run("list metrics with pagination cursor", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		tenantID := "upshs"

		// Create multiple metrics
		for i := 0; i < 5; i++ {
			metric, err := domain.NewMetric(
				tenantID,
				"paginated.metric",
				"2025-09-12",
				float64(i),
				domain.UnitCount,
				// Metrics are keyed by name, date and dimensions, so the dimension
				// has to actually vary or every upsert collapses onto one record
				// and there is nothing to paginate.
				domain.Dimensions{"index": strconv.Itoa(i)},
			)
			if err != nil {
				t.Fatalf("new metric: %v", err)
			}
			if err := repo.UpsertMetric(ctx, tenantID, metric); err != nil {
				t.Fatalf("upsert metric: %v", err)
			}
		}

		// Paginate through results
		filter := ports.ListFilter{
			Limit:      2,
			MetricName: "paginated.metric",
		}
		page1, cursor1, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list page 1: %v", err)
		}
		if len(page1) != 2 || cursor1 == "" {
			t.Fatalf("page 1 unexpected: got %d items, cursor=%q", len(page1), cursor1)
		}

		// Get next page
		filter.Cursor = cursor1
		page2, _, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list page 2: %v", err)
		}
		if len(page2) == 0 {
			t.Fatalf("page 2 is empty")
		}

		// Verify no overlap
		page1IDs := make(map[string]bool)
		for _, m := range page1 {
			page1IDs[m.ID] = true
		}
		for _, m := range page2 {
			if page1IDs[m.ID] {
				t.Fatalf("cursor pagination repeated record %s", m.ID)
			}
		}
	})

	t.Run("apply growth event and verify funnel metrics", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		tenantID := "upshs"

		event := domain.GrowthEvent{
			EventID:    uuid.NewString(),
			EventType:  "leads.generated.v1",
			Stage:      domain.GrowthLeads,
			BucketDate: "2025-09-12",
			LeadID:     uuid.NewString(),
			Source:     "organic",
			CampaignID: "",
			OccurredAt: time.Now().UTC(),
		}

		if err := repo.ApplyGrowthEvent(ctx, tenantID, event); err != nil {
			t.Fatalf("apply growth event: %v", err)
		}

		// Verify funnel metric and legacy metric
		filter := ports.ListFilter{
			Limit: 100,
		}
		metrics, _, err := repo.ListMetrics(ctx, tenantID, filter)
		if err != nil {
			t.Fatalf("list metrics: %v", err)
		}

		foundFunnel := false
		foundLegacy := false
		for _, m := range metrics {
			if m.MetricName == "growth.funnel.leads" {
				foundFunnel = true
			}
			if m.MetricName == "growth.leads.count" {
				foundLegacy = true
			}
		}
		if !foundFunnel || !foundLegacy {
			t.Fatalf("growth metrics not found: funnel=%v, legacy=%v", foundFunnel, foundLegacy)
		}
	})

	t.Run("growth rollups aggregates facts correctly", func(t *testing.T) {
		ctx := tenantCtx("upshs")
		tenantID := "upshs"
		date := "2025-09-12"

		// Create two growth events
		event1 := domain.GrowthEvent{
			EventID:    uuid.NewString(),
			EventType:  "leads.generated.v1",
			Stage:      domain.GrowthLeads,
			BucketDate: date,
			LeadID:     uuid.NewString(),
			Source:     "organic",
			OccurredAt: time.Now().UTC(),
		}
		event2 := domain.GrowthEvent{
			EventID:    uuid.NewString(),
			EventType:  "leads.generated.v1",
			Stage:      domain.GrowthLeads,
			BucketDate: date,
			LeadID:     uuid.NewString(),
			Source:     "paid",
			OccurredAt: time.Now().UTC(),
		}

		if err := repo.ApplyGrowthEvent(ctx, tenantID, event1); err != nil {
			t.Fatalf("apply event 1: %v", err)
		}
		if err := repo.ApplyGrowthEvent(ctx, tenantID, event2); err != nil {
			t.Fatalf("apply event 2: %v", err)
		}

		// Get rollups
		rollups, err := repo.GrowthRollups(ctx, tenantID, date, date)
		if err != nil {
			t.Fatalf("growth rollups: %v", err)
		}

		if len(rollups) < 2 {
			t.Fatalf("expected at least 2 rollups, got %d: %+v", len(rollups), rollups)
		}

		// Verify aggregation
		totalValue := 0.0
		for _, r := range rollups {
			if r.Stage == domain.GrowthLeads {
				totalValue += r.Value
			}
		}
		if totalValue < 2 {
			t.Fatalf("expected at least 2 total leads, got %v", totalValue)
		}
	})
}

func TestPostgresAnalyticsRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runAnalyticsContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoAnalyticsRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runAnalyticsContract(t, mongoadapter.NewRepository(mg.Store))
}
