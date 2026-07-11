// Package events implements the NATS JetStream subscriber for analytics projections.
package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/auraedu/analytics-service/internal/application"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
)

// Subscriber consumes NATS JetStream events and projects them into analytics metrics.
type Subscriber struct {
	js         eventbus.JetStreamContext
	projection *application.Projection
	log        *slog.Logger
	subs       []*eventbus.Subscription
}

var _ ports.Subscriber = (*Subscriber)(nil)

// eventTypes lists the CloudEvent types this projection cares about.
func eventTypes() []string {
	return []string{
		"student.enrolled.v1",
		"attendance.marked.v1",
		"assessment.score_recorded.v1",
		"payment.received.v1",
		"invoice.created.v1",
		"report.published.v1",
	}
}

// NewSubscriber creates a NATS-backed subscriber.
func NewSubscriber(js eventbus.JetStreamContext, projection *application.Projection, log *slog.Logger) *Subscriber {
	if log == nil {
		log = slog.Default()
	}
	return &Subscriber{js: js, projection: projection, log: log}
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
		sub, err := eventbus.Subscribe(s.js, "AURA", "analytics-projection", et, s.projection.ProcessEvent, nil)
		if err != nil {
			return fmt.Errorf("events: subscribe %s: %w", et, err)
		}
		s.subs = append(s.subs, sub)
		s.log.Info("subscribed to event type", "type", et)
	}
	return nil
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
