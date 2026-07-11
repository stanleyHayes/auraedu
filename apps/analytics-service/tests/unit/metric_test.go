package unit

import (
	"testing"

	"github.com/auraedu/analytics-service/internal/domain"
)

func TestNewMetric_RequiresTenant(t *testing.T) {
	if _, err := domain.NewMetric("", "students.count", "2025-09-01", 1, domain.UnitCount, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewMetric_RequiresMetricName(t *testing.T) {
	if _, err := domain.NewMetric("tenant-1", "", "2025-09-01", 1, domain.UnitCount, nil); err == nil {
		t.Fatal("expected error when metric_name is empty")
	}
}

func TestNewMetric_RequiresBucketDate(t *testing.T) {
	if _, err := domain.NewMetric("tenant-1", "students.count", "", 1, domain.UnitCount, nil); err == nil {
		t.Fatal("expected error when bucket_date is empty")
	}
	if _, err := domain.NewMetric("tenant-1", "students.count", "not-a-date", 1, domain.UnitCount, nil); err == nil {
		t.Fatal("expected error when bucket_date is invalid")
	}
}

func TestNewMetric_RequiresValidUnit(t *testing.T) {
	if _, err := domain.NewMetric("tenant-1", "students.count", "2025-09-01", 1, domain.Unit("invalid"), nil); err == nil {
		t.Fatal("expected error when unit is invalid")
	}
}

func TestNewMetric_CountCannotBeNegative(t *testing.T) {
	if _, err := domain.NewMetric("tenant-1", "students.count", "2025-09-01", -1, domain.UnitCount, nil); err == nil {
		t.Fatal("expected error for negative count value")
	}
}

func TestNewMetric_Valid(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "students.count", "2025-09-01", 1, domain.UnitCount, domain.Dimensions{"subject_id": "sub-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", m.TenantID)
	}
	if m.MetricName != "students.count" {
		t.Fatalf("metric_name not set: got %q", m.MetricName)
	}
	if m.BucketDate.String() != "2025-09-01" {
		t.Fatalf("bucket_date not set: got %q", m.BucketDate.String())
	}
	if m.Value != 1 {
		t.Fatalf("value not set: got %v", m.Value)
	}
	if m.Unit != domain.UnitCount {
		t.Fatalf("unit not set: got %q", m.Unit)
	}
	if len(m.Dimensions) != 1 || m.Dimensions["subject_id"] != "sub-1" {
		t.Fatalf("dimensions not set: got %v", m.Dimensions)
	}
}

func TestMetric_Increment(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "students.count", "2025-09-01", 1, domain.UnitCount, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Increment(2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Value != 3 {
		t.Fatalf("expected value 3, got %v", m.Value)
	}
}

func TestMetric_Increment_WrongUnit(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "payments.total", "2025-09-01", 10, domain.UnitSum, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Increment(1); err == nil {
		t.Fatal("expected error when incrementing a sum metric")
	}
}

func TestMetric_Add(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "payments.total", "2025-09-01", 10, domain.UnitSum, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Add(5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Value != 15 {
		t.Fatalf("expected value 15, got %v", m.Value)
	}
}

func TestMetric_AddSample(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "assessments.avg_score", "2025-09-01", 80, domain.UnitAverage, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.SampleCount == nil || *m.SampleCount != 1 {
		t.Fatalf("expected initial sample_count 1, got %v", m.SampleCount)
	}
	if err := m.AddSample(100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Value != 90 {
		t.Fatalf("expected average 90, got %v", m.Value)
	}
	if m.SampleCount == nil || *m.SampleCount != 2 {
		t.Fatalf("expected sample_count 2, got %v", m.SampleCount)
	}
}

func TestMetric_ValueString(t *testing.T) {
	m, err := domain.NewMetric("tenant-1", "payments.total", "2025-09-01", 12.5, domain.UnitSum, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ValueString() != "12.5" {
		t.Fatalf("expected value string 12.5, got %q", m.ValueString())
	}
}
