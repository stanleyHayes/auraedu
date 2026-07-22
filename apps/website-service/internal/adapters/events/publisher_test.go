package events

import (
	"context"
	"testing"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const testTenantID = "readiness-academy"

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

func TestWebsitePublishersConformToAuthoritativeContracts(t *testing.T) {
	page, err := domain.NewPage(testTenantID, "about-us", "About us")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	section, err := domain.NewSection(testTenantID, page.ID, domain.SectionTypeHero, domain.Content{"title": "Welcome"}, 0)
	if err != nil {
		t.Fatalf("new section: %v", err)
	}
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))

	pageCases := []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "website.page_created.v1"},
		{eventType: "website.page_updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "website.page_published.v1"},
		{eventType: "website.page_deleted.v1"},
	}
	for _, testCase := range pageCases {
		if err := publisher.PublishPage(context.Background(), testCase.eventType, page, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertWebsiteContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, page.ID, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, testTenantID, ports.PageEventData(page, testCase.meta)); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertWebsiteContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, page.ID, eventID)
	}

	sectionCases := []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "website.section_created.v1"},
		{eventType: "website.section_updated.v1", meta: map[string]any{"changed_fields": []string{"content"}}},
		{eventType: "website.section_deleted.v1"},
	}
	for _, testCase := range sectionCases {
		if err := publisher.PublishSection(context.Background(), testCase.eventType, section, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertWebsiteContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, section.ID, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, testTenantID, ports.SectionEventData(section, testCase.meta)); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertWebsiteContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, section.ID, eventID)
	}
}

func assertWebsiteContract(t *testing.T, message *nats.Msg, eventType, subject, eventID string) {
	t.Helper()
	event := testkit.AssertEventContract(t, eventType, message.Data)
	if event["source"] != "website-service" || event["tenant_id"] != testTenantID || event["subject"] != subject {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	if _, serialized := event["idempotency_key"]; serialized {
		t.Fatalf("transport deduplication key leaked into envelope: %+v", event)
	}
	if eventID != "" && (event["id"] != eventID || message.Header.Get("Nats-Msg-Id") != eventID) {
		t.Fatalf("durable event identity is not stable: event=%+v header=%q", event, message.Header.Get("Nats-Msg-Id"))
	}
}
