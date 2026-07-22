package events

import (
	"context"
	"testing"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ messages []*nats.Msg }

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

func TestFilePublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	file, err := domain.NewFileUpload(tenantID, "transcript.pdf", "application/pdf", uuid.NewString(), "admissions", 1024, "checksum")
	if err != nil {
		t.Fatalf("new file: %v", err)
	}
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))

	for _, testCase := range []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "file.uploaded.v1"},
		{eventType: "file.updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "file.deleted.v1"},
	} {
		if err := publisher.Publish(context.Background(), testCase.eventType, file, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertFileContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, tenantID, file.ID, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, tenantID, ports.FileEventData(file, testCase.meta)); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertFileContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, tenantID, file.ID, eventID)
	}
}

func assertFileContract(t *testing.T, message *nats.Msg, eventType, tenantID, subject, eventID string) {
	t.Helper()
	event := testkit.AssertEventContract(t, eventType, message.Data)
	if event["source"] != "file-service" || event["tenant_id"] != tenantID || event["subject"] != subject {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	if eventID != "" && event["id"] != eventID {
		t.Fatalf("outbox identity is not stable: %+v", event)
	}
	if eventID != "" && message.Header.Get("Nats-Msg-Id") != eventID {
		t.Fatalf("broker deduplication key does not match outbox identity")
	}
}
