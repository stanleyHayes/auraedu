package unit

import (
	"testing"

	"github.com/auraedu/notification-service/internal/domain"
)

func TestNewTemplate_RequiresTenant(t *testing.T) {
	if _, err := domain.NewTemplate("", "welcome", "email", "Hello", "Body"); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewTemplate_RequiresName(t *testing.T) {
	if _, err := domain.NewTemplate("tenant-1", "", "email", "Hello", "Body"); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewTemplate_Valid(t *testing.T) {
	tmpl, err := domain.NewTemplate("tenant-1", "welcome", "email", "Hello", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Status != string(domain.TemplateStatusActive) {
		t.Fatalf("expected active status, got %q", tmpl.Status)
	}
}

func TestTemplate_ApplyUpdate(t *testing.T) {
	tmpl, _ := domain.NewTemplate("tenant-1", "welcome", "email", "Hello", "Body")
	status := string(domain.TemplateStatusArchived)
	changed, err := tmpl.ApplyUpdate(domain.TemplatePatch{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "status" {
		t.Fatalf("expected status changed, got %v", changed)
	}
	if tmpl.Status != string(domain.TemplateStatusArchived) {
		t.Fatalf("expected archived status, got %q", tmpl.Status)
	}
}
