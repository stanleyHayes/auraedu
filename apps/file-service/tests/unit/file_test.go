package unit

import (
	"testing"

	"github.com/auraedu/file-service/internal/domain"
)

func TestNewFile_RequiresTenant(t *testing.T) {
	if _, err := domain.NewFile("id-1", "", "Acme"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewFile_Valid(t *testing.T) {
	e, err := domain.NewFile("id-1", "upshs", "Acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "upshs" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
}
