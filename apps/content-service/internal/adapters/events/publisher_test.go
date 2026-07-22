package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ messages []*nats.Msg }

func (capture *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	capture.messages = append(capture.messages, message)
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

func TestContentPublisherConformsToPrivacySafeContracts(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	draft := domain.Draft{ID: uuid.NewString(), TenantID: "school-a", ContentType: "social_post", Version: 1, Generator: "openai:model", BrandProfileVersion: 1, CreatedAt: now, UpdatedAt: now, Status: domain.StatusDraft}
	capture := &captureJetStream{}
	publisher := New(eventbus.NewPublisher(capture))
	tests := []struct {
		eventType string
		payload   map[string]any
	}{
		{eventType: "content.draft_generated.v1", payload: ports.DraftGeneratedEventData(draft)},
		{eventType: "content.status_changed.v1", payload: ports.StatusChangedEventData(draft, domain.StatusPendingReview, "reviewer")},
	}
	for _, test := range tests {
		if err := publisher.Publish(context.Background(), test.eventType, draft.TenantID, test.payload); err != nil {
			t.Fatalf("publish %s: %v", test.eventType, err)
		}
		event := testkit.AssertEventContract(t, test.eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "content-service" || event["tenant_id"] != draft.TenantID {
			t.Fatalf("unexpected envelope: %#v", event)
		}
		data, ok := event["data"].(map[string]any)
		if !ok {
			t.Fatalf("event data has unexpected type: %#v", event["data"])
		}
		if data["content_id"] != draft.ID {
			t.Fatalf("unexpected payload: %#v", data)
		}
		for _, prohibited := range []string{"content", "brief", "audience", "facts", "review_note"} {
			if _, exists := data[prohibited]; exists {
				t.Fatalf("private field %q leaked into event: %#v", prohibited, data)
			}
		}
	}
}
