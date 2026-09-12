package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/billing-service/internal/domain"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"

	"testing"
	"time"
)

func replicaStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMongoBillingReferenceRace(t *testing.T) {
	ctx := context.Background()
	s := replicaStore(t)
	plans := NewPlanRepository(s)
	subs := NewSubscriptionRepository(s)
	invoices := NewSaaSInvoiceRepository(s)
	for iteration := range 10 {
		plan, err := domain.NewPlan("Basic", fmt.Sprintf("basic-%d", iteration), "GHS", "monthly", 100, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := plans.Create(ctx, plan); err != nil {
			t.Fatal(err)
		}
		sub, err := domain.NewSubscription("tenant-a", plan.ID, time.Now(), time.Now().Add(time.Hour), "active", nil)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		created, deleted := make(chan error, 1), make(chan error, 1)
		go func() { <-start; created <- subs.Create(ctx, "tenant-a", sub) }()
		go func() { <-start; deleted <- plans.Delete(ctx, plan.ID) }()
		close(start)
		ce, de := <-created, <-deleted
		if ce == nil {
			if !errors.Is(de, domain.ErrConflict) {
				t.Fatalf("plan not restricted %v", de)
			}
			if _, err := plans.GetByID(ctx, plan.ID); err != nil {
				t.Fatalf("plan rollback %v", err)
			}
		} else {
			if !errors.Is(ce, domain.ErrNotFound) || de != nil {
				t.Fatalf("plan race create=%v delete=%v", ce, de)
			}
			continue
		}
		inv, err := domain.NewSaaSInvoice("tenant-a", sub.ID, 100, nil)
		if err != nil {
			t.Fatal(err)
		}
		start = make(chan struct{})
		go func() { <-start; created <- invoices.Create(ctx, "tenant-a", inv) }()
		go func() { <-start; deleted <- subs.Delete(ctx, "tenant-a", sub.ID) }()
		close(start)
		ce, de = <-created, <-deleted
		if ce == nil {
			if !errors.Is(de, domain.ErrConflict) {
				t.Fatalf("subscription not restricted %v", de)
			}
			if _, err := subs.GetByID(ctx, "tenant-a", sub.ID); err != nil {
				t.Fatalf("subscription rollback %v", err)
			}
		} else if !errors.Is(ce, domain.ErrNotFound) || de != nil {
			t.Fatalf("invoice race create=%v delete=%v", ce, de)
		}
	}
}
