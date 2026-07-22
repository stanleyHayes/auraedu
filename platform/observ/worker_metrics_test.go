package observ

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestWorkerMetricsCollapseUndeclaredJobLabels(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
		otel.SetMeterProvider(previous)
	})

	worker := NewWorkerMetrics("notification-service-worker", "scheduled-delivery")
	worker.Observe(context.Background(), "tenant-or-event-id", time.Now().Add(-time.Millisecond), errors.New("failed"))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, exported := range scope.Metrics {
			if exported.Name != "auraedu.worker.jobs" {
				continue
			}
			sum, ok := exported.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("job metric data type=%T", exported.Data)
			}
			point := sum.DataPoints[0]
			if job, _ := point.Attributes.Value(attribute.Key("job")); job.AsString() != "unknown" {
				t.Fatalf("undeclared job leaked into labels: %q", job.AsString())
			}
			if outcome, _ := point.Attributes.Value(attribute.Key("outcome")); outcome.AsString() != "failed" {
				t.Fatalf("outcome=%q", outcome.AsString())
			}
			return
		}
	}
	t.Fatal("worker job counter was not exported")
}
