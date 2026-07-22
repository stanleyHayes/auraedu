// Package events adapts the platform eventbus to the audit service subscriber port.
package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
)

const subjectAll = ">"

// Subscriber consumes all events from the NATS JetStream event bus and
// dispatches them to a ports.Handler.
type Subscriber struct {
	js      eventbus.JetStreamContext
	sub     *eventbus.Subscription
	handler ports.Handler
	log     *slog.Logger
	metrics *observ.WorkerMetrics
}

// NewSubscriber creates a NATS JetStream subscriber.
func NewSubscriber(js eventbus.JetStreamContext, log *slog.Logger, metrics ...*observ.WorkerMetrics) *Subscriber {
	if log == nil {
		log = slog.Default()
	}
	var workerMetrics *observ.WorkerMetrics
	if len(metrics) > 0 {
		workerMetrics = metrics[0]
	}
	return &Subscriber{js: js, log: log, metrics: workerMetrics}
}

// Start registers a durable consumer named "audit-sink" on all AURA events.
func (s *Subscriber) Start(_ context.Context, handler ports.Handler) error {
	s.handler = handler
	sub, err := eventbus.Subscribe(
		s.js,
		eventbus.EventStreamName,
		"audit-sink",
		subjectAll,
		s.handleEvent,
		nil,
	)
	if err != nil {
		return fmt.Errorf("events: subscribe: %w", err)
	}
	s.sub = sub
	s.log.Info("audit sink subscriber started", "subject", "AURA.>", "consumer", "audit-sink")
	return nil
}

// Stop unsubscribes from NATS.
func (s *Subscriber) Stop() error {
	if s.sub != nil {
		if err := s.sub.Unsubscribe(); err != nil {
			return err
		}
		s.sub = nil
	}
	return nil
}

func (s *Subscriber) handleEvent(ctx context.Context, event tenancy.CloudEvent) error {
	started := time.Now()
	if err := s.handler(ctx, event); err != nil {
		s.metrics.Observe(ctx, "audit-sink", started, err)
		s.log.Error("events: handler failed", "err", err)
		return err
	}
	s.metrics.Observe(ctx, "audit-sink", started, nil)
	return nil
}
