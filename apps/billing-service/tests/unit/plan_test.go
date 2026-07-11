package unit

import (
	"testing"

	"github.com/auraedu/billing-service/internal/domain"
)

func TestNewPlan_RequiresName(t *testing.T) {
	if _, err := domain.NewPlan("", "starter", "GHS", "monthly", 0, nil, nil); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewPlan_RequiresCode(t *testing.T) {
	if _, err := domain.NewPlan("Starter", "", "GHS", "monthly", 0, nil, nil); err == nil {
		t.Fatal("expected error when code is empty")
	}
}

func TestNewPlan_RequiresNonNegativePrice(t *testing.T) {
	if _, err := domain.NewPlan("Starter", "starter", "GHS", "monthly", -1, nil, nil); err == nil {
		t.Fatal("expected error when price_cents is negative")
	}
}

func TestNewPlan_DefaultsCurrency(t *testing.T) {
	p, err := domain.NewPlan("Starter", "starter", "", "monthly", 1000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Currency != "GHS" {
		t.Fatalf("expected default currency GHS, got %q", p.Currency)
	}
}

func TestNewPlan_RequiresValidInterval(t *testing.T) {
	if _, err := domain.NewPlan("Starter", "starter", "GHS", "weekly", 1000, nil, nil); err == nil {
		t.Fatal("expected error for invalid billing_interval")
	}
}

func TestNewPlan_NormalizesCode(t *testing.T) {
	p, err := domain.NewPlan("Starter", "STARTER", "GHS", "monthly", 1000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Code != "starter" {
		t.Fatalf("expected lowercase code, got %q", p.Code)
	}
}

func TestPlan_HasFeature(t *testing.T) {
	p, err := domain.NewPlan("Starter", "starter", "GHS", "monthly", 1000, nil, []string{"billing", "students"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.HasFeature("billing") {
		t.Fatal("expected plan to have billing feature")
	}
	if p.HasFeature("ai") {
		t.Fatal("expected plan not to have ai feature")
	}
}

func TestPlan_ApplyUpdate(t *testing.T) {
	p, err := domain.NewPlan("Starter", "starter", "GHS", "monthly", 1000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "Updated Starter"
	price := 2000
	status := string(domain.PlanStatusArchived)
	changed, err := p.ApplyUpdate(domain.PlanPatch{
		Name:       &name,
		PriceCents: &price,
		Status:     &status,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if p.Name != name || p.PriceCents != price || p.Status != status {
		t.Fatalf("update not applied: %+v", p)
	}
}

func TestPlan_ApplyUpdate_InvalidStatus(t *testing.T) {
	p, err := domain.NewPlan("Starter", "starter", "GHS", "monthly", 1000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "deleted"
	if _, err := p.ApplyUpdate(domain.PlanPatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestPlan_Validate_InvalidStatus(t *testing.T) {
	p, _ := domain.NewPlan("Starter", "starter", "GHS", "monthly", 1000, nil, nil)
	p.Status = "unknown"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
