// Package adapters_test runs one contract against every fees repository driver.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/fees-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/fees-service/internal/adapters/postgres"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

type invoiceRepo interface {
	ports.InvoiceRepository
	ports.BalanceRepository
	ports.ReceiptRepository
	ports.PaymentReconciliationRepository
	ports.InvoiceLifecycleRepository
	ports.OutboxRepository
}

type feesDrivers struct {
	structures ports.FeeStructureRepository
	invoices   invoiceRepo
}

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func newStructure(tenantID string) *domain.FeeStructure {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.FeeStructure{
		ID: uuid.NewString(), TenantID: tenantID, Name: "Term 1 tuition",
		AcademicYearID: uuid.NewString(), AmountCents: 50000, Currency: "GHS",
		Recurrence: "termly", Target: "all_students", Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
}

func newInvoice(tenantID, studentID, structureID string, cents int) *domain.Invoice {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Invoice{
		ID: uuid.NewString(), TenantID: tenantID, StudentID: studentID,
		FeeStructureID: structureID, AmountCents: cents, BalanceCents: cents,
		Status: string(domain.InvoiceStatusPending), IssuedAt: now,
		CreatedAt: now, UpdatedAt: now,
	}
}

func runFeesContract(t *testing.T, d feesDrivers) {
	const tenant = "upshs"
	const other = "aboom"
	ctx := tenantCtx(tenant)

	t.Run("an invoice round trips and stays inside its tenant", func(t *testing.T) {
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		inv := newInvoice(tenant, uuid.NewString(), s.ID, 50000)
		if err := d.invoices.Create(ctx, tenant, inv); err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		got, err := d.invoices.GetByID(ctx, tenant, inv.ID)
		if err != nil || got.BalanceCents != 50000 {
			t.Fatalf("round trip changed the invoice: %+v err=%v", got, err)
		}
		if _, err := d.invoices.GetByID(tenantCtx(other), other, inv.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("a payment reduces the balance and issues a receipt", func(t *testing.T) {
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		inv := newInvoice(tenant, uuid.NewString(), s.ID, 50000)
		if err := d.invoices.Create(ctx, tenant, inv); err != nil {
			t.Fatalf("create invoice: %v", err)
		}

		paymentID := uuid.NewString()
		updated, receipt, created, err := d.invoices.ApplyPayment(ctx, tenant, ports.PaymentApplication{
			InvoiceID: inv.ID, PaymentID: paymentID, AmountCents: 20000,
			ReceivedAt: time.Now().UTC().Truncate(time.Second),
		})
		if err != nil || !created {
			t.Fatalf("apply payment: created=%v err=%v", created, err)
		}
		if updated.BalanceCents != 30000 || updated.Status != string(domain.InvoiceStatusPartial) {
			t.Fatalf("balance or status wrong after payment: %+v", updated)
		}
		if receipt.AppliedCents != 20000 || receipt.Currency != "GHS" {
			t.Fatalf("receipt wrong: %+v", receipt)
		}

		// The stored invoice must agree with what was returned.
		stored, err := d.invoices.GetByID(ctx, tenant, inv.ID)
		if err != nil || stored.BalanceCents != 30000 {
			t.Fatalf("stored invoice disagrees: %+v err=%v", stored, err)
		}
		found, err := d.invoices.GetReceiptByID(ctx, tenant, receipt.ID)
		if err != nil || found.PaymentID != paymentID {
			t.Fatalf("receipt not retrievable: %+v err=%v", found, err)
		}
	})

	t.Run("the same payment is never applied twice", func(t *testing.T) {
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		inv := newInvoice(tenant, uuid.NewString(), s.ID, 50000)
		if err := d.invoices.Create(ctx, tenant, inv); err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		paymentID := uuid.NewString()
		application := ports.PaymentApplication{
			InvoiceID: inv.ID, PaymentID: paymentID, AmountCents: 20000,
			ReceivedAt: time.Now().UTC().Truncate(time.Second),
		}
		if _, _, created, err := d.invoices.ApplyPayment(ctx, tenant, application); err != nil || !created {
			t.Fatalf("first application: created=%v err=%v", created, err)
		}
		// A redelivered payment must return the stored result, not spend again.
		updated, _, created, err := d.invoices.ApplyPayment(ctx, tenant, application)
		if err != nil {
			t.Fatalf("replayed application: %v", err)
		}
		if created {
			t.Fatal("a replayed payment was applied a second time")
		}
		if updated.BalanceCents != 30000 {
			t.Fatalf("a replayed payment moved the balance to %d", updated.BalanceCents)
		}
	})

	t.Run("paying the full balance settles the invoice", func(t *testing.T) {
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		inv := newInvoice(tenant, uuid.NewString(), s.ID, 10000)
		if err := d.invoices.Create(ctx, tenant, inv); err != nil {
			t.Fatalf("create invoice: %v", err)
		}
		// Overpaying must not drive the balance negative.
		updated, receipt, _, err := d.invoices.ApplyPayment(ctx, tenant, ports.PaymentApplication{
			InvoiceID: inv.ID, PaymentID: uuid.NewString(), AmountCents: 15000,
			ReceivedAt: time.Now().UTC().Truncate(time.Second),
		})
		if err != nil {
			t.Fatalf("apply payment: %v", err)
		}
		if updated.BalanceCents != 0 || updated.Status != string(domain.InvoiceStatusPaid) {
			t.Fatalf("full payment did not settle the invoice: %+v", updated)
		}
		if receipt.AppliedCents != 10000 || receipt.OverpaymentCents != 5000 {
			t.Fatalf("overpayment not recorded: applied=%d over=%d", receipt.AppliedCents, receipt.OverpaymentCents)
		}
	})

	t.Run("a balance totals per currency and stays inside the tenant", func(t *testing.T) {
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		studentID := uuid.NewString()
		for range 2 {
			if err := d.invoices.Create(ctx, tenant, newInvoice(tenant, studentID, s.ID, 25000)); err != nil {
				t.Fatalf("create invoice: %v", err)
			}
		}
		balance, err := d.invoices.GetStudentBalance(ctx, tenant, studentID)
		if err != nil || len(balance.Totals) != 1 {
			t.Fatalf("balance: %+v err=%v", balance, err)
		}
		if balance.Totals[0].Currency != "GHS" || balance.Totals[0].OutstandingCents != 50000 {
			t.Fatalf("balance totals wrong: %+v", balance.Totals[0])
		}
		empty, err := d.invoices.GetStudentBalance(tenantCtx(other), other, studentID)
		if err == nil && len(empty.Totals) != 0 {
			t.Fatalf("a balance leaked across tenants: %+v", empty.Totals)
		}
	})

	t.Run("a lifecycle commit queues its events", func(t *testing.T) {
		drainFees(t, d.invoices)
		s := newStructure(tenant)
		if err := d.structures.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create structure: %v", err)
		}
		inv := newInvoice(tenant, uuid.NewString(), s.ID, 30000)
		err := d.invoices.CommitInvoiceLifecycle(ctx, tenant, inv, ports.InvoiceMutationCreate,
			[]ports.LifecycleEvent{{EventType: "invoice.issued.v1", Payload: map[string]any{"invoice_id": inv.ID}}})
		if err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		if _, err := d.invoices.GetByID(ctx, tenant, inv.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the invoice: %v", err)
		}
		claimed, err := d.invoices.ClaimPendingFeeEvents(ctx, 100)
		if err != nil || len(claimed) != 1 || claimed[0].EventType != "invoice.issued.v1" {
			t.Fatalf("claimed %d events: %+v err=%v", len(claimed), claimed, err)
		}
		if err := d.invoices.MarkFeeEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := d.invoices.ClaimPendingFeeEvents(ctx, 100)
		if err != nil || len(again) != 0 {
			t.Fatalf("a published event was claimed again: %d err=%v", len(again), err)
		}
	})
}

func drainFees(t *testing.T, repo invoiceRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingFeeEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkFeeEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresFeesRepositoriesSatisfyTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runFeesContract(t, feesDrivers{
		structures: pgadapter.NewFeeStructureRepository(pg.DB),
		invoices:   pgadapter.NewInvoiceRepository(pg.DB),
	})
}

func TestMongoFeesRepositoriesSatisfyTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runFeesContract(t, feesDrivers{
		structures: mongoadapter.NewStructureRepository(mg.Store),
		invoices:   mongoadapter.NewInvoiceRepository(mg.Store),
	})
}
