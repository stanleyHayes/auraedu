package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/auraedu/analytics-service/internal/application"
	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
)

// memoryRepository is an in-memory stub used to test the projection use case
// without spinning up Postgres or NATS.
type memoryRepository struct {
	mu      sync.Mutex
	metrics []*domain.Metric
}

var _ ports.Repository = (*memoryRepository)(nil)

func (r *memoryRepository) UpsertMetric(_ context.Context, tenantID string, m *domain.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m.TenantID = tenantID
	for _, existing := range r.metrics {
		if existing.TenantID == tenantID &&
			existing.MetricName == m.MetricName &&
			existing.BucketDate.String() == m.BucketDate.String() &&
			existing.Dimensions.Key() == m.Dimensions.Key() {
			switch m.Unit {
			case domain.UnitCount, domain.UnitSum:
				existing.Value += m.Value
			case domain.UnitAverage:
				if err := existing.AddSample(m.Value); err != nil {
					return err
				}
			case domain.UnitPercentage:
				existing.Value += m.Value
			}
			existing.UpdatedAt = m.UpdatedAt
			return nil
		}
	}
	r.metrics = append(r.metrics, m)
	return nil
}

func (r *memoryRepository) ListMetrics(_ context.Context, tenantID string, filter ports.ListFilter) ([]*domain.Metric, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []*domain.Metric
	for _, m := range r.metrics {
		if m.TenantID != tenantID {
			continue
		}
		if filter.MetricName != "" && m.MetricName != filter.MetricName {
			continue
		}
		if filter.BucketDateFrom != "" && m.BucketDate.String() < filter.BucketDateFrom {
			continue
		}
		if filter.BucketDateTo != "" && m.BucketDate.String() > filter.BucketDateTo {
			continue
		}
		if filter.DimensionKey != "" && filter.DimensionValue != "" {
			if m.Dimensions[filter.DimensionKey] != filter.DimensionValue {
				continue
			}
		}
		out = append(out, m)
	}
	return out, "", nil
}

func newMemoryRepo() *memoryRepository { return &memoryRepository{} }

func mustEvent(t *testing.T, eventType string, data any) tenancy.CloudEvent {
	t.Helper()
	event, err := tenancy.NewCloudEvent(eventType, "test", "evt-1", tenantA, data)
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	event.Time = "2025-09-01T10:00:00Z"
	return event
}

func TestProjection_StudentEnrolled(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	event := mustEvent(t, "student.enrolled.v1", map[string]any{"student_id": "s1"})
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].MetricName != "students.count" || metrics[0].Value != 1 {
		t.Fatalf("unexpected metric: %+v", metrics[0])
	}
}

func TestProjection_AttendanceMarked(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	for _, status := range []string{"present", "absent", "late", "excused"} {
		data := map[string]any{"student_id": "s1", "status": status}
		event := mustEvent(t, "attendance.marked.v1", data)
		if err := proj.ProcessEvent(ctx, event); err != nil {
			t.Fatalf("process %s event: %v", status, err)
		}
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %d", len(metrics))
	}
	for _, m := range metrics {
		if m.Value != 1 {
			t.Fatalf("expected value 1 for %s, got %v", m.MetricName, m.Value)
		}
	}
}

func TestProjection_AssessmentScore(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	data := map[string]any{
		"score":            80,
		"subject_id":       "sub-1",
		"academic_year_id": "ay-1",
	}
	event := mustEvent(t, "assessment.score_recorded.v1", data)
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	data["score"] = 100
	event = mustEvent(t, "assessment.score_recorded.v1", data)
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}

	byName := make(map[string]*domain.Metric)
	for _, m := range metrics {
		byName[m.MetricName] = m
	}
	if byName["assessments.count"].Value != 2 {
		t.Fatalf("expected count 2, got %v", byName["assessments.count"].Value)
	}
	if byName["assessments.sum_score"].Value != 180 {
		t.Fatalf("expected sum 180, got %v", byName["assessments.sum_score"].Value)
	}
	if byName["assessments.avg_score"].Value != 90 {
		t.Fatalf("expected avg 90, got %v", byName["assessments.avg_score"].Value)
	}
}

func TestProjection_PaymentReceived(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	data := map[string]any{"amount": 50, "currency": "GHS"}
	event := mustEvent(t, "payment.received.v1", data)
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, MetricName: "payments.total"})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].Value != 50 {
		t.Fatalf("unexpected metric: %+v", metrics)
	}
}

func TestProjection_UnknownEventIgnored(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	event := mustEvent(t, "some.unknown.v1", map[string]any{})
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process unknown event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(metrics))
	}
}

func TestProjection_RawJSONPayload(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	raw, err := json.Marshal(map[string]any{"amount": 125})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "invoice.created.v1",
		Source:      "test",
		ID:          "evt-2",
		TenantID:    tenantA,
		Time:        "2025-09-02T00:00:00Z",
		Data:        raw,
	}
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, MetricName: "invoices.total"})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].Value != 125 {
		t.Fatalf("unexpected metric: %+v", metrics)
	}
	if metrics[0].BucketDate.String() != "2025-09-02" {
		t.Fatalf("expected bucket_date 2025-09-02, got %q", metrics[0].BucketDate.String())
	}
}
