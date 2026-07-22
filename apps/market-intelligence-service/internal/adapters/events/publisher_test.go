package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/market-intelligence-service/internal/ports"
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

func TestLifecycleEventsConformToAuthoritativeContracts(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	actorID := uuid.NewString()
	cases := []struct {
		eventType string
		id        string
		kind      domain.Kind
		extra     map[string]any
	}{
		{"intelligence.source.created.v1", uuid.NewString(), domain.KindReputation, nil},
		{"intelligence.source.reviewed.v1", uuid.NewString(), domain.KindCompetitor, nil},
		{"intelligence.observation.created.v1", uuid.NewString(), domain.KindReputation, nil},
		{"intelligence.observation.reviewed.v1", uuid.NewString(), domain.KindCompetitor, nil},
		{"intelligence.observation.resolved.v1", uuid.NewString(), domain.KindReputation, nil},
		{"intelligence.alert_rule.updated.v1", "school-one", domain.KindReputation, nil},
		{"intelligence.alert.acknowledged.v1", uuid.NewString(), domain.KindReputation, nil},
		{"intelligence.competitor_summary.created.v1", uuid.NewString(), domain.KindCompetitor, map[string]any{"item_count": 3, "source_count": 2}},
		{"intelligence.competitor_summary.reviewed.v1", uuid.NewString(), domain.KindCompetitor, nil},
		{"intelligence.alert.changed.v1", uuid.NewString(), domain.KindReputation, map[string]any{"category": "recurring_issue", "observation_count": 4, "threshold": 3, "window_days": 7, "reason": "threshold_reached"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.eventType, func(t *testing.T) {
			js := &captureJetStream{}
			publisher := New(eventbus.NewPublisher(js))
			data := ports.LifecycleEventData(testCase.id, testCase.kind, actorID, now)
			for key, value := range testCase.extra {
				data[key] = value
			}
			eventID := uuid.NewString()
			if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, "school-one", data); err != nil {
				t.Fatalf("publish: %v", err)
			}
			if got := js.message.Header.Get("Nats-Msg-Id"); got != eventID {
				t.Fatalf("Nats-Msg-Id=%q, want %q", got, eventID)
			}
			event := testkit.AssertEventContract(t, testCase.eventType, js.message.Data)
			if event["id"] != eventID || event["tenant_id"] != "school-one" {
				t.Fatalf("event identity=%+v", event)
			}
		})
	}
}
