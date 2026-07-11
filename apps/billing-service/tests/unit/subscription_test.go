package unit

import (
	"testing"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
)

func TestNewSubscription_RequiresTenant(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewSubscription("", "plan-1", now, now.AddDate(0, 1, 0), "active", nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewSubscription_RequiresPlan(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewSubscription("tenant-1", "", now, now.AddDate(0, 1, 0), "active", nil); err == nil {
		t.Fatal("expected error when plan_id is empty")
	}
}

func TestNewSubscription_RequiresValidStatus(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "pending", nil); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestNewSubscription_RequiresValidPeriod(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, -1, 0), "active", nil); err == nil {
		t.Fatal("expected error when period_end before period_start")
	}
}

func TestSubscription_ChangePlan(t *testing.T) {
	now := time.Now().UTC()
	s, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "active", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.ChangePlan("plan-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.PlanID != "plan-2" {
		t.Fatalf("expected plan_id plan-2, got %q", s.PlanID)
	}
}

func TestSubscription_ChangePlan_RejectsEmpty(t *testing.T) {
	now := time.Now().UTC()
	s, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "active", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.ChangePlan(""); err == nil {
		t.Fatal("expected error for empty plan_id")
	}
}

func TestSubscription_ChangePlan_RejectsCancelled(t *testing.T) {
	now := time.Now().UTC()
	s, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "cancelled", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.ChangePlan("plan-2"); err == nil {
		t.Fatal("expected error when changing cancelled subscription")
	}
}

func TestSubscription_ApplyUpdate(t *testing.T) {
	now := time.Now().UTC()
	s, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "active", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := string(domain.SubscriptionStatusPastDue)
	end := now.AddDate(0, 2, 0)
	changed, err := s.ApplyUpdate(domain.SubscriptionPatch{
		Status:           &status,
		CurrentPeriodEnd: &end,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if s.Status != status {
		t.Fatalf("expected status %q, got %q", status, s.Status)
	}
}

func TestSubscription_ApplyUpdate_InvalidPeriod(t *testing.T) {
	now := time.Now().UTC()
	s, err := domain.NewSubscription("tenant-1", "plan-1", now, now.AddDate(0, 1, 0), "active", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	start := now.AddDate(0, 2, 0)
	end := now.AddDate(0, 1, 0)
	if _, err := s.ApplyUpdate(domain.SubscriptionPatch{
		CurrentPeriodStart: &start,
		CurrentPeriodEnd:   &end,
	}); err == nil {
		t.Fatal("expected error when end before start")
	}
}
