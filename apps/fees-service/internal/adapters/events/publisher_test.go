package events

import (
	"context"
	"testing"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
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

func TestFeesPublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	feeStructure, err := domain.NewFeeStructure(
		tenantID,
		"Term tuition",
		uuid.NewString(),
		"GHS",
		string(domain.RecurrenceTermly),
		string(domain.TargetAllStudents),
		125000,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("new fee structure: %v", err)
	}
	dueDate, err := domain.NewDate("2026-08-31")
	if err != nil {
		t.Fatalf("new due date: %v", err)
	}
	invoice, err := domain.NewInvoice(tenantID, uuid.NewString(), feeStructure.ID, feeStructure.AmountCents, feeStructure.AmountCents, dueDate, nil)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}

	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	assertLastContract := func(eventType, subject, eventID string) {
		t.Helper()
		if len(capture.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		event := testkit.AssertEventContract(t, eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "fees-service" || event["tenant_id"] != tenantID || event["subject"] != subject {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		if eventID != "" && event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
		if eventID != "" && capture.messages[len(capture.messages)-1].Header.Get("Nats-Msg-Id") != eventID {
			t.Fatalf("broker deduplication key does not match outbox identity")
		}
	}

	assignment := map[string]any{
		"invoice_id":   invoice.ID,
		"student_id":   invoice.StudentID,
		"amount_cents": invoice.AmountCents,
	}
	if err := publisher.PublishFeeStructure(context.Background(), "fee.assigned.v1", feeStructure, assignment); err != nil {
		t.Fatalf("publish fee assignment: %v", err)
	}
	assertLastContract("fee.assigned.v1", feeStructure.ID, "")

	if err := publisher.PublishInvoice(context.Background(), "invoice.created.v1", invoice, nil); err != nil {
		t.Fatalf("publish invoice creation: %v", err)
	}
	assertLastContract("invoice.created.v1", invoice.ID, "")

	changed := []string{"status", "balance_cents"}
	paid := string(domain.InvoiceStatusPaid)
	if _, err := invoice.ApplyUpdate(domain.InvoicePatch{Status: &paid}); err != nil {
		t.Fatalf("mark invoice paid: %v", err)
	}
	for _, eventType := range []string{"invoice.updated.v1", "invoice.paid.v1", "invoice.deleted.v1"} {
		meta := map[string]any(nil)
		if eventType != "invoice.deleted.v1" {
			meta = map[string]any{"changed_fields": changed}
		}
		if err := publisher.PublishInvoice(context.Background(), eventType, invoice, meta); err != nil {
			t.Fatalf("publish %s: %v", eventType, err)
		}
		assertLastContract(eventType, invoice.ID, "")
	}

	for _, eventType := range []string{"invoice.created.v1", "invoice.updated.v1", "invoice.paid.v1", "invoice.deleted.v1"} {
		eventID := uuid.NewString()
		if err := publisher.PublishWithID(
			context.Background(), eventID, eventType, tenantID, ports.InvoiceEventData(eventType, invoice, nil),
		); err != nil {
			t.Fatalf("publish outbox %s: %v", eventType, err)
		}
		assertLastContract(eventType, invoice.ID, eventID)
	}
}
