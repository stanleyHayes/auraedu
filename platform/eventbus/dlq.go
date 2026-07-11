package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

// DLQStreamName is the default JetStream stream that stores dead-lettered events.
const DLQStreamName = "AURA_DLQ"

// dlqEvent is the envelope persisted to the DLQ stream.
type dlqEvent struct {
	Original  tenancy.CloudEvent `json:"original"`
	Error     string             `json:"error"`
	Timestamp time.Time          `json:"timestamp"`
}

// JetStreamDLQ publishes failed events to a dedicated NATS JetStream DLQ stream.
type JetStreamDLQ struct {
	js     JetStreamContext
	prefix string
	stream string
}

// NewJetStreamDLQ creates a DLQ publisher backed by NATS JetStream.
func NewJetStreamDLQ(js JetStreamContext) *JetStreamDLQ {
	return &JetStreamDLQ{js: js, prefix: "AURA", stream: DLQStreamName}
}

// DeadLetter writes the failed event to the DLQ stream so it can be retried or
// inspected later. It returns the publication error, if any.
func (d *JetStreamDLQ) DeadLetter(ctx context.Context, event tenancy.CloudEvent, err error) error {
	entry := dlqEvent{
		Original:  event,
		Error:     err.Error(),
		Timestamp: time.Now().UTC(),
	}
	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("eventbus: marshal dlq entry: %w", marshalErr)
	}

	subject := fmt.Sprintf("%s.dlq.%s", d.prefix, event.Type)
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

	_, pubErr := d.js.PublishMsg(msg, nats.Context(ctx))
	if pubErr != nil {
		return fmt.Errorf("eventbus: publish dlq event: %w", pubErr)
	}
	return nil
}

// EnsureDLQStream creates the DLQ stream if it does not already exist.
func EnsureDLQStream(js JetStreamContext) (*nats.StreamInfo, error) {
	info, err := js.StreamInfo(DLQStreamName)
	if err == nil && info != nil {
		return info, nil
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return nil, fmt.Errorf("eventbus: dlq stream info: %w", err)
	}
	return js.AddStream(&nats.StreamConfig{
		Name:      DLQStreamName,
		Subjects:  []string{"AURA.dlq.*"},
		Retention: nats.WorkQueuePolicy,
		MaxMsgs:   1_000_000,
		MaxAge:    30 * 24 * time.Hour,
	})
}
