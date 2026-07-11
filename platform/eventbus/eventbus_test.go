package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

type fakeJS struct {
	published []*nats.Msg
	subject   string
	cb        nats.MsgHandler
}

func (f *fakeJS) PublishMsg(msg *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error) {
	f.published = append(f.published, msg)
	return &nats.PubAck{}, nil
}

func (f *fakeJS) StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}

func (f *fakeJS) AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *cfg}, nil
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
	event := tenancy.CloudEvent{SpecVersion: "1.0", Type: "student.enrolled", ID: "evt-1"}
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
	if fake.published[0].Subject != "AURA.student.enrolled" {
		t.Fatalf("unexpected subject: %s", fake.published[0].Subject)
	}
}

func TestEnsureStream(t *testing.T) {
	fake := &fakeJS{}
	info, err := EnsureStream(fake, "AURA")
	if err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	if info.Config.Name != "AURA" {
		t.Fatalf("unexpected stream name: %s", info.Config.Name)
	}
}

func TestSubject(t *testing.T) {
	if got := Subject("AURA", "student.enrolled"); got != "AURA.student.enrolled" {
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

	_, err := Subscribe(fake, "AURA", "student-consumer", "student.enrolled", handler, nil)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if fake.subject != "AURA.student.enrolled" {
		t.Fatalf("unexpected subscription subject: %s", fake.subject)
	}

	data, err := json.Marshal(tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        "student.enrolled",
		ID:          "evt-1",
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
	if msg.Subject != "AURA.dlq.student.enrolled" {
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
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "AURA.dlq.*" {
		t.Fatalf("unexpected dlq subjects: %v", info.Config.Subjects)
	}
}
