package unit

import (
	"testing"

	"github.com/auraedu/student-service/internal/domain"
)

func TestNewStudent_RequiresTenant(t *testing.T) {
	if _, err := domain.NewStudent("id-1", "", "Acme"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewStudent_Valid(t *testing.T) {
	e, err := domain.NewStudent("id-1", "upshs", "Acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "upshs" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
}
