package mongo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func TestMongoDeliveryJourneyAndConcurrency(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := EnsureIndexes(ctx, s); err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepository(s)
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	m := &domain.Message{ID: "scheduled", TenantID: "a", RecipientID: "recipient", Status: "pending", ScheduledAt: &past, CreatedAt: now, Metadata: map[string]any{"delivery_address_hash": "hash"}}
	if err := r.Create(ctx, "a", m); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetByID(ctx, "b", m.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("tenant isolation", err)
	}
	var claims atomic.Int64
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			items, err := r.ClaimDue(ctx, 100, time.Minute)
			if err != nil {
				t.Error(err)
			}
			claims.Add(int64(len(items)))
		})
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatalf("scheduler claimed %d", claims.Load())
	}
	provider := "resend"
	accepted := "accepted"
	m.Provider = &provider
	m.Status = "sent"
	m.DeliveryStatus = &accepted
	m.SentAt = &now
	if err := r.CommitDeliveryOutcome(ctx, "a", m, "provider-id", false, "notification.sent.v1", map[string]any{"message_id": m.ID}); err != nil {
		t.Fatal(err)
	}
	events, err := r.ClaimPendingNotificationEvents(ctx, 100)
	if err != nil || len(events) != 1 {
		t.Fatalf("outcome event %+v %v", events, err)
	}
	f := ports.DeliveryFeedback{ID: "feedback", MessageID: m.ID, Provider: provider, ProviderMessageID: "provider-id", AddressHash: "hash", Status: "bounced", EventType: "email.bounced", OccurredAt: now}
	var applied atomic.Int64
	for range 8 {
		wg.Go(func() {
			ok, err := r.ApplyDeliveryFeedback(ctx, f)
			if err != nil {
				t.Error(err)
			}
			if ok {
				applied.Add(1)
			}
		})
	}
	wg.Wait()
	if applied.Load() != 1 {
		t.Fatalf("feedback applied %d", applied.Load())
	}
	suppressed, err := r.IsEmailSuppressed(ctx, "a", "hash")
	if err != nil || !suppressed {
		t.Fatal("suppression missing", err)
	}
	suppressed, err = r.IsEmailSuppressed(ctx, "b", "hash")
	if err != nil || suppressed {
		t.Fatal("suppression tenant leak", err)
	}
	f.ID = "late-delivery"
	f.Status = "delivered"
	f.OccurredAt = now.Add(time.Hour)
	if _, err = r.ApplyDeliveryFeedback(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetByID(ctx, "a", m.ID)
	if err != nil || got.DeliveryStatus == nil || *got.DeliveryStatus != "bounced" {
		t.Fatalf("monotone feedback %+v %v", got, err)
	}
	jr := NewJourneyRepository(s)
	j := &domain.Journey{ID: "journey", TenantID: "a", Status: "active", Version: 1, CreatedBy: "admin", CancelOnEvents: []string{"application.submitted.v1"}}
	if err = jr.CreateJourney(ctx, "a", j); err != nil {
		t.Fatal(err)
	}
	var enrolled atomic.Int64
	for n := range 8 {
		wg.Go(func() {
			e := ports.JourneyEnrollment{ID: fmt.Sprint("enrollment-", n), TenantID: "a", JourneyID: j.ID, EventID: "trigger", LeadID: "lead"}
			e.Messages = []*domain.Message{{ID: fmt.Sprint("journey-message-", n), TenantID: "a", Status: "pending", Metadata: map[string]any{"journey_id": j.ID, "journey_enrollment_id": e.ID}}}
			ok, err := jr.EnrollJourney(ctx, e)
			if err != nil {
				t.Error(err)
			}
			if ok {
				enrolled.Add(1)
			}
		})
	}
	wg.Wait()
	if enrolled.Load() != 1 {
		t.Fatalf("enrolled %d times", enrolled.Load())
	}
	stats, err := jr.JourneyStats(ctx, "a", j.ID)
	if err != nil || stats.Enrolled != 1 || stats.Scheduled != 1 {
		t.Fatalf("enrollment stats %+v %v", stats, err)
	}
	cancelled, err := jr.CancelJourneysForEvent(ctx, "a", "lead", "cancel-event", "application.submitted.v1")
	if err != nil || cancelled != 1 {
		t.Fatalf("cancel %d %v", cancelled, err)
	}
	stats, err = jr.JourneyStats(ctx, "a", j.ID)
	if err != nil || stats.Cancelled != 1 || stats.Scheduled != 0 {
		t.Fatalf("cancel stats %+v %v", stats, err)
	}
	// A mid-enrollment duplicate message rolls the enrollment and preceding message back.
	bad := ports.JourneyEnrollment{ID: "bad", TenantID: "a", JourneyID: j.ID, EventID: "bad-trigger", Messages: []*domain.Message{{ID: "new-before-failure", Status: "pending"}, {ID: m.ID, Status: "pending"}}}
	_, err = jr.EnrollJourney(ctx, bad)
	if err == nil {
		t.Fatal("duplicate message must fail enrollment")
	}
	if _, err = r.GetByID(ctx, "a", "new-before-failure"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial enrollment write", err)
	}
	stats, err = jr.JourneyStats(ctx, "a", j.ID)
	if err != nil || stats.Enrolled != 1 {
		t.Fatalf("partial enrollment ledger %+v %v", stats, err)
	}
	ledger := NewProcessedEventRepository(s)
	var processed atomic.Int64
	for range 8 {
		wg.Go(func() {
			ok, err := ledger.Claim(ctx, "a", "event", "type")
			if err != nil {
				t.Error(err)
			}
			if ok {
				processed.Add(1)
			}
		})
	}
	wg.Wait()
	if processed.Load() != 1 {
		t.Fatalf("processed %d times", processed.Load())
	}
	if ok, err := ledger.Claim(ctx, "b", "event", "type"); err != nil || !ok {
		t.Fatal("tenant-bound event dedupe", err)
	}
}
func testStore(t *testing.T) *pmongo.Store {
	t.Helper()
	ctx := context.Background()
	c, err := tcmongo.Run(ctx, "mongo:8.0", tcmongo.WithReplicaSet("rs0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s, err := pmongo.Open(ctx, pmongo.Config{URI: uri + "&directConnection=true", Database: "adapter_test", MaxPoolSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { require.NoError(t, s.Close(context.Background())) })
	return s
}
