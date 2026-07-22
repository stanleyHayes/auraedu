package observ

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// WorkerMetrics records bounded job outcomes for background processes. Jobs
// must be declared at construction; runtime values outside the allowlist
// collapse to "unknown" so tenant IDs, event IDs and payload values cannot
// become metric labels.
type WorkerMetrics struct {
	service  string
	allowed  map[string]struct{}
	jobs     metric.Int64Counter
	duration metric.Float64Histogram
}

func NewWorkerMetrics(service string, jobs ...string) *WorkerMetrics {
	allowed := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if normalized := strings.TrimSpace(job); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	meter := otel.Meter("github.com/auraedu/platform/observ/workers")
	jobCounter, err := meter.Int64Counter(
		"auraedu.worker.jobs",
		metric.WithDescription("Background job attempts by service, bounded job name and outcome."),
	)
	if err != nil {
		panic(fmt.Errorf("observ: create worker job counter: %w", err))
	}
	jobDuration, err := meter.Float64Histogram(
		"auraedu.worker.job.duration",
		metric.WithDescription("Background job duration in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Errorf("observ: create worker duration histogram: %w", err))
	}
	return &WorkerMetrics{
		service: strings.TrimSpace(service), allowed: allowed,
		jobs: jobCounter, duration: jobDuration,
	}
}

// Observe records success when err is nil and failure otherwise.
func (m *WorkerMetrics) Observe(ctx context.Context, job string, started time.Time, err error) {
	if m == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "failed"
	}
	job = strings.TrimSpace(job)
	if _, ok := m.allowed[job]; !ok {
		job = "unknown"
	}
	service := m.service
	if service == "" {
		service = "unknown-worker"
	}
	attrs := metric.WithAttributes(
		attribute.String("service", service),
		attribute.String("job", job),
		attribute.String("outcome", outcome),
	)
	m.jobs.Add(ctx, 1, attrs)
	m.duration.Record(ctx, time.Since(started).Seconds(), attrs)
}
