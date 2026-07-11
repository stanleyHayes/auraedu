package unit

import (
	"testing"

	"github.com/auraedu/payment-service/internal/domain"
)

func TestNewTransaction_RequiresTenant(t *testing.T) {
	if _, err := domain.NewTransaction("", "pay-1", "credit", "success", "ref-1", 10000); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewTransaction_RequiresPaymentID(t *testing.T) {
	if _, err := domain.NewTransaction("tenant-1", "", "credit", "success", "ref-1", 10000); err == nil {
		t.Fatal("expected error when payment_id is empty")
	}
}

func TestNewTransaction_RequiresReference(t *testing.T) {
	if _, err := domain.NewTransaction("tenant-1", "pay-1", "credit", "success", "", 10000); err == nil {
		t.Fatal("expected error when reference is empty")
	}
}

func TestNewTransaction_RequiresNonNegativeAmount(t *testing.T) {
	if _, err := domain.NewTransaction("tenant-1", "pay-1", "credit", "success", "ref-1", -1); err == nil {
		t.Fatal("expected error when amount_cents is negative")
	}
}

func TestNewTransaction_Valid(t *testing.T) {
	tx, err := domain.NewTransaction("tenant-1", "pay-1", "credit", "success", "ref-1", 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Status != string(domain.TransactionStatusSuccess) {
		t.Fatalf("expected status success, got %q", tx.Status)
	}
	if tx.Type != string(domain.TransactionTypeCredit) {
		t.Fatalf("expected type credit, got %q", tx.Type)
	}
}

func TestTransaction_Validate_InvalidType(t *testing.T) {
	tx, err := domain.NewTransaction("tenant-1", "pay-1", "credit", "success", "ref-1", 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tx.Type = "unknown"
	if err := tx.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestTransaction_Validate_InvalidStatus(t *testing.T) {
	tx, err := domain.NewTransaction("tenant-1", "pay-1", "credit", "success", "ref-1", 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tx.Status = "unknown"
	if err := tx.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
