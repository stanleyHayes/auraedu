package adapters_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/fees-service/internal/adapters/mongo"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestMongoFeesPaymentAndReferences(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
		t.Fatal(err)
	}
	structures := mongoadapter.NewStructureRepository(db.Store)
	r := mongoadapter.NewInvoiceRepository(db.Store)
	s := newStructure("a")
	if err := structures.Create(ctx, "a", s); err != nil {
		t.Fatal(err)
	}
	a, b := newInvoice("a", uuid.NewString(), s.ID, 1000), newInvoice("a", uuid.NewString(), s.ID, 1000)
	for _, i := range []*domain.Invoice{a, b} {
		if err := r.Create(ctx, "a", i); err != nil {
			t.Fatal(err)
		}
	}
	foreign := newInvoice("b", uuid.NewString(), s.ID, 1000)
	if err := r.Create(ctx, "b", foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign fee structure accepted", err)
	}
	payment := uuid.NewString()
	var wins atomic.Int64
	var wg sync.WaitGroup
	for n := range 8 {
		wg.Go(func() {
			id := a.ID
			if n%2 == 1 {
				id = b.ID
			}
			_, _, created, err := r.ApplyPayment(ctx, "a", ports.PaymentApplication{InvoiceID: id, PaymentID: payment, AmountCents: 100, ReceivedAt: time.Now()})
			if err != nil {
				t.Error(err)
			}
			if created {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("payment applied %d times", wins.Load())
	}
	one, err := r.GetByID(ctx, "a", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	two, err := r.GetByID(ctx, "a", b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if one.BalanceCents+two.BalanceCents != 1900 {
		t.Fatal("cross-invoice payment duplicated")
	}
	paid := one
	if two.BalanceCents < 1000 {
		paid = two
	}
	if err = r.Delete(ctx, "a", paid.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("receipt-bearing invoice deleted", err)
	}
	if err = structures.Delete(ctx, "a", s.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("referenced fee structure deleted", err)
	}
}
