package observ

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func InitTracing(serviceName, serviceVersion string) (func(context.Context) error, error) {
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("observ: create trace exporter: %w", err)
	}

	rp := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(rp),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func StringAttr(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}
