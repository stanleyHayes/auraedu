package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ message *nats.Msg }

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.message = message
	return &nats.PubAck{Stream: "AURA_EVENTS"}, nil
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

func TestRoleChangePublisherConformsToAuthoritativeContract(t *testing.T) {
	const tenantID = "readiness-academy"
	eventID := uuid.NewString()
	roleChange := ports.RoleChangeEvent{
		TenantID: tenantID, UserID: uuid.NewString(), PreviousRole: "teacher",
		NewRole: "principal", Permissions: []string{"users.read", "users.update"},
	}
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	err := publisher.Publish(context.Background(), ports.Event{
		SpecVersion: "1.0", Type: "user.role_changed.v1", Source: "identity-service",
		ID: eventID, Time: time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC),
		TenantID: tenantID, DataContentType: "application/json", Data: ports.RoleChangeEventData(roleChange),
	})
	if err != nil {
		t.Fatalf("publish role change: %v", err)
	}
	if capture.message == nil {
		t.Fatal("publisher did not call JetStream")
	}
	event := testkit.AssertEventContract(t, "user.role_changed.v1", capture.message.Data)
	if event["source"] != "identity-service" || event["tenant_id"] != tenantID || event["id"] != eventID {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	if _, serialized := event["idempotency_key"]; serialized {
		t.Fatalf("transport deduplication key leaked into strict envelope: %+v", event)
	}
	if capture.message.Header.Get("Nats-Msg-Id") != eventID {
		t.Fatalf("JetStream deduplication key=%q, want %q", capture.message.Header.Get("Nats-Msg-Id"), eventID)
	}
}
