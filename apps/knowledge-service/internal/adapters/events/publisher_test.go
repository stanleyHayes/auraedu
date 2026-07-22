package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/knowledge-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ message *nats.Msg }

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.message = message
	return &nats.PubAck{}, nil
}
func (*captureJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}
func (*captureJetStream) AddStream(config *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *config}, nil
}
func (*captureJetStream) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return &nats.Subscription{}, nil
}

func TestPublishWithIDPreservesOutboxIdentity(t *testing.T) {
	js := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	eventID := uuid.NewString()
	source := domain.Source{
		ID:          uuid.NewString(),
		TenantID:    "school-one",
		SourceType:  "programme",
		Locale:      "en",
		Version:     1,
		EffectiveAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
	}
	payload := ports.ApprovalEventData(source)
	if err := publisher.PublishWithID(context.Background(), eventID, "knowledge.source_approved.v1", source.TenantID, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if js.message == nil {
		t.Fatal("no event published")
	}
	if js.message.Header.Get("Nats-Msg-Id") != eventID {
		t.Fatalf("stable JetStream identity missing: %v", js.message.Header)
	}
	event := testkit.AssertEventContract(t, "knowledge.source_approved.v1", js.message.Data)
	if event["id"] != eventID || event["subject"] != payload["source_id"] || event["type"] != "knowledge.source_approved.v1" {
		t.Fatalf("outbox identity not preserved: %+v", event)
	}
}
