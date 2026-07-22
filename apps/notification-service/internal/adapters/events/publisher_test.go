package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ messages []*nats.Msg }

const testTenantID = "readiness-academy"

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.messages = append(c.messages, message)
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

func TestNotificationPublishersConformToAuthoritativeContracts(t *testing.T) {
	templateID := uuid.NewString()
	message, err := domain.NewMessage(
		testTenantID,
		uuid.NewString(),
		string(domain.ChannelEmail),
		"Admissions update",
		"Your application has been updated.",
		&templateID,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))

	if err := publisher.PublishMessageSent(context.Background(), message); err != nil {
		t.Fatalf("publish sent: %v", err)
	}
	sent := assertNotificationContract(t, capture.messages[len(capture.messages)-1], "notification.sent.v1", message.ID, "")
	sentData := eventData(t, sent)
	if _, leaked := sentData["template_id"]; leaked {
		t.Fatalf("public sent event leaked internal template identity: %+v", sentData)
	}

	rawProviderError := "smtp rejected private-recipient@example.edu"
	if err := publisher.PublishMessageFailed(context.Background(), message, rawProviderError); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	failed := assertNotificationContract(t, capture.messages[len(capture.messages)-1], "notification.failed.v1", message.ID, "")
	failedData := eventData(t, failed)
	if failedData["reason"] != "delivery_failed" {
		t.Fatalf("failure event retained raw provider details: %+v", failedData)
	}

	for _, testCase := range []struct {
		eventType string
		data      map[string]any
	}{
		{eventType: "notification.sent.v1", data: ports.MessageSentEventData(message)},
		{eventType: "notification.failed.v1", data: ports.MessageFailedEventData(message, rawProviderError)},
	} {
		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, testTenantID, testCase.data); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertNotificationContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, message.ID, eventID)
	}

	journeyID := uuid.NewString()
	changedBy := uuid.NewString()
	journeyEventID := uuid.NewString()
	journeyData := map[string]any{
		"journey_id":    journeyID,
		"status":        "active",
		"trigger_event": "application.started.v1",
		"version":       1,
		"step_count":    2,
		"changed_by":    changedBy,
		"changed_at":    time.Now().UTC().Format(time.RFC3339),
	}
	if err := publisher.PublishWithID(context.Background(), journeyEventID, "communication.journey_changed.v1", testTenantID, journeyData); err != nil {
		t.Fatalf("publish journey lifecycle: %v", err)
	}
	journeyEvent := assertNotificationContract(
		t,
		capture.messages[len(capture.messages)-1],
		"communication.journey_changed.v1",
		journeyID,
		journeyEventID,
	)
	if data := eventData(t, journeyEvent); data["changed_by"] != changedBy {
		t.Fatalf("journey lifecycle actor=%v, want %s", data["changed_by"], changedBy)
	}
}

func eventData(t *testing.T, event map[string]any) map[string]any {
	t.Helper()
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data is not an object: %#v", event["data"])
	}
	return data
}

func assertNotificationContract(t *testing.T, message *nats.Msg, eventType, subject, eventID string) map[string]any {
	t.Helper()
	event := testkit.AssertEventContract(t, eventType, message.Data)
	if event["source"] != "notification-service" || event["tenant_id"] != testTenantID || event["subject"] != subject {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	if eventID != "" {
		if event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
		if message.Header.Get("Nats-Msg-Id") != eventID {
			t.Fatalf("JetStream deduplication key=%q, want %q", message.Header.Get("Nats-Msg-Id"), eventID)
		}
	}
	return event
}
