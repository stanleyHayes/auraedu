package observ

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func InitTracing(serviceName, serviceVersion string) (func(context.Context) error, error) {
	rp := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		attribute.String("deployment.environment.name", strings.TrimSpace(os.Getenv("ENVIRONMENT"))),
	)

	metricShutdown, err := initMetrics(rp)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	exporterName := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER")))
	if endpoint == "" && exporterName != "console" {
		tp := sdktrace.NewTracerProvider(sdktrace.WithResource(rp), sdktrace.WithSampler(sdktrace.NeverSample()))
		otel.SetTracerProvider(tp)
		return combinedShutdown(tp.Shutdown, metricShutdown), nil
	}

	var exp sdktrace.SpanExporter
	if exporterName == "console" {
		exp, err = stdouttrace.New()
	} else {
		exp, err = otlptracegrpc.New(context.Background())
	}
	if err != nil {
		shutdownErr := metricShutdown(context.Background())
		return nil, errors.Join(fmt.Errorf("observ: create trace exporter: %w", err), shutdownErr)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(rp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(traceSampleRatio()))),
	)
	otel.SetTracerProvider(tp)
	return combinedShutdown(tp.Shutdown, metricShutdown), nil
}

func initMetrics(rp *resource.Resource) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if endpoint == "" || strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_METRICS_EXPORTER")), "none") {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlpmetricgrpc.New(context.Background())
	if err != nil {
		return nil, fmt.Errorf("observ: create metrics exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(rp),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))),
	)
	otel.SetMeterProvider(provider)
	return provider.Shutdown, nil
}

func combinedShutdown(shutdowns ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		for i := len(shutdowns) - 1; i >= 0; i-- {
			if shutdowns[i] != nil {
				errs = append(errs, shutdowns[i](ctx))
			}
		}
		return errors.Join(errs...)
	}
}

func traceSampleRatio() float64 {
	if value := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed >= 0 && parsed <= 1 {
			return parsed
		}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") {
		return 0.1
	}
	return 1
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
