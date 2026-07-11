// Package eventbus wraps NATS JetStream with AuraEDU conventions: subject
// prefixes, tenant propagation via headers and a minimal mockable interface.
package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type Publisher struct {
	js     JetStreamContext
	prefix string
}

func NewPublisher(js JetStreamContext) *Publisher {
	return &Publisher{js: js, prefix: "AURA"}
}

func (p *Publisher) Publish(ctx context.Context, event tenancy.CloudEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("eventbus: invalid event: %w", err)
	}
	if event.ID == "" {
		event.ID = uuid.Must(uuid.NewV7()).String()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("eventbus: marshal event: %w", err)
	}

	subject := Subject(p.prefix, event.Type)
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	if event.IdempotencyKey != "" {
		msg.Header.Set("Nats-Msg-Id", event.IdempotencyKey)
	}
	if rid := tenancy.RequestID(ctx); rid != "" {
		msg.Header.Set(tenancy.HeaderRequestID, rid)
	}

	_, err = p.js.PublishMsg(msg, nats.Context(ctx))
	return err
}

func Subject(prefix, eventType string) string {
	return fmt.Sprintf("%s.%s", prefix, eventType)
}

type Subscription struct {
	sub      *nats.Subscription
	handler  Handler
	dlq      DLQ
	stream   string
	consumer string
}

type Handler func(ctx context.Context, event tenancy.CloudEvent) error

type DLQ interface {
	DeadLetter(ctx context.Context, event tenancy.CloudEvent, err error) error
}

type DLQFunc func(ctx context.Context, event tenancy.CloudEvent, err error) error

func (f DLQFunc) DeadLetter(ctx context.Context, event tenancy.CloudEvent, err error) error {
	return f(ctx, event, err)
}

func noopDLQ() DLQ {
	return DLQFunc(func(context.Context, tenancy.CloudEvent, error) error { return nil })
}

func Subscribe(js JetStreamContext, stream, consumer, eventType string, h Handler, dlq DLQ) (*Subscription, error) {
	if dlq == nil {
		dlq = noopDLQ()
	}
	subject := Subject("AURA", eventType)
	sub, err := js.Subscribe(subject, func(msg *nats.Msg) {
		handleMessage(msg, h, dlq)
	}, nats.Durable(consumer), nats.ManualAck(), nats.AckWait(30*time.Second))
	if err != nil {
		return nil, fmt.Errorf("eventbus: subscribe: %w", err)
	}
	return &Subscription{sub: sub, handler: h, dlq: dlq, stream: stream, consumer: consumer}, nil
}

func (s *Subscription) Unsubscribe() error {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.Unsubscribe()
}

func handleMessage(msg *nats.Msg, h Handler, dlq DLQ) {
	ctx := context.Background()
	var event tenancy.CloudEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		_ = msg.Nak()
		return
	}
	if err := event.Validate(); err != nil {
		_ = msg.Nak()
		return
	}
	ctx = tenancy.WithContext(ctx, event.TenantContext())
	if err := h(ctx, event); err != nil {
		_ = dlq.DeadLetter(ctx, event, err)
		_ = msg.Term()
		return
	}
	_ = msg.Ack()
}

func EnsureStream(js JetStreamContext, stream string) (*nats.StreamInfo, error) {
	info, err := js.StreamInfo(stream)
	if err == nil && info != nil {
		return info, nil
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return nil, fmt.Errorf("eventbus: stream info: %w", err)
	}
	return js.AddStream(&nats.StreamConfig{
		Name:      stream,
		Subjects:  []string{"AURA.*"},
		Retention: nats.WorkQueuePolicy,
		MaxMsgs:   1_000_000,
		MaxAge:    7 * 24 * time.Hour,
	})
}

// JetStreamContext is the minimal subset of nats.JetStreamContext used by the
// event bus. It lets tests and callers substitute a mock.
type JetStreamContext interface {
	PublishMsg(msg *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error)
	StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	Subscribe(subject string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
}
