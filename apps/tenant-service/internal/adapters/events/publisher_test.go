package events

import (
	"context"
	"testing"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/nats-io/nats.go"
)

type fakeJetStream struct{ messages []*nats.Msg }

func (f *fakeJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	f.messages = append(f.messages, message)
	return &nats.PubAck{Stream: "AURA"}, nil
}
func (*fakeJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}
func (*fakeJetStream) AddStream(config *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *config}, nil
}
func (*fakeJetStream) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return nil, nil
}

func TestPublishWithIDUsesStableCloudEventAndJetStreamDeduplication(t *testing.T) {
	js := &fakeJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	eventID := "11111111-1111-4111-8111-111111111111"
	if err := publisher.PublishWithID(context.Background(), eventID, "tenant.created.v1", "readiness-academy", map[string]any{
		"tenant_code": "readiness-academy", "name": "Readiness Academy", "plan": "growth",
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(js.messages) != 1 {
		t.Fatalf("messages=%d", len(js.messages))
	}
	message := js.messages[0]
	if message.Subject != "AURA.tenant.created.v1" {
		t.Fatalf("subject=%q", message.Subject)
	}
	if got := message.Header.Get("Nats-Msg-Id"); got != eventID {
		t.Fatalf("Nats-Msg-Id=%q, want %q", got, eventID)
	}
	envelope := testkit.AssertEventContract(t, "tenant.created.v1", message.Data)
	if envelope["id"] != eventID || envelope["tenant_id"] != "readiness-academy" || envelope["type"] != "tenant.created.v1" {
		t.Fatalf("event=%+v", envelope)
	}
}
