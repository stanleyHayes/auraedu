package application

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type deliveryMetrics struct {
	attempts metric.Int64Counter
	duration metric.Float64Histogram
}

func newDeliveryMetrics() deliveryMetrics {
	meter := otel.Meter("github.com/auraedu/notification-service")
	attempts, attemptsErr := meter.Int64Counter(
		"auraedu.notification.deliveries",
		metric.WithDescription("Notification provider delivery attempts by bounded channel and outcome."),
	)
	duration, durationErr := meter.Float64Histogram(
		"auraedu.notification.delivery.duration",
		metric.WithDescription("Notification provider delivery duration in seconds."),
		metric.WithUnit("s"),
	)
	if attemptsErr != nil {
		slog.Error("initialize notification delivery counter", "err", attemptsErr)
	}
	if durationErr != nil {
		slog.Error("initialize notification delivery histogram", "err", durationErr)
	}
	return deliveryMetrics{attempts: attempts, duration: duration}
}

func (m deliveryMetrics) observe(ctx context.Context, channel, outcome string, started time.Time) {
	if m.attempts == nil || m.duration == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("channel", boundedChannel(channel)),
		attribute.String("outcome", outcome),
	)
	m.attempts.Add(ctx, 1, attrs)
	m.duration.Record(ctx, time.Since(started).Seconds(), attrs)
}

func boundedChannel(channel string) string {
	switch channel {
	case "email", "sms", "whatsapp", "in_app", "push":
		return channel
	default:
		return "unknown"
	}
}
