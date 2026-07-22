package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ message *nats.Msg }

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.message = message
	return &nats.PubAck{Stream: "AURA"}, nil
}
func (*captureJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}
func (*captureJetStream) AddStream(config *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *config}, nil
}
func (*captureJetStream) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return nil, nil
}

func TestQuestionUnansweredUsesCanonicalContractAndStableOutboxID(t *testing.T) {
	js := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	eventID := uuid.NewString()
	response := domain.Response{
		TenantID:  "school-one",
		SessionID: uuid.NewString(),
		MessageID: uuid.NewString(),
		Locale:    "en-GH",
		CreatedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
	}
	if err := publisher.PublishWithID(context.Background(), eventID, "assistant.question_unanswered.v1", response.TenantID, ports.EscalationEventData(response)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if js.message == nil {
		t.Fatal("no event published")
	}
	if got := js.message.Header.Get("Nats-Msg-Id"); got != eventID {
		t.Fatalf("Nats-Msg-Id=%q, want %q", got, eventID)
	}
	event := testkit.AssertEventContract(t, "assistant.question_unanswered.v1", js.message.Data)
	if event["id"] != eventID || event["subject"] != response.MessageID {
		t.Fatalf("event identity=%+v", event)
	}
}
