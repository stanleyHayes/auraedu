package integration

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/google/uuid"
)

func TestDeliveryFeedbackReplayOrderingSuppressionAndTenantIsolation(t *testing.T) {
	database := journeyTestDatabase(t)
	repository := postgres.NewMessageRepository(database)
	ctx := withTenant(context.Background(), tenantA)
	addressHash := fmt.Sprintf("%x", sha256.Sum256([]byte("teacher@example.com")))
	message, err := domain.NewMessage(tenantA, recipientA, "email", "Welcome", "Welcome to AuraEDU", nil,
		map[string]any{"delivery_address_hash": addressHash}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(ctx, tenantA, message); err != nil {
		t.Fatal(err)
	}
	acceptedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	providerMessageID := uuid.NewString()
	message.MarkSent()
	message.MarkProviderAccepted("resend", acceptedAt)
	if err := repository.CommitDeliveryOutcome(ctx, tenantA, message, providerMessageID, false, "notification.sent.v1", map[string]any{"message_id": message.ID}); err != nil {
		t.Fatal(err)
	}

	delivered := ports.DeliveryFeedback{
		ID: "msg-delivered-1", Provider: "resend", ProviderMessageID: providerMessageID,
		MessageID: message.ID, EventType: "email.delivered", Status: "delivered",
		AddressHash: addressHash, OccurredAt: acceptedAt.Add(time.Minute),
	}
	applied, err := repository.ApplyDeliveryFeedback(context.Background(), delivered)
	if err != nil || !applied {
		t.Fatalf("delivered applied=%v err=%v", applied, err)
	}
	if applied, err = repository.ApplyDeliveryFeedback(context.Background(), delivered); err != nil || applied {
		t.Fatalf("delivery replay applied=%v err=%v", applied, err)
	}

	older := delivered
	older.ID = "msg-delayed-older"
	older.EventType = "email.delivery_delayed"
	older.Status = "delayed"
	older.OccurredAt = acceptedAt
	if applied, err = repository.ApplyDeliveryFeedback(context.Background(), older); err != nil || !applied {
		t.Fatalf("older audit event applied=%v err=%v", applied, err)
	}
	stored, err := repository.GetByID(ctx, tenantA, message.ID)
	if err != nil || stored.DeliveryStatus == nil || *stored.DeliveryStatus != "delivered" {
		t.Fatalf("out-of-order projection=%+v err=%v", stored, err)
	}
	laterAccepted := delivered
	laterAccepted.ID = "msg-accepted-later"
	laterAccepted.EventType = "email.sent"
	laterAccepted.Status = "accepted"
	laterAccepted.OccurredAt = acceptedAt.Add(90 * time.Second)
	if applied, err = repository.ApplyDeliveryFeedback(context.Background(), laterAccepted); err != nil || !applied {
		t.Fatalf("later lower-state audit event applied=%v err=%v", applied, err)
	}
	stored, err = repository.GetByID(ctx, tenantA, message.ID)
	if err != nil || stored.DeliveryStatus == nil || *stored.DeliveryStatus != "delivered" {
		t.Fatalf("monotonic delivery projection=%+v err=%v", stored, err)
	}

	bounced := delivered
	bounced.ID = "msg-bounced-1"
	bounced.EventType = "email.bounced"
	bounced.Status = "bounced"
	bounced.OccurredAt = acceptedAt.Add(2 * time.Minute)
	if applied, err = repository.ApplyDeliveryFeedback(context.Background(), bounced); err != nil || !applied {
		t.Fatalf("bounce applied=%v err=%v", applied, err)
	}
	suppressed, err := repository.IsEmailSuppressed(ctx, tenantA, addressHash)
	if err != nil || !suppressed {
		t.Fatalf("tenant suppression=%v err=%v", suppressed, err)
	}
	otherTenantSuppressed, err := repository.IsEmailSuppressed(withTenant(context.Background(), tenantB), tenantB, addressHash)
	if err != nil || otherTenantSuppressed {
		t.Fatalf("cross-tenant suppression=%v err=%v", otherTenantSuppressed, err)
	}
	manualHash := fmt.Sprintf("%x", sha256.Sum256([]byte("guardian@example.com")))
	if err := repository.SuppressEmail(context.Background(), tenantA, manualHash, "unsubscribed", "unsubscribe:test-event", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if suppressed, err = repository.IsEmailSuppressed(ctx, tenantA, manualHash); err != nil || !suppressed {
		t.Fatalf("manual opt-out=%v err=%v", suppressed, err)
	}
}
