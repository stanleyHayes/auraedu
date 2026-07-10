package unit

import (
	"testing"

	"github.com/auraedu/staff-service/internal/domain"
)

func TestNewStaff_RequiresTenant(t *testing.T) {
	if _, err := domain.NewStaff("id-1", "", "Acme"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewStaff_Valid(t *testing.T) {
	e, err := domain.NewStaff("id-1", "upshs", "Acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "upshs" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
}
