package application

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestDeliveryMetricsUseBoundedLabels(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown metric provider: %v", err)
		}
		otel.SetMeterProvider(previous)
	})

	metrics := newDeliveryMetrics()
	metrics.observe(context.Background(), "recipient@example.com", "failed", time.Now().Add(-time.Millisecond))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scope := range collected.ScopeMetrics {
		for _, exported := range scope.Metrics {
			if exported.Name != "auraedu.notification.deliveries" {
				continue
			}
			sum, ok := exported.Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) != 1 {
				t.Fatalf("unexpected delivery counter data: %#v", exported.Data)
			}
			channel, ok := sum.DataPoints[0].Attributes.Value("channel")
			if !ok || channel.AsString() != "unknown" {
				t.Fatalf("channel label must be bounded, got %v", channel)
			}
			outcome, ok := sum.DataPoints[0].Attributes.Value("outcome")
			if !ok || outcome.AsString() != "failed" {
				t.Fatalf("outcome label=%v", outcome)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("delivery counter was not exported")
	}
}
