package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestMongoPaymentAtomicReconciliation(t *testing.T) {
	ctx := context.Background()
	store := testkit.NewMongo(ctx, t).Store
	if err := EnsureIndexes(ctx, store); err != nil {
		t.Fatal(err)
	}
	r := NewPaymentRepository(store)
	tr := NewTransactionRepository(store)
	wr := NewWebhookEventRepository(store)
	p, err := domain.NewPayment("tenant-a", "invoice", "mock", "GHS", 100, json.RawMessage(`{"invoice":true}`))
	if err != nil {
		t.Fatal(err)
	}
	ref := "provider-ref"
	p.ProviderReference = &ref
	p.CheckoutURL = &ref
	if err = r.CommitPaymentLifecycle(ctx, "tenant-a", p, ports.PaymentMutationCreate, "payment.created.v1", map[string]any{"payment_id": p.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetByID(ctx, "tenant-b", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant read: %v", err)
	}
	if err = r.Update(ctx, "tenant-b", p); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant update: %v", err)
	}
	persisted, err := r.GetByID(ctx, "tenant-a", p.ID)
	if err != nil || persisted.CheckoutURL != nil {
		t.Fatalf("transient checkout persisted: %+v %v", persisted, err)
	}
	bad := *p
	bad.Status = "success"
	tx := &domain.Transaction{ID: uuid.NewString(), PaymentID: p.ID, Status: "success", AmountCents: 100, CreatedAt: time.Now().UTC()}
	if err = r.CommitReconciliation(ctx, "tenant-a", &bad, tx, "payment.received.v1", map[string]any{"invalid": make(chan int)}); err == nil {
		t.Fatal("invalid event committed")
	}
	persisted, err = r.GetByID(ctx, "tenant-a", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != "pending" {
		t.Fatal("partial reconciliation")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cloned := *p
			cloned.Status = "success"
			tx := &domain.Transaction{ID: uuid.NewString(), PaymentID: p.ID, Status: "success", AmountCents: 100, CreatedAt: time.Now().UTC()}
			errs <- r.CommitReconciliation(ctx, "tenant-a", &cloned, tx, "payment.received.v1", map[string]any{"payment_id": p.ID})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, _, err := tr.ListByPayment(ctx, "tenant-a", p.ID, ports.TransactionFilter{Limit: 100})
	if err != nil || len(entries) != 1 {
		t.Fatalf("ledger replay: %d %v", len(entries), err)
	}
	if _, err = tr.GetByID(ctx, "tenant-b", entries[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ledger isolation: %v", err)
	}
	failed := *p
	failed.Status = "failed"
	if err = r.CommitReconciliation(ctx, "tenant-a", &failed, tx, "payment.failed.v1", map[string]any{}); err != nil || failed.Status != "success" {
		t.Fatalf("success regressed: %+v %v", failed, err)
	}
	pending, err := r.ClaimPendingPaymentEvents(ctx, 100)
	if err != nil || len(pending) != 2 {
		t.Fatalf("outbox atomicity: %d %v", len(pending), err)
	}
	if again, err := NewPaymentRepository(store).ClaimPendingPaymentEvents(ctx, 100); err != nil || len(again) != 0 {
		t.Fatalf("duplicate lease %d %v", len(again), err)
	}
	for _, e := range pending {
		if err = r.MarkPaymentEventPublished(ctx, e.ID); err != nil {
			t.Fatal(err)
		}
	}
	w, err := domain.NewWebhookEvent("mock", "charge.success", json.RawMessage(`{"data":{"reference":"provider-ref"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	w.MarkProcessed()
	if err = wr.Create(ctx, "tenant-a", w); err != nil {
		t.Fatal(err)
	}
	ok, err := wr.HasProcessedReference(ctx, "tenant-a", "mock", ref)
	if err != nil || !ok {
		t.Fatalf("webhook dedupe %v %v", ok, err)
	}
	ok, err = wr.HasProcessedReference(ctx, "tenant-b", "mock", ref)
	if err != nil || ok {
		t.Fatalf("webhook isolation %v %v", ok, err)
	}
	if err = r.Delete(ctx, "tenant-a", p.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ledger restrict: %v", err)
	}
	removable, err := domain.NewPayment("tenant-a", "invoice", "mock", "GHS", 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Create(ctx, "tenant-a", removable); err != nil {
		t.Fatal(err)
	}
	if err = r.CommitPaymentLifecycle(ctx, "tenant-a", removable, ports.PaymentMutationDelete, "payment.deleted.v1", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	pending, err = r.ClaimPendingPaymentEvents(ctx, 100)
	if err != nil || len(pending) != 1 {
		t.Fatalf("delete lost event %d %v", len(pending), err)
	}
}
