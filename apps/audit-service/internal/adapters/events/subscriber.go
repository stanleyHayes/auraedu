package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

const subjectAll = "AURA.>"

// envelope extends the platform CloudEvent with the actorid extension, which is
// not part of the base type.
type envelope struct {
	tenancy.CloudEvent
	ActorID string `json:"actorid,omitempty"`
}

// Subscriber consumes all events from the NATS JetStream event bus and
// dispatches them to a ports.Handler.
type Subscriber struct {
	js      eventbus.JetStreamContext
	sub     *nats.Subscription
	handler ports.Handler
	log     *slog.Logger
}

// NewSubscriber creates a NATS JetStream subscriber.
func NewSubscriber(js eventbus.JetStreamContext, log *slog.Logger) *Subscriber {
	return &Subscriber{js: js, log: log}
}

// Start registers a durable consumer named "audit-sink" on all AURA events.
func (s *Subscriber) Start(ctx context.Context, handler ports.Handler) error {
	s.handler = handler
	sub, err := s.js.Subscribe(subjectAll, s.handleMsg,
		nats.Durable("audit-sink"),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("events: subscribe: %w", err)
	}
	s.sub = sub
	s.log.Info("audit sink subscriber started", "subject", subjectAll, "consumer", "audit-sink")
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

func (s *Subscriber) handleMsg(msg *nats.Msg) {
	ctx := context.Background()
	var env envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		s.log.Error("events: unmarshal message", "err", err)
		_ = msg.Nak()
		return
	}
	if err := env.CloudEvent.Validate(); err != nil {
		s.log.Error("events: invalid cloudevent", "err", err)
		_ = msg.Nak()
		return
	}

	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{
		TenantID: env.CloudEvent.TenantID,
		ActorID:  env.ActorID,
	})

	if err := s.handler(ctx, env.CloudEvent); err != nil {
		s.log.Error("events: handler failed", "err", err)
		_ = msg.Term()
		return
	}
	_ = msg.Ack()
}
