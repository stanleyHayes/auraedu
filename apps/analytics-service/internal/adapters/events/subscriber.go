// Package events implements the NATS JetStream subscriber for analytics projections.
package events

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/analytics-service/internal/application"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
)

// Subscriber consumes NATS JetStream events and projects them into analytics metrics.
type Subscriber struct {
	js         eventbus.JetStreamContext
	projection *application.Projection
	log        *slog.Logger
	subs       []*eventbus.Subscription
	metrics    *observ.WorkerMetrics
}

var _ ports.Subscriber = (*Subscriber)(nil)

// eventTypes lists the CloudEvent types this projection cares about.
func eventTypes() []string {
	return []string{
		"lead.created.v1",
		"application.started.v1",
		"application.submitted.v1",
		"application.admitted.v1",
		"offer.issued.v1",
		"offer.accepted.v1",
		"student.enrolled.v1",
		"attendance.marked.v1",
		"assessment.score_recorded.v1",
		"assessment.score_updated.v1",
		"assessment.score_deleted.v1",
		"payment.received.v1",
		"invoice.created.v1",
		"report.published.v1",
	}
}

// NewSubscriber creates a NATS-backed subscriber.
func NewSubscriber(js eventbus.JetStreamContext, projection *application.Projection, log *slog.Logger, metrics ...*observ.WorkerMetrics) *Subscriber {
	if log == nil {
		log = slog.Default()
	}
	var workerMetrics *observ.WorkerMetrics
	if len(metrics) > 0 {
		workerMetrics = metrics[0]
	}
	return &Subscriber{js: js, projection: projection, log: log, metrics: workerMetrics}
}

// Start subscribes durable consumers for all supported event types.
func (s *Subscriber) Start(_ context.Context) error {
	if s.js == nil {
		return fmt.Errorf("events: JetStream context is nil")
	}
	if _, err := eventbus.EnsureStream(s.js, "AURA"); err != nil {
		return fmt.Errorf("events: ensure stream: %w", err)
	}
	for _, et := range eventTypes() {
		// A JetStream durable consumer owns one filter subject. Reusing one
		// durable name for every event type makes the second subscription fail
		// with a consumer-filter mismatch.
		consumer := "analytics-projection-" + strings.ReplaceAll(et, ".", "-")
		sub, err := eventbus.Subscribe(s.js, "AURA", consumer, et, s.processEvent, nil)
		if err != nil {
			return fmt.Errorf("events: subscribe %s: %w", et, err)
		}
		s.subs = append(s.subs, sub)
		s.log.Info("subscribed to event type", "type", et)
	}
	return nil
}

func (s *Subscriber) processEvent(ctx context.Context, event tenancy.CloudEvent) error {
	started := time.Now()
	err := s.projection.ProcessEvent(ctx, event)
	s.metrics.Observe(ctx, "projection", started, err)
	return err
}

// Stop unsubscribes all active subscriptions.
func (s *Subscriber) Stop() error {
	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.log.Warn("failed to unsubscribe", "err", err)
		}
	}
	s.subs = nil
	return nil
}
