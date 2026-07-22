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

// EventStreamName is the canonical pub/sub stream. The original "AURA"
// stream used WorkQueuePolicy, which cannot be migrated in place to pub/sub
// retention, so existing installations are moved aside and this stream is
// created without discarding legacy data.
const EventStreamName = "AURA_EVENTS"

// MaxEventBytes is the largest event envelope accepted by consumers. NATS has
// a broker-level payload limit too, but enforcing this here keeps the policy
// stable when brokers are configured differently.
const MaxEventBytes = 1 << 20

// ErrEventTooLarge is returned before an oversized envelope reaches NATS.
// Callers can use errors.Is to distinguish the permanent payload failure from
// a transient broker failure.
var ErrEventTooLarge = errors.New("eventbus: event envelope exceeds maximum size")

func NewPublisher(js JetStreamContext) *Publisher {
	return &Publisher{js: js, prefix: "AURA"}
}

func (p *Publisher) Publish(ctx context.Context, event tenancy.CloudEvent) error {
	// Assign the event ID before validating: Validate rejects empty IDs, so
	// generating afterwards would make this fallback unreachable and silently
	// fail every caller that lets the bus assign IDs.
	if event.ID == "" {
		event.ID = uuid.Must(uuid.NewV7()).String()
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("eventbus: invalid event: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("eventbus: marshal event: %w", err)
	}
	if len(data) > MaxEventBytes {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrEventTooLarge, len(data), MaxEventBytes)
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
		if _, err := EnsureDLQStream(js); err != nil {
			return nil, fmt.Errorf("eventbus: ensure DLQ stream: %w", err)
		}
		dlq = NewJetStreamDLQ(js)
	}
	if err := reconcileConsumerPolicy(js, EventStreamName, consumer); err != nil {
		return nil, fmt.Errorf("eventbus: reconcile consumer: %w", err)
	}
	subject := Subject("AURA", eventType)
	sub, err := js.Subscribe(subject, func(msg *nats.Msg) {
		handleMessage(msg, h, dlq)
	}, nats.Durable(consumer), nats.ManualAck(), nats.AckWait(30*time.Second), nats.MaxDeliver(5))
	if err != nil {
		return nil, fmt.Errorf("eventbus: subscribe: %w", err)
	}
	return &Subscription{sub: sub, handler: h, dlq: dlq, stream: stream, consumer: consumer}, nil
}

type consumerManager interface {
	ConsumerInfo(stream, consumer string, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
	UpdateConsumer(stream string, cfg *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
}

func reconcileConsumerPolicy(js JetStreamContext, stream, consumer string) error {
	manager, ok := js.(consumerManager)
	if !ok {
		return nil
	}
	info, err := manager.ConsumerInfo(stream, consumer)
	if errors.Is(err, nats.ErrConsumerNotFound) || errors.Is(err, nats.ErrStreamNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	config := info.Config
	if config.AckPolicy == nats.AckExplicitPolicy && config.AckWait == 30*time.Second && config.MaxDeliver == 5 {
		return nil
	}
	config.AckPolicy = nats.AckExplicitPolicy
	config.AckWait = 30 * time.Second
	config.MaxDeliver = 5
	_, err = manager.UpdateConsumer(stream, &config)
	return err
}

func (s *Subscription) Unsubscribe() error {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.Unsubscribe()
}

func handleMessage(msg *nats.Msg, h Handler, dlq DLQ) {
	ctx := context.Background()
	if len(msg.Data) > MaxEventBytes {
		_ = msg.Term()
		return
	}
	var event tenancy.CloudEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		_ = msg.Term()
		return
	}
	if err := event.Validate(); err != nil {
		_ = msg.Term()
		return
	}
	ctx = tenancy.WithContext(ctx, event.TenantContext())
	if err := h(ctx, event); err != nil {
		metadata, metadataErr := msg.Metadata()
		if metadataErr == nil && metadata.NumDelivered >= 5 {
			if dlqErr := dlq.DeadLetter(ctx, event, err); dlqErr == nil {
				_ = msg.Term()
				return
			}
		}
		if nakErr := msg.NakWithDelay(2 * time.Second); nakErr != nil {
			_ = msg.Term()
		}
		return
	}
	_ = msg.Ack()
}

func EnsureStream(js JetStreamContext, stream string) (*nats.StreamInfo, error) {
	desired := nats.StreamConfig{
		Name:      EventStreamName,
		Subjects:  []string{"AURA.>"},
		Retention: nats.LimitsPolicy,
		MaxMsgs:   1_000_000,
		MaxAge:    7 * 24 * time.Hour,
	}
	info, err := js.StreamInfo(EventStreamName)
	if err == nil && info != nil {
		if streamConfigMatches(info.Config, desired) {
			return info, nil
		}
		updater, ok := js.(interface {
			UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
		})
		if !ok {
			return nil, fmt.Errorf("eventbus: stream %s requires subject/retention migration", EventStreamName)
		}
		return updater.UpdateStream(&desired)
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return nil, fmt.Errorf("eventbus: stream info: %w", err)
	}

	// Preserve an old WorkQueue stream by moving its filter to a non-overlapping
	// legacy subject. NATS does not allow changing retention to/from WorkQueue.
	legacy, legacyErr := js.StreamInfo(stream)
	if legacyErr == nil && legacy != nil {
		if streamConfigMatches(legacy.Config, nats.StreamConfig{Subjects: desired.Subjects, Retention: desired.Retention}) {
			return legacy, nil
		}
		updater, ok := js.(interface {
			UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
		})
		if !ok {
			return nil, fmt.Errorf("eventbus: legacy stream %s requires subject migration", stream)
		}
		legacyConfig := legacy.Config
		legacyConfig.Subjects = []string{"AURA_LEGACY.>"}
		if _, err := updater.UpdateStream(&legacyConfig); err != nil {
			return nil, fmt.Errorf("eventbus: move legacy stream: %w", err)
		}
	} else if !errors.Is(legacyErr, nats.ErrStreamNotFound) {
		return nil, fmt.Errorf("eventbus: legacy stream info: %w", legacyErr)
	}
	return js.AddStream(&desired)
}

func streamConfigMatches(current, desired nats.StreamConfig) bool {
	return len(current.Subjects) == 1 && current.Subjects[0] == desired.Subjects[0] && current.Retention == desired.Retention
}

// JetStreamContext is the minimal subset of nats.JetStreamContext used by the
// event bus. It lets tests and callers substitute a mock.
type JetStreamContext interface {
	PublishMsg(msg *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error)
	StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	Subscribe(subject string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
}
