package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

type fakeJS struct {
	published []*nats.Msg
	subject   string
	cb        nats.MsgHandler
	stream    *nats.StreamInfo
	consumer  *nats.ConsumerInfo
	updated   *nats.ConsumerConfig
}

func (f *fakeJS) ConsumerInfo(_ string, _ string, _ ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	if f.consumer == nil {
		return nil, nats.ErrConsumerNotFound
	}
	return f.consumer, nil
}

func (f *fakeJS) UpdateConsumer(_ string, cfg *nats.ConsumerConfig, _ ...nats.JSOpt) (*nats.ConsumerInfo, error) {
	updated := *cfg
	f.updated = &updated
	return &nats.ConsumerInfo{Config: updated}, nil
}

func (f *fakeJS) PublishMsg(msg *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error) {
	f.published = append(f.published, msg)
	return &nats.PubAck{}, nil
}

func (f *fakeJS) StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	if f.stream != nil && f.stream.Config.Name == stream {
		return f.stream, nil
	}
	return nil, nats.ErrStreamNotFound
}

func (f *fakeJS) AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.stream = &nats.StreamInfo{Config: *cfg}
	return f.stream, nil
}

func (f *fakeJS) UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.stream = &nats.StreamInfo{Config: *cfg}
	return f.stream, nil
}

func (f *fakeJS) Subscribe(subject string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	f.subject = subject
	f.cb = cb
	return &nats.Subscription{}, nil
}

func TestPublisherValidatesEvent(t *testing.T) {
	fake := &fakeJS{}
	pub := NewPublisher(fake)

	ctx := context.Background()
	event := tenancy.CloudEvent{SpecVersion: "1.0", Type: "student.enrolled.v1", Source: "student-service", ID: "evt-1", Time: "2026-07-20T10:00:00Z", Data: json.RawMessage(`{"student_id":"s1"}`)}
	if err := pub.Publish(ctx, event); !errors.Is(err, tenancy.ErrMissingEventTenant) {
		t.Fatalf("expected missing tenant error, got %v", err)
	}

	event.TenantID = "upshs"
	if err := pub.Publish(ctx, event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fake.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(fake.published))
	}
	if fake.published[0].Subject != "AURA.student.enrolled.v1" {
		t.Fatalf("unexpected subject: %s", fake.published[0].Subject)
	}
}

func TestPublisherRejectsOversizedEnvelopeBeforeBroker(t *testing.T) {
	fake := &fakeJS{}
	pub := NewPublisher(fake)
	event := tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.enrolled.v1",
		Source:      "student-service",
		ID:          "evt-oversized",
		Time:        "2026-07-20T10:00:00Z",
		TenantID:    "upshs",
		Data:        json.RawMessage(`{"content":"` + strings.Repeat("x", MaxEventBytes) + `"}`),
	}

	err := pub.Publish(context.Background(), event)
	if !errors.Is(err, ErrEventTooLarge) {
		t.Fatalf("expected oversized event error, got %v", err)
	}
	if len(fake.published) != 0 {
		t.Fatalf("oversized event reached broker: %d messages", len(fake.published))
	}
}

func TestEnsureStream(t *testing.T) {
	fake := &fakeJS{}
	info, err := EnsureStream(fake, "AURA")
	if err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	if info.Config.Name != EventStreamName {
		t.Fatalf("unexpected stream name: %s", info.Config.Name)
	}
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "AURA.>" || info.Config.Retention != nats.LimitsPolicy {
		t.Fatalf("unexpected stream config: %+v", info.Config)
	}
}

func TestEnsureStreamRepairsLegacyFilterAndRetention(t *testing.T) {
	fake := &fakeJS{stream: &nats.StreamInfo{Config: nats.StreamConfig{Name: "AURA", Subjects: []string{"AURA.*"}, Retention: nats.WorkQueuePolicy}}}
	info, err := EnsureStream(fake, "AURA")
	if err != nil {
		t.Fatalf("repair stream: %v", err)
	}
	if info.Config.Name != EventStreamName || info.Config.Subjects[0] != "AURA.>" || info.Config.Retention != nats.LimitsPolicy {
		t.Fatalf("legacy stream not repaired: %+v", info.Config)
	}
}

func TestSubject(t *testing.T) {
	if got := Subject("AURA", "student.enrolled.v1"); got != "AURA.student.enrolled.v1" {
		t.Fatalf("unexpected subject: %s", got)
	}
}

