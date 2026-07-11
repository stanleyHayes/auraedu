package unit

import (
	"testing"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
)

func TestNewSaaSInvoice_RequiresTenant(t *testing.T) {
	if _, err := domain.NewSaaSInvoice("", "sub-1", 1000, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewSaaSInvoice_RequiresSubscription(t *testing.T) {
	if _, err := domain.NewSaaSInvoice("tenant-1", "", 1000, nil); err == nil {
		t.Fatal("expected error when subscription_id is empty")
	}
}

func TestNewSaaSInvoice_RequiresNonNegativeAmount(t *testing.T) {
	if _, err := domain.NewSaaSInvoice("tenant-1", "sub-1", -1, nil); err == nil {
		t.Fatal("expected error when amount_cents is negative")
	}
}

func TestSaaSInvoice_MarkPaid(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := inv.MarkPaid(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Status != string(domain.SaaSInvoiceStatusPaid) {
		t.Fatalf("expected status paid, got %q", inv.Status)
	}
	if inv.PaidAt == nil {
		t.Fatal("expected paid_at to be set")
	}
}

func TestSaaSInvoice_MarkPaid_RejectsVoid(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inv.Status = string(domain.SaaSInvoiceStatusVoid)
	if err := inv.MarkPaid(); err == nil {
		t.Fatal("expected error when paying void invoice")
	}
}

func TestSaaSInvoice_MarkVoid(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := inv.MarkVoid(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Status != string(domain.SaaSInvoiceStatusVoid) {
		t.Fatalf("expected status void, got %q", inv.Status)
	}
}

func TestSaaSInvoice_MarkVoid_RejectsPaid(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inv.Status = string(domain.SaaSInvoiceStatusPaid)
	if err := inv.MarkVoid(); err == nil {
		t.Fatal("expected error when voiding paid invoice")
	}
}

func TestSaaSInvoice_ApplyUpdate(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	amount := 2000
	status := string(domain.SaaSInvoiceStatusOpen)
	due := time.Now().UTC().AddDate(0, 0, 7)
	changed, err := inv.ApplyUpdate(domain.SaaSInvoicePatch{
		AmountCents: &amount,
		Status:      &status,
		DueDate:     &due,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if inv.AmountCents != amount || inv.Status != status {
		t.Fatalf("update not applied: %+v", inv)
	}
}

func TestSaaSInvoice_ApplyUpdate_InvalidStatus(t *testing.T) {
	inv, err := domain.NewSaaSInvoice("tenant-1", "sub-1", 1000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "pending"
	if _, err := inv.ApplyUpdate(domain.SaaSInvoicePatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}
