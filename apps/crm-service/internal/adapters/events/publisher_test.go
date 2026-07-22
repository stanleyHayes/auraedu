package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct {
	message *nats.Msg
}

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

func assertPublishedContract(t *testing.T, capture *captureJetStream, eventType string) {
	t.Helper()
	if capture.message == nil {
		t.Fatal("publisher did not call JetStream")
	}
	event := testkit.AssertEventContract(t, eventType, capture.message.Data)
	if event["type"] != eventType || event["source"] != "crm-service" || event["tenant_id"] != "tenant-a" {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	capture.message = nil
}

func TestCRMEventPublishersConformToAuthoritativeContracts(t *testing.T) {
	now := time.Date(2026, time.July, 20, 10, 30, 0, 0, time.UTC)
	leadID := uuid.NewString()
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	ctx := context.Background()

	lead := &domain.Lead{ID: leadID, TenantID: "tenant-a", Source: "website", Stage: domain.StageNew, CreatedAt: now}
	if err := publisher.LeadCreated(ctx, lead); err != nil {
		t.Fatalf("publish lead: %v", err)
	}
	assertPublishedContract(t, capture, "lead.created.v1")

	score := domain.LeadScore{Score: 72, Confidence: "high", PositiveFactors: []domain.ScoreFactor{{Code: "engaged"}}, RuleVersion: "v1", EvaluatedAt: now}
	if err := publisher.LeadScored(ctx, "tenant-a", leadID, score); err != nil {
		t.Fatalf("publish score: %v", err)
	}
	assertPublishedContract(t, capture, "lead.scored.v1")

	interaction := &domain.Interaction{ID: uuid.NewString(), TenantID: "tenant-a", LeadID: leadID, Channel: "website", Direction: "inbound", ActorType: "prospect", OccurredAt: now}
	if err := publisher.InteractionCreated(ctx, interaction); err != nil {
		t.Fatalf("publish interaction: %v", err)
	}
	assertPublishedContract(t, capture, "lead.interaction_created.v1")

	feedback := &domain.Feedback{ID: uuid.NewString(), TenantID: "tenant-a", FeedbackType: "helpful", CreatedAt: now}
	if err := publisher.FeedbackSubmitted(ctx, feedback); err != nil {
		t.Fatalf("publish feedback: %v", err)
	}
	assertPublishedContract(t, capture, "growth.feedback_submitted.v1")

	callback := &domain.CallbackRequest{ID: uuid.NewString(), TenantID: "tenant-a", LeadID: leadID, PreferredAt: now.Add(time.Hour), Timezone: "Africa/Accra", Locale: "en-GH", Status: domain.CallbackRequested, CreatedAt: now}
	if err := publisher.CallbackRequested(ctx, callback); err != nil {
		t.Fatalf("publish callback: %v", err)
	}
	assertPublishedContract(t, capture, "growth.callback_requested.v1")
}
