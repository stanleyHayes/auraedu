package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/campaign-service/internal/ports"
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

func TestCampaignPublisherConformsToAuthoritativeContract(t *testing.T) {
	const tenantID = "readiness-academy"
	createdAt := time.Date(2026, time.July, 20, 9, 0, 0, 0, time.UTC)
	campaign, err := domain.NewCampaign(domain.CreateInput{
		TenantID: tenantID, Name: "Admissions 2027", Objective: "Increase qualified applications",
		Channel: "website", AudienceDefinition: "Prospective senior high school graduates",
		Currency: "GHS", ProgrammeIDs: []string{uuid.NewString()}, Budget: 5000,
		StartAt: createdAt.Add(24 * time.Hour), EndAt: createdAt.Add(30 * 24 * time.Hour),
		OwnerUserID: uuid.NewString(),
	}, createdAt)
	if err != nil {
		t.Fatalf("new campaign: %v", err)
	}
	previous := campaign.Status
	if err := campaign.Submit(campaign.OwnerUserID, createdAt.Add(time.Hour)); err != nil {
		t.Fatalf("submit campaign: %v", err)
	}
	payload := ports.StatusChangedEventData(campaign, previous)
	capture := &captureJetStream{}
	publisher := New(eventbus.NewPublisher(capture))

	if err := publisher.Publish(context.Background(), "campaign.status_changed.v1", tenantID, payload); err != nil {
		t.Fatalf("publish campaign transition: %v", err)
	}
	assertCampaignContract(t, capture.messages[len(capture.messages)-1], tenantID, campaign.ID, "")

	eventID := uuid.NewString()
	if err := publisher.PublishWithID(context.Background(), eventID, "campaign.status_changed.v1", tenantID, payload); err != nil {
		t.Fatalf("publish outbox campaign transition: %v", err)
	}
	assertCampaignContract(t, capture.messages[len(capture.messages)-1], tenantID, campaign.ID, eventID)
}

func assertCampaignContract(t *testing.T, message *nats.Msg, tenantID, campaignID, eventID string) {
	t.Helper()
	event := testkit.AssertEventContract(t, "campaign.status_changed.v1", message.Data)
	if event["source"] != "campaign-service" || event["tenant_id"] != tenantID {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("event data is not an object: %+v", event["data"])
	}
	if data["campaign_id"] != campaignID {
		t.Fatalf("unexpected campaign payload: %+v", data)
	}
	if _, serialized := event["idempotency_key"]; serialized {
		t.Fatalf("transport deduplication key leaked into strict envelope: %+v", event)
	}
	if eventID != "" && (event["id"] != eventID || message.Header.Get("Nats-Msg-Id") != eventID) {
		t.Fatalf("durable event identity is not stable: event=%+v header=%q", event, message.Header.Get("Nats-Msg-Id"))
	}
}
