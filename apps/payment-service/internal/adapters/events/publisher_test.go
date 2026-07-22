package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct {
	messages []*nats.Msg
}

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

func TestPaymentPublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	payment, err := domain.NewPayment(tenantID, uuid.NewString(), string(domain.ProviderPaystack), "GHS", 125000, nil)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	reference := "provider-reference-1"
	completedAt := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	status := string(domain.PaymentStatusSuccess)
	if _, err := payment.ApplyUpdate(domain.PaymentPatch{
		ProviderReference: &reference,
		Status:            &status,
		CompletedAt:       &completedAt,
	}); err != nil {
		t.Fatalf("complete payment: %v", err)
	}

	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	assertLastContract := func(eventType, eventID string) {
		t.Helper()
		if len(capture.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		event := testkit.AssertEventContract(t, eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "payment-service" || event["tenant_id"] != tenantID || event["subject"] != payment.ID {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		if eventID != "" && event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
		if eventID != "" && capture.messages[len(capture.messages)-1].Header.Get("Nats-Msg-Id") != eventID {
			t.Fatalf("broker deduplication key does not match outbox identity")
		}
	}

	testCases := []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "payment.created.v1"},
		{eventType: "payment.initiated.v1", meta: map[string]any{"provider_reference": reference}},
		{eventType: "payment.updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "payment.received.v1", meta: map[string]any{"provider_reference": reference}},
		{eventType: "payment.failed.v1", meta: map[string]any{"reason": "provider declined payment"}},
		{eventType: "payment.deleted.v1"},
	}
	for _, testCase := range testCases {
		if err := publisher.PublishPayment(context.Background(), testCase.eventType, payment, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(
			context.Background(),
			eventID,
			testCase.eventType,
			tenantID,
			ports.PaymentEventData(testCase.eventType, payment, testCase.meta),
		); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, eventID)
	}
}
