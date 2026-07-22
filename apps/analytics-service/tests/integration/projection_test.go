package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
	mu              sync.Mutex
	metrics         []*domain.Metric
	processed       map[string]bool
	leadSources     map[string]string
	applicationLead map[string]string
	scoreFacts      map[string]domain.AssessmentScoreEvent
}

var _ ports.Repository = (*memoryRepository)(nil)

func (r *memoryRepository) UpsertMetric(_ context.Context, tenantID string, m *domain.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.upsertMetricLocked(tenantID, m)
}

func (r *memoryRepository) upsertMetricLocked(tenantID string, m *domain.Metric) error {
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

func (r *memoryRepository) ApplyMetricEvent(_ context.Context, tenantID, eventID, _ string, metrics []*domain.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.processed == nil {
		r.processed = map[string]bool{}
	}
	key := tenantID + ":" + eventID
	if r.processed[key] {
		return nil
	}
	r.processed[key] = true
	for _, metric := range metrics {
		if err := r.upsertMetricLocked(tenantID, metric); err != nil {
			delete(r.processed, key)
			return err
		}
	}
	return nil
}

func (r *memoryRepository) ApplyAssessmentScoreEvent(_ context.Context, tenantID string, event domain.AssessmentScoreEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scoreFacts == nil {
		r.scoreFacts = map[string]domain.AssessmentScoreEvent{}
	}
	key := tenantID + ":" + event.EventID
	if r.processed[key] {
		return nil
	}
	r.processed[key] = true
	factKey := tenantID + ":" + event.ScoreID
	if event.Operation == domain.ScoreDeleted {
		delete(r.scoreFacts, factKey)
	} else {
		r.scoreFacts[factKey] = event
	}

	kept := r.metrics[:0]
	for _, metric := range r.metrics {
		if metric.TenantID != tenantID || !strings.HasPrefix(metric.MetricName, "assessments.") {
			kept = append(kept, metric)
		}
	}
	r.metrics = kept
	type aggregate struct {
		event              domain.AssessmentScoreEvent
		count              int64
		sum, percentageSum float64
	}
	rollups := map[string]*aggregate{}
	for factKey, fact := range r.scoreFacts {
		if !strings.HasPrefix(factKey, tenantID+":") {
			continue
		}
		groupKey := fact.BucketDate() + "|" + fact.Dimensions().Key()
		item := rollups[groupKey]
		if item == nil {
			item = &aggregate{event: fact}
			rollups[groupKey] = item
		}
		item.count++
		item.sum += fact.Score
		item.percentageSum += fact.Score / fact.MaxScore * 100
	}
	for _, item := range rollups {
		for _, spec := range []struct {
			name  string
			value float64
			unit  domain.Unit
		}{
			{name: "assessments.count", value: float64(item.count), unit: domain.UnitCount},
			{name: "assessments.sum_score", value: item.sum, unit: domain.UnitSum},
			{name: "assessments.avg_score", value: item.sum / float64(item.count), unit: domain.UnitAverage},
			{name: "assessments.avg_percentage", value: item.percentageSum / float64(item.count), unit: domain.UnitAverage},
		} {
			metric, err := domain.NewMetric(tenantID, spec.name, item.event.BucketDate(), spec.value, spec.unit, item.event.Dimensions())
			if err != nil {
				return err
			}
			if spec.unit == domain.UnitAverage {
				metric.SampleCount = &item.count
			}
			r.metrics = append(r.metrics, metric)
		}
	}
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

func (r *memoryRepository) ApplyGrowthEvent(ctx context.Context, tenantID string, event domain.GrowthEvent) error {
	r.mu.Lock()
	if r.processed == nil {
		r.processed = map[string]bool{}
		r.leadSources = map[string]string{}
		r.applicationLead = map[string]string{}
	}
	key := tenantID + ":" + event.EventID
	if r.processed[key] {
		r.mu.Unlock()
		return nil
	}
	r.processed[key] = true
	if event.LeadID != "" && event.Source != "" {
		r.leadSources[tenantID+":"+event.LeadID] = event.Source
	}
	if event.ApplicationID != "" && event.LeadID != "" {
		r.applicationLead[tenantID+":"+event.ApplicationID] = event.LeadID
	}
	if event.LeadID == "" && event.ApplicationID != "" {
		event.LeadID = r.applicationLead[tenantID+":"+event.ApplicationID]
	}
	if event.Source == "" && event.LeadID != "" {
		event.Source = r.leadSources[tenantID+":"+event.LeadID]
	}
	r.mu.Unlock()
	dims := domain.Dimensions{}
	for k, v := range map[string]string{"source": event.Source, "campaign_id": event.CampaignID, "programme_id": event.ProgrammeID, "intake_id": event.IntakeID} {
		if v != "" {
			dims[k] = v
		}
	}
	m, err := domain.NewMetric(tenantID, "growth.funnel."+event.Stage, event.BucketDate, 1, domain.UnitCount, dims)
	if err != nil {
		return err
	}
	if err := r.UpsertMetric(ctx, tenantID, m); err != nil {
		return err
	}
	if event.Stage == domain.GrowthLeads {
		legacy, err := domain.NewMetric(tenantID, "growth.leads.count", event.BucketDate, 1, domain.UnitCount, nil)
		if err != nil {
			return err
		}
		return r.UpsertMetric(ctx, tenantID, legacy)
	}
	return nil
}

func (r *memoryRepository) GrowthRollups(_ context.Context, tenantID, fromDate, toDate string) ([]domain.GrowthRollup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	byKey := map[string]domain.GrowthRollup{}
	for _, m := range r.metrics {
		if m.TenantID != tenantID || !strings.HasPrefix(m.MetricName, "growth.funnel.") || m.BucketDate.String() < fromDate || m.BucketDate.String() > toDate {
			continue
		}
		row := domain.GrowthRollup{Stage: strings.TrimPrefix(m.MetricName, "growth.funnel."), Source: m.Dimensions["source"], CampaignID: m.Dimensions["campaign_id"], ProgrammeID: m.Dimensions["programme_id"], IntakeID: m.Dimensions["intake_id"], Value: m.Value}
		key := row.Stage + "|" + row.Source + "|" + row.CampaignID + "|" + row.ProgrammeID + "|" + row.IntakeID
		current := byKey[key]
		current.Stage = row.Stage
		current.Source = row.Source
		current.CampaignID = row.CampaignID
		current.ProgrammeID = row.ProgrammeID
		current.IntakeID = row.IntakeID
		current.Value += row.Value
		byKey[key] = current
	}
	out := make([]domain.GrowthRollup, 0, len(byKey))
	for _, row := range byKey {
		out = append(out, row)
	}
	return out, nil
}

func newMemoryRepo() *memoryRepository {
	return &memoryRepository{processed: map[string]bool{}, leadSources: map[string]string{}, applicationLead: map[string]string{}, scoreFacts: map[string]domain.AssessmentScoreEvent{}}
}

func mustEvent(t *testing.T, eventType string, data any) tenancy.CloudEvent {
	t.Helper()
	event, err := tenancy.NewCloudEvent(eventType, "test", "evt-1", tenantA, data)
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	event.Time = "2025-09-01T10:00:00Z"
	event.ID = eventType + "-evt"
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

func TestProjection_LeadCreated(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	for _, eventType := range []string{"lead.created.v1"} {
		event := mustEvent(t, eventType, map[string]any{"lead_id": "lead-1", "source": "website"})
		if err := proj.ProcessEvent(ctx, event); err != nil {
			t.Fatalf("process %s: %v", eventType, err)
		}
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10, MetricName: "growth.leads.count"})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].Value != 1 {
		t.Fatalf("expected one lead-count metric with value 1, got %+v", metrics)
	}
}

func TestProjection_GrowthFunnelAttributionAndIdempotency(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.Default())
	ctx := context.Background()
	leadID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	applicationID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	programmeID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	intakeID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	lead := mustEvent(t, "lead.created.v1", map[string]any{"lead_id": leadID, "source": "referral", "created_at": "2025-09-01T10:00:00Z"})
	if err := proj.ProcessEvent(ctx, lead); err != nil {
		t.Fatal(err)
	}
	// JetStream redelivery of the same event ID must not inflate the funnel.
	if err := proj.ProcessEvent(ctx, lead); err != nil {
		t.Fatal(err)
	}
	for index, eventType := range []string{"application.started.v1", "application.submitted.v1", "application.admitted.v1", "offer.issued.v1", "offer.accepted.v1"} {
		event := mustEvent(t, eventType, map[string]any{"application_id": applicationID, "lead_id": leadID, "programme_id": programmeID, "intake_id": intakeID, "started_at": "2025-09-01T10:00:00Z"})
		event.ID = fmt.Sprintf("growth-%d", index)
		if err := proj.ProcessEvent(ctx, event); err != nil {
			t.Fatalf("process %s: %v", eventType, err)
		}
	}
	rows, err := repo.GrowthRollups(ctx, tenantA, "2025-09-01", "2025-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("expected six funnel stages, got %+v", rows)
	}
	for _, row := range rows {
		if row.Value != 1 {
			t.Fatalf("expected deduplicated value 1, got %+v", row)
		}
		if row.Stage != domain.GrowthLeads && (row.Source != "referral" || row.ProgrammeID != programmeID) {
			t.Fatalf("expected correlated attribution, got %+v", row)
		}
	}
}

