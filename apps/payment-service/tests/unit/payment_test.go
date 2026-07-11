package unit

import (
	"testing"

	"github.com/auraedu/payment-service/internal/domain"
)

func TestNewPayment_RequiresTenant(t *testing.T) {
	if _, err := domain.NewPayment("", "inv-1", "mock", "GHS", 10000, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewPayment_RequiresInvoiceID(t *testing.T) {
	if _, err := domain.NewPayment("tenant-1", "", "mock", "GHS", 10000, nil); err == nil {
		t.Fatal("expected error when invoice_id is empty")
	}
}

func TestNewPayment_RequiresPositiveAmount(t *testing.T) {
	if _, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 0, nil); err == nil {
		t.Fatal("expected error when amount_cents is zero")
	}
	if _, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", -1, nil); err == nil {
		t.Fatal("expected error when amount_cents is negative")
	}
}

func TestNewPayment_DefaultsCurrency(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Currency != "GHS" {
		t.Fatalf("expected currency GHS, got %q", p.Currency)
	}
}

func TestNewPayment_RequiresValidProvider(t *testing.T) {
	if _, err := domain.NewPayment("tenant-1", "inv-1", "unknown", "GHS", 10000, nil); err == nil {
		t.Fatal("expected error for invalid provider")
	}
}

func TestNewPayment_StatusPending(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != string(domain.PaymentStatusPending) {
		t.Fatalf("expected status pending, got %q", p.Status)
	}
}

func TestPayment_ApplyUpdate_Status(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := string(domain.PaymentStatusProcessing)
	changed, err := p.ApplyUpdate(domain.PaymentPatch{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != status {
		t.Fatalf("status not updated: got %q", p.Status)
	}
	if !contains(changed, "status") {
		t.Fatal("expected status in changed fields")
	}
}

func TestPayment_ApplyUpdate_InvalidStatus(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := p.ApplyUpdate(domain.PaymentPatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestPayment_ApplyUpdate_ProviderReference(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ref := "mock_ref_123"
	if _, err := p.ApplyUpdate(domain.PaymentPatch{ProviderReference: &ref}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ProviderReference == nil || *p.ProviderReference != ref {
		t.Fatal("provider_reference not updated")
	}
}

func TestPayment_Validate_InvalidProvider(t *testing.T) {
	p, err := domain.NewPayment("tenant-1", "inv-1", "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p.Provider = "unknown"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for invalid provider")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
