package unit

import (
	"testing"

	"github.com/auraedu/fees-service/internal/domain"
)

func TestNewInvoice_RequiresTenant(t *testing.T) {
	if _, err := domain.NewInvoice("", "student-1", "fs-1", 10000, 10000, domain.Date{}, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewInvoice_RequiresStudentID(t *testing.T) {
	if _, err := domain.NewInvoice("tenant-1", "", "fs-1", 10000, 10000, domain.Date{}, nil); err == nil {
		t.Fatal("expected error when student_id is empty")
	}
}

func TestNewInvoice_RequiresFeeStructureID(t *testing.T) {
	if _, err := domain.NewInvoice("tenant-1", "student-1", "", 10000, 10000, domain.Date{}, nil); err == nil {
		t.Fatal("expected error when fee_structure_id is empty")
	}
}

func TestNewInvoice_RequiresNonNegativeAmount(t *testing.T) {
	if _, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", -1, 0, domain.Date{}, nil); err == nil {
		t.Fatal("expected error when amount_cents is negative")
	}
}

func TestNewInvoice_BalanceDefaultsToAmount(t *testing.T) {
	inv, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, -1, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.BalanceCents != 10000 {
		t.Fatalf("expected balance_cents to default to amount_cents, got %d", inv.BalanceCents)
	}
}

func TestNewInvoice_BalanceCannotExceedAmount(t *testing.T) {
	if _, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 10001, domain.Date{}, nil); err == nil {
		t.Fatal("expected error when balance_cents exceeds amount_cents")
	}
}

func TestNewInvoice_Valid(t *testing.T) {
	inv, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 5000, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Status != string(domain.InvoiceStatusPending) {
		t.Fatalf("expected status pending, got %q", inv.Status)
	}
	if inv.BalanceCents != 5000 {
		t.Fatalf("expected balance 5000, got %d", inv.BalanceCents)
	}
}

func TestInvoice_ApplyUpdate_StatusToPaidZerosBalance(t *testing.T) {
	inv, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 10000, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := string(domain.InvoiceStatusPaid)
	changed, err := inv.ApplyUpdate(domain.InvoicePatch{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Status != status {
		t.Fatalf("status not updated: got %q", inv.Status)
	}
	if inv.BalanceCents != 0 {
		t.Fatalf("expected balance zeroed on paid transition, got %d", inv.BalanceCents)
	}
	if !contains(changed, "balance_cents") {
		t.Fatal("expected balance_cents in changed fields")
	}
}

func TestInvoice_ApplyUpdate_InvalidStatus(t *testing.T) {
	inv, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 10000, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := inv.ApplyUpdate(domain.InvoicePatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestInvoice_ApplyUpdate_AmountResetsBalance(t *testing.T) {
	inv, err := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 10000, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	amount := 5000
	changed, err := inv.ApplyUpdate(domain.InvoicePatch{AmountCents: &amount})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.AmountCents != 5000 || inv.BalanceCents != 5000 {
		t.Fatalf("expected amount and balance reset to 5000, got amount=%d balance=%d", inv.AmountCents, inv.BalanceCents)
	}
	if !contains(changed, "balance_cents") {
		t.Fatal("expected balance_cents in changed fields")
	}
}

func TestInvoice_Validate_InvalidStatus(t *testing.T) {
	inv, _ := domain.NewInvoice("tenant-1", "student-1", "fs-1", 10000, 10000, domain.Date{}, nil)
	inv.Status = "unknown"
	if err := inv.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
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
