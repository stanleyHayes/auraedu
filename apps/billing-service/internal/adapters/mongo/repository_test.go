package mongo

import (
	"context"
	"errors"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"

	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMongoBillingTrialAndOutbox(t *testing.T) {
	ctx := context.Background()
	store := replicaStore(t)
	if err := EnsureIndexes(ctx, store); err != nil {
		t.Error(err)
		return
	}
	plans := NewPlanRepository(store)
	subs := NewSubscriptionRepository(store)
	invoices := NewSaaSInvoiceRepository(store)
	plan, err := domain.NewPlan("Basic", "basic", "GHS", "monthly", 100, nil, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if err := plans.Create(ctx, plan); err != nil {
		t.Error(err)
		return
	}
	duplicate := *plan
	duplicate.ID = "different"
	if err := plans.Create(ctx, &duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("plan uniqueness %v", err)
	}
	var won atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := domain.NewSubscription("tenant-a", plan.ID, time.Now(), time.Now().Add(time.Hour), "trialing", nil)
			if err != nil {
				t.Error(err)
				return
			}
			err = subs.CommitSubscriptionLifecycle(ctx, "tenant-a", ports.BillingMutationSubscriptionCreate, s, []ports.LifecycleEvent{{TenantID: "tenant-a", EventType: "subscription.created.v1", Payload: map[string]any{"subscription_id": s.ID}}})
			if err == nil {
				won.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
		return
	}
	if won.Load() != 1 {
		t.Fatalf("trial winners %d", won.Load())
	}
	records, _, err := subs.List(ctx, "tenant-a", ports.SubscriptionFilter{Limit: 100})
	if err != nil || len(records) != 1 {
		t.Fatalf("trials %d %v", len(records), err)
	}
	sub := records[0]
	if _, err = subs.GetByID(ctx, "tenant-b", sub.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant scope %v", err)
	}
	inv, err := domain.NewSaaSInvoice("tenant-a", sub.ID, 100, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if err = invoices.Create(ctx, "tenant-b", inv); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross tenant invoice reference %v", err)
	}
	if err = invoices.CommitInvoiceLifecycle(ctx, "tenant-a", ports.BillingMutationInvoiceCreate, inv, []ports.LifecycleEvent{{TenantID: "tenant-b", EventType: "invoice.created.v1"}}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("event tenant scope %v", err)
	}
	if err = invoices.CommitInvoiceLifecycle(ctx, "tenant-a", ports.BillingMutationInvoiceCreate, inv, []ports.LifecycleEvent{{TenantID: "tenant-a", EventType: "invoice.created.v1", Payload: map[string]any{"invoice_id": inv.ID}}}); err != nil {
		t.Error(err)
		return
	}
	events, err := subs.ClaimPendingBillingEvents(ctx, 100)
	if err != nil || len(events) != 2 {
		t.Fatalf("combined outbox %d %v", len(events), err)
	}
	for _, e := range events {
		if err = subs.MarkBillingEventPublished(ctx, e.ID); err != nil {
			t.Error(err)
			return
		}
	}
	sub.Status = "active"
	if err = subs.Update(ctx, "tenant-a", sub); err != nil {
		t.Error(err)
		return
	}
	next, err := domain.NewSubscription("tenant-a", plan.ID, time.Now(), time.Now().Add(time.Hour), "trialing", nil)
	if err != nil {
		t.Error(err)
		return
	}
	if err = subs.Create(ctx, "tenant-a", next); err != nil {
		t.Fatalf("trial slot not released %v", err)
	}
}