func TestProjection_AttendanceMarked(t *testing.T) {
	repo := newMemoryRepo()
	proj := application.NewProjection(repo, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	for _, status := range []string{"present", "absent", "late", "excused"} {
		data := map[string]any{"student_id": "s1", "status": status}
		event := mustEvent(t, "attendance.marked.v1", data)
		event.ID = "attendance-" + status
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
		"score_id":         "score-1",
		"assessment_id":    "assessment-1",
		"student_id":       "student-1",
		"score":            80,
		"max_score":        100,
		"subject_id":       "sub-1",
		"academic_year_id": "ay-1",
		"recorded_at":      "2025-09-01T09:00:00Z",
	}
	event := mustEvent(t, "assessment.score_recorded.v1", data)
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	data["score"] = 100
	data["score_id"] = "score-2"
	event = mustEvent(t, "assessment.score_recorded.v1", data)
	event.ID = "assessment-score-2"
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("process event: %v", err)
	}

	metrics, _, err := repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %d", len(metrics))
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
	if byName["assessments.avg_percentage"].Value != 90 {
		t.Fatalf("expected average percentage 90, got %v", byName["assessments.avg_percentage"].Value)
	}

	// JetStream may redeliver the same CloudEvent. The persisted event id must
	// make the complete four-metric projection exactly-once.
	if err := proj.ProcessEvent(ctx, event); err != nil {
		t.Fatalf("redeliver event: %v", err)
	}
	metrics, _, err = repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list after redelivery: %v", err)
	}
	for _, metric := range metrics {
		if metric.MetricName == "assessments.count" && metric.Value != 2 {
			t.Fatalf("redelivery inflated assessment count: %+v", metric)
		}
		if metric.Unit == domain.UnitAverage && (metric.SampleCount == nil || *metric.SampleCount != 2) {
			t.Fatalf("redelivery inflated average sample count: %+v", metric)
		}
	}

	data["score_id"] = "score-1"
	data["score"] = 40
	data["max_score"] = 80
	updated := mustEvent(t, "assessment.score_updated.v1", data)
	updated.ID = "assessment-score-update-1"
	if err := proj.ProcessEvent(ctx, updated); err != nil {
		t.Fatalf("update score projection: %v", err)
	}
	metrics, _, err = repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	byName = make(map[string]*domain.Metric)
	for _, metric := range metrics {
		byName[metric.MetricName] = metric
	}
	if byName["assessments.count"].Value != 2 || byName["assessments.sum_score"].Value != 140 || byName["assessments.avg_score"].Value != 70 || byName["assessments.avg_percentage"].Value != 75 {
		t.Fatalf("score update did not replace aggregates: %+v", byName)
	}

	data["score_id"] = "score-2"
	data["score"] = 100
	data["max_score"] = 100
	deleted := mustEvent(t, "assessment.score_deleted.v1", data)
	deleted.ID = "assessment-score-delete-2"
	if err := proj.ProcessEvent(ctx, deleted); err != nil {
		t.Fatalf("delete score projection: %v", err)
	}
	metrics, _, err = repo.ListMetrics(ctx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	byName = make(map[string]*domain.Metric)
	for _, metric := range metrics {
		byName[metric.MetricName] = metric
	}
	if byName["assessments.count"].Value != 1 || byName["assessments.sum_score"].Value != 40 || byName["assessments.avg_score"].Value != 40 || byName["assessments.avg_percentage"].Value != 50 {
		t.Fatalf("score deletion did not remove aggregates: %+v", byName)
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
