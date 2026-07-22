package events

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type fakeJetStream struct {
	messages []*nats.Msg
}

func (f *fakeJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	f.messages = append(f.messages, message)
	return &nats.PubAck{Stream: "AURA_EVENTS"}, nil
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

func TestPublishPlanRequiresTenantContext(t *testing.T) {
	js := &fakeJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	plan, err := domain.NewPlan("Growth", "growth", "GHS", "monthly", 12000, nil, nil)
	if err != nil {
		t.Fatalf("new plan: %v", err)
	}

	err = publisher.PublishPlan(context.Background(), "billing.plan_upgraded.v1", plan, map[string]any{
		"previous_plan": "starter",
	})
	if err == nil || !strings.Contains(err.Error(), "tenant_id is required") {
		t.Fatalf("expected tenant_id validation error, got %v", err)
	}
	if len(js.messages) != 0 {
		t.Fatalf("expected no message for invalid tenant context, got %d", len(js.messages))
	}
}

func TestPublishPlanEmitsTenantScopedUpgradeEvent(t *testing.T) {
	const tenantID = "readiness-academy"
	js := &fakeJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	plan, err := domain.NewPlan("Growth", "growth", "GHS", "monthly", 12000, nil, []string{"analytics"})
	if err != nil {
		t.Fatalf("new plan: %v", err)
	}

	if err := publisher.PublishPlan(context.Background(), "billing.plan_upgraded.v1", plan, map[string]any{
		"tenant_id":     tenantID,
		"previous_plan": "starter",
	}); err != nil {
		t.Fatalf("publish plan: %v", err)
	}
	if len(js.messages) != 1 {
		t.Fatalf("messages=%d, want 1", len(js.messages))
	}
	message := js.messages[0]
	if message.Subject != "AURA.billing.plan_upgraded.v1" {
		t.Fatalf("subject=%q", message.Subject)
	}

	event := testkit.AssertEventContract(t, "billing.plan_upgraded.v1", message.Data)
	if event["type"] != "billing.plan_upgraded.v1" || event["tenant_id"] != tenantID || event["subject"] != plan.ID {
		t.Fatalf("unexpected envelope: %+v", event)
	}
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%T, want object", event["data"])
	}
	if data["tenant_id"] != tenantID || data["previous_plan"] != "starter" || data["plan_code"] != "growth" {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestBillingLifecyclePublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	now := time.Date(2026, time.July, 20, 10, 30, 0, 0, time.UTC)
	trialEndsAt := now.Add(14 * 24 * time.Hour)
	planID := uuid.NewString()
	subscription, err := domain.NewSubscription(
		tenantID,
		planID,
		now,
		now.Add(30*24*time.Hour),
		string(domain.SubscriptionStatusTrialing),
		&trialEndsAt,
	)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}

	js := &fakeJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	assertLastContract := func(eventType string) map[string]any {
		t.Helper()
		if len(js.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		message := js.messages[len(js.messages)-1]
		event := testkit.AssertEventContract(t, eventType, message.Data)
		if event["tenant_id"] != tenantID || event["source"] != "billing-service" {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		return event
	}

	if err := publisher.PublishSubscription(context.Background(), "billing.subscription_changed.v1", subscription, map[string]any{
		"plan_key": "growth",
	}); err != nil {
		t.Fatalf("publish subscription change: %v", err)
	}
	assertLastContract("billing.subscription_changed.v1")

	if err := publisher.PublishSubscription(context.Background(), "billing.trial_started.v1", subscription, map[string]any{
		"plan_key":      "growth",
		"trial_ends_at": trialEndsAt.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("publish trial start: %v", err)
	}
	assertLastContract("billing.trial_started.v1")

	invoice, err := domain.NewSaaSInvoice(tenantID, subscription.ID, 12000, nil)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}
	if err := publisher.PublishInvoice(context.Background(), "billing.invoice_created.v1", invoice, nil); err != nil {
		t.Fatalf("publish invoice: %v", err)
	}
	assertLastContract("billing.invoice_created.v1")

	outboxID := uuid.NewString()
	if err := publisher.PublishWithID(context.Background(), outboxID, "billing.invoice_created.v1", tenantID, map[string]any{
		"invoice_id":      invoice.ID,
		"tenant_id":       tenantID,
		"subscription_id": subscription.ID,
		"amount_cents":    invoice.AmountCents,
		"status":          invoice.Status,
	}); err != nil {
		t.Fatalf("publish outbox invoice: %v", err)
	}
	outboxEvent := assertLastContract("billing.invoice_created.v1")
	if outboxEvent["id"] != outboxID || js.messages[len(js.messages)-1].Header.Get("Nats-Msg-Id") != outboxID {
		t.Fatalf("outbox identity is not stable: %+v", outboxEvent)
	}
}
