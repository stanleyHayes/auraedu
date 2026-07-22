package observ

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInitTracingIsNoopWithoutConfiguredExporter(t *testing.T) {
	previous := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "")
	shutdown, err := InitTracing("test-service", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, span := Tracer("test").Start(context.Background(), "not-exported")
	if span.IsRecording() {
		t.Fatal("trace recorded without an explicitly configured exporter")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTraceSampleRatio(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
	if got := traceSampleRatio(); got != 0.1 {
		t.Fatalf("production ratio=%v", got)
	}
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
	if got := traceSampleRatio(); got != 0.25 {
		t.Fatalf("configured ratio=%v", got)
	}
}
