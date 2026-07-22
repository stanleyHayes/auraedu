package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/adapters/events"
	"github.com/auraedu/payment-service/internal/domain"
)

// TestPaymentEventData_ContractFields pins the payload shape required by
// contracts/events/payment.received.v1.json (payment_id, invoice_id, amount) and
// payment.failed.v1.json (payment_id).
func TestPaymentEventData_ContractFields(t *testing.T) {
	p, err := domain.NewPayment(unitTenantA, unitInvoice, "paystack", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	ref := "ref-1"
	now := time.Now().UTC()
	if _, err := p.ApplyUpdate(domain.PaymentPatch{ProviderReference: &ref, CompletedAt: &now}); err != nil {
		t.Fatalf("apply update: %v", err)
	}

	data := events.PaymentEventData("payment.received.v1", p, nil)

	for _, key := range []string{"payment_id", "invoice_id", "amount", "gateway"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("contract field %q missing from event data: %v", key, data)
		}
	}
	if data["payment_id"] != p.ID {
		t.Fatalf("payment_id mismatch: %v", data["payment_id"])
	}
	if data["invoice_id"] != unitInvoice {
		t.Fatalf("invoice_id mismatch: %v", data["invoice_id"])
	}
	if data["amount"] != 10000 {
		t.Fatalf("amount mismatch: %v", data["amount"])
	}
	if data["gateway"] != "paystack" {
		t.Fatalf("gateway mismatch: %v", data["gateway"])
	}
	if _, leaked := data["provider_reference"]; leaked {
		t.Fatalf("received event leaked provider reference: %v", data)
	}
	failed := events.PaymentEventData(
		"payment.failed.v1",
		p,
		map[string]any{"reason": "provider reported status failed"},
	)
	if failed["reason"] != "provider reported status failed" {
		t.Fatalf("failed event lost reason: %v", failed)
	}

	// The whole payload must stay JSON-marshalable (CloudEvent data).
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("event data must marshal: %v", err)
	}
	var roundtrip map[string]any
	if err := json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatalf("event data must round-trip: %v", err)
	}
	// amount must remain a JSON number per the contract schema.
	if _, ok := roundtrip["amount"].(float64); !ok {
		t.Fatalf("amount must be a JSON number, got %T", roundtrip["amount"])
	}
}