func TestHandleMessage(t *testing.T) {
	fake := &fakeJS{}
	var received *tenancy.CloudEvent
	handler := func(ctx context.Context, event tenancy.CloudEvent) error {
		received = &event
		return nil
	}

	_, err := Subscribe(fake, "AURA", "student-consumer", "student.enrolled.v1", handler, nil)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if fake.subject != "AURA.student.enrolled.v1" {
		t.Fatalf("unexpected subscription subject: %s", fake.subject)
	}

	data, err := json.Marshal(tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.enrolled.v1",
		Source:      "student-service",
		ID:          "evt-1",
		Time:        "2026-07-20T10:00:00Z",
		TenantID:    "upshs",
		Data:        json.RawMessage(`{"id":"s1"}`),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	msg := &nats.Msg{Data: data}
	fake.cb(msg)

	if received == nil {
		t.Fatal("expected event to be handled")
	}
	if received.TenantID != "upshs" {
		t.Fatalf("unexpected tenant: %s", received.TenantID)
	}
}

func TestHandleMessageRejectsOversizedEnvelope(t *testing.T) {
	called := false
	handleMessage(
		&nats.Msg{Data: make([]byte, MaxEventBytes+1)},
		func(context.Context, tenancy.CloudEvent) error {
			called = true
			return nil
		},
		noopDLQ(),
	)
	if called {
		t.Fatal("oversized event reached handler")
	}
}

func TestSubscribeReconcilesExistingDurablePolicy(t *testing.T) {
	fake := &fakeJS{consumer: &nats.ConsumerInfo{Config: nats.ConsumerConfig{
		Durable:       "student-consumer",
		FilterSubject: "AURA.student.enrolled.v1",
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       time.Minute,
		MaxDeliver:    -1,
	}}}
	_, err := Subscribe(fake, EventStreamName, "student-consumer", "student.enrolled.v1", func(context.Context, tenancy.CloudEvent) error {
		return nil
	}, noopDLQ())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if fake.updated == nil {
		t.Fatal("expected existing durable to be updated")
	}
	if fake.updated.Durable != "student-consumer" || fake.updated.FilterSubject != "AURA.student.enrolled.v1" {
		t.Fatalf("consumer identity changed: %+v", fake.updated)
	}
	if fake.updated.AckPolicy != nats.AckExplicitPolicy || fake.updated.AckWait != 30*time.Second || fake.updated.MaxDeliver != 5 {
		t.Fatalf("retry policy not reconciled: %+v", fake.updated)
	}
}

func TestJetStreamDLQDeadLetter(t *testing.T) {
	fake := &fakeJS{}
	dlq := NewJetStreamDLQ(fake)

	event := tenancy.CloudEvent{
		SpecVersion:    "1.0",
		Type:           "student.enrolled",
		ID:             "evt-1",
		TenantID:       "upshs",
		IdempotencyKey: "idem-1",
		Data:           json.RawMessage(`{"id":"s1"}`),
	}
	handlerErr := errors.New("student not found")

	if err := dlq.DeadLetter(context.Background(), event, handlerErr); err != nil {
		t.Fatalf("dead letter: %v", err)
	}

	if len(fake.published) != 1 {
		t.Fatalf("expected 1 dlq message, got %d", len(fake.published))
	}
	msg := fake.published[0]
	if msg.Subject != "AURA_DLQ.student.enrolled" {
		t.Fatalf("unexpected dlq subject: %s", msg.Subject)
	}
	if msg.Header.Get("Nats-Msg-Id") != "idem-1" {
		t.Fatalf("expected idempotency key header")
	}

	var entry dlqEvent
	if err := json.Unmarshal(msg.Data, &entry); err != nil {
		t.Fatalf("unmarshal dlq entry: %v", err)
	}
	if entry.Original.ID != event.ID {
		t.Fatalf("unexpected original event id: %s", entry.Original.ID)
	}
	if entry.Error != handlerErr.Error() {
		t.Fatalf("unexpected error string: %s", entry.Error)
	}
	if entry.Timestamp.IsZero() {
		t.Fatal("expected timestamp")
	}
}

func TestEnsureDLQStream(t *testing.T) {
	fake := &fakeJS{}
	info, err := EnsureDLQStream(fake)
	if err != nil {
		t.Fatalf("ensure dlq stream: %v", err)
	}
	if info.Config.Name != DLQStreamName {
		t.Fatalf("unexpected dlq stream name: %s", info.Config.Name)
	}
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "AURA_DLQ.>" {
		t.Fatalf("unexpected dlq subjects: %v", info.Config.Subjects)
	}
}
