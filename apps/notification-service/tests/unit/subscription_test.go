package unit

import (
	"testing"

	"github.com/auraedu/notification-service/internal/domain"
)

func TestNewSubscription_RequiresTenant(t *testing.T) {
	if _, err := domain.NewSubscription("", "550e8400-e29b-41d4-a716-446655440000", "email", true); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewSubscription_RequiresUser(t *testing.T) {
	if _, err := domain.NewSubscription("tenant-1", "", "email", true); err == nil {
		t.Fatal("expected error when user_id is empty")
	}
}

func TestNewSubscription_Valid(t *testing.T) {
	sub, err := domain.NewSubscription("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sub.IsEnabled {
		t.Fatal("expected subscription enabled")
	}
}

func TestSubscription_ApplyUpdate(t *testing.T) {
	sub, err := domain.NewSubscription("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", true)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	enabled := false
	changed, err := sub.ApplyUpdate(domain.SubscriptionPatch{IsEnabled: &enabled})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "is_enabled" {
		t.Fatalf("expected is_enabled changed, got %v", changed)
	}
	if sub.IsEnabled {
		t.Fatal("expected subscription disabled")
	}
}
