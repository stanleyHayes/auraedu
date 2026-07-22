package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/adapters/postgres"
	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/testkit"
)

func TestPaymentLifecycleMutationsCommitWithOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewPaymentRepository(tdb.DB)
	payment, err := domain.NewPayment(tenantA, invoiceA, "paystack", "GHS", 4200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitPaymentLifecycle(ctx, tenantA, payment, ports.PaymentMutationCreate, "payment.created.v1", ports.PaymentEventData("payment.created.v1", payment, nil)); err != nil {
		t.Fatal(err)
	}
	processing := string(domain.PaymentStatusProcessing)
	changed, err := payment.ApplyUpdate(domain.PaymentPatch{Status: &processing})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitPaymentLifecycle(ctx, tenantA, payment, ports.PaymentMutationUpdate, "payment.updated.v1", ports.PaymentEventData("payment.updated.v1", payment, map[string]any{"changed_fields": changed})); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitPaymentLifecycle(ctx, tenantA, payment, ports.PaymentMutationDelete, "payment.deleted.v1", ports.PaymentEventData("payment.deleted.v1", payment, nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, tenantA, payment.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted payment remained visible: %v", err)
	}
	outbox, err := repo.ClaimPendingPaymentEvents(context.Background(), 10)
	if err != nil || len(outbox) != 3 {
		t.Fatalf("lifecycle outbox=%+v err=%v", outbox, err)
	}
	counts := map[string]int{}
	for _, item := range outbox {
		counts[item.EventType]++
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil || payload["payment_id"] != payment.ID {
			t.Fatalf("lifecycle payload=%+v err=%v", payload, err)
		}
	}
	if counts["payment.created.v1"] != 1 || counts["payment.updated.v1"] != 1 || counts["payment.deleted.v1"] != 1 {
		t.Fatalf("unexpected lifecycle events: %+v", counts)
	}
}

func TestPaymentLifecycleRollsBackWithoutOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewPaymentRepository(tdb.DB)
	payment, err := domain.NewPayment(tenantA, invoiceA, "paystack", "GHS", 4200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE payment_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitPaymentLifecycle(ctx, tenantA, payment, ports.PaymentMutationCreate, "payment.created.v1", ports.PaymentEventData("payment.created.v1", payment, nil)); err == nil {
		t.Fatal("payment create must fail when its durable event cannot be written")
	}
	if _, err := repo.GetByID(ctx, tenantA, payment.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("payment create escaped rollback: %v", err)
	}
}

func TestReconciliationRollsBackWhenOutboxWriteFails(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewPaymentRepository(tdb.DB)
	txRepo := postgres.NewTransactionRepository(tdb.DB)
	payment := mustCreatePayment(ctx, t, repo, invoiceA, "paystack", 4200)

	status := string(domain.PaymentStatusSuccess)
	completedAt := time.Now().UTC()
	if _, err := payment.ApplyUpdate(domain.PaymentPatch{Status: &status, CompletedAt: &completedAt}); err != nil {
		t.Fatal(err)
	}
	transaction, err := domain.NewTransaction(tenantA, payment.ID, "credit", "success", "ref-atomic", payment.AmountCents)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE payment_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitReconciliation(ctx, tenantA, payment, transaction, "payment.received.v1", ports.PaymentEventData("payment.received.v1", payment, nil)); err == nil {
		t.Fatal("reconciliation must fail when its durable event cannot be written")
	}

	stored, err := repo.GetByID(ctx, tenantA, payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(domain.PaymentStatusPending) || stored.CompletedAt != nil {
		t.Fatalf("payment mutation escaped rolled-back transaction: %+v", stored)
	}
	transactions, _, err := txRepo.ListByPayment(ctx, tenantA, payment.ID, ports.TransactionFilter{Limit: 10})
	if err != nil || len(transactions) != 0 {
		t.Fatalf("ledger mutation escaped rolled-back transaction: transactions=%+v err=%v", transactions, err)
	}
}

// configurableProvider lets tests pick the initiate reference and verify outcome.
type configurableProvider struct {
	ref    string
	status string
}

func (c *configurableProvider) Initiate(_ context.Context, p domain.Payment) (string, string, error) {
	return c.ref, "https://checkout.test/" + p.ID, nil
}

func (c *configurableProvider) Verify(_ context.Context, _ string) (string, error) {
	return c.status, nil
}

func newEnabledService(
	pRepo ports.PaymentRepository,
	txRepo ports.TransactionRepository,
	wRepo ports.WebhookEventRepository,
	prov ports.PaymentProvider,
	tenantID string,
) *application.Service {
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantID, application.FeaturePayments, true)
	return application.NewService(pRepo, txRepo, wRepo,
		application.WithFeatureGate(gates),
		application.WithPaymentProvider(prov),
	)
}

func TestWebhookEventRepository_HasProcessedReference(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	_, _, wRepo := newRepos(t)

	w, err := domain.NewWebhookEvent("paystack", "charge.success", []byte(`{"reference":"ref-persist-1"}`), nil)
	if err != nil {
		t.Fatalf("new webhook event: %v", err)
	}
	w.SetTenant(tenantA)
	if err := wRepo.Create(ctx, tenantA, w); err != nil {
		t.Fatalf("create webhook event: %v", err)
	}

	// Unprocessed events must not trip the guard.
	ok, err := wRepo.HasProcessedReference(ctx, tenantA, "paystack", "ref-persist-1")
	if err != nil {
		t.Fatalf("has processed reference: %v", err)
	}
	if ok {
		t.Fatal("unprocessed event must not count as processed")
	}

	w.MarkProcessed()
	if err := wRepo.Update(ctx, tenantA, w); err != nil {
		t.Fatalf("mark processed: %v", err)
	}
	ok, err = wRepo.HasProcessedReference(ctx, tenantA, "paystack", "ref-persist-1")
	if err != nil {
		t.Fatalf("has processed reference: %v", err)
	}
	if !ok {
		t.Fatal("processed event must trip the guard")
	}

	// Nested paystack shape (data.reference) is extracted too.
	nested, err := domain.NewWebhookEvent("paystack", "charge.success", []byte(`{"event":"charge.success","data":{"reference":"ref-persist-2"}}`), nil)
	if err != nil {
		t.Fatalf("new webhook event: %v", err)
	}
	nested.SetTenant(tenantA)
	nested.MarkProcessed()
	if err := wRepo.Create(ctx, tenantA, nested); err != nil {
		t.Fatalf("create nested webhook event: %v", err)
	}
	ok, err = wRepo.HasProcessedReference(ctx, tenantA, "paystack", "ref-persist-2")
	if err != nil {
		t.Fatalf("has processed reference (nested): %v", err)
	}
	if !ok {
		t.Fatal("guard must extract data.reference from the nested shape")
	}

	// Tenant isolation and reference/provider specificity.
	bCtx := withTenant(context.Background(), tenantB)
	cases := []struct {
		name     string
		ctx      context.Context
		tenant   string
		provider string
		ref      string
	}{
		{"other tenant", bCtx, tenantB, "paystack", "ref-persist-1"},
		{"other reference", ctx, tenantA, "paystack", "ref-unknown"},
		{"other provider", ctx, tenantA, "flutterwave", "ref-persist-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := wRepo.HasProcessedReference(tc.ctx, tc.tenant, tc.provider, tc.ref)
			if err != nil {
				t.Fatalf("has processed reference: %v", err)
			}
			if ok {
				t.Fatal("guard must not match")
			}
		})
	}
}

// TestService_ProcessWebhookAppliesOnceAgainstDB proves the idempotency guard against
// a real database: a redelivered webhook records an audit row but never re-applies.
func TestService_ProcessWebhookAppliesOnceAgainstDB(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, wRepo := newRepos(t)

	prov := &configurableProvider{ref: "ref-db-1", status: string(domain.PaymentStatusSuccess)}
	svc := newEnabledService(pRepo, txRepo, wRepo, prov, tenantA)

	p, err := svc.CreatePayment(ctx, actorWithPerms(tenantA, application.PermManage), application.CreatePaymentRequest{
		InvoiceID: invoiceA, AmountCents: 10000, Currency: "GHS", Provider: "paystack",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := svc.InitiatePayment(ctx, actorWithPerms(tenantA, application.PermInitiate), p.ID); err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	req := application.ProcessWebhookRequest{
		Provider: "paystack",
		Payload:  []byte(`{"reference":"ref-db-1","tenant_id":"` + tenantA + `"}`),
	}
	if _, err := svc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if _, err := svc.ProcessWebhook(ctx, req); err != nil {
		t.Fatalf("duplicate delivery: %v", err)
	}

	got, err := pRepo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("expected completed_at persisted")
	}

	txs, _, err := txRepo.ListByPayment(ctx, tenantA, p.ID, ports.TransactionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("duplicate webhook must not double-apply: expected 1 transaction, got %d", len(txs))
	}
	outboxRepo, ok := pRepo.(ports.OutboxRepository)
	if !ok {
		t.Fatalf("payment repository does not expose the outbox contract")
	}
	outbox, err := outboxRepo.ClaimPendingPaymentEvents(context.Background(), 10)
	if err != nil || len(outbox) != 3 {
		t.Fatalf("payment reconciliation outbox=%+v err=%v", outbox, err)
	}
	counts := map[string]int{}
	for _, item := range outbox {
		counts[item.EventType]++
		if item.TenantID != tenantA {
			t.Fatalf("unexpected outbox tenant: %+v", item)
		}
		if item.EventType == "payment.received.v1" {
			var payload map[string]any
			if err := json.Unmarshal(item.Payload, &payload); err != nil {
				t.Fatalf("decode payment outbox: %v", err)
			}
			if payload["payment_id"] != p.ID || payload["invoice_id"] != invoiceA || payload["amount"] != float64(10000) {
				t.Fatalf("unexpected payment outbox payload: %+v", payload)
			}
		}
		if err := outboxRepo.MarkPaymentEventPublished(context.Background(), item.ID); err != nil {
			t.Fatalf("mark payment event published: %v", err)
		}
	}
	if counts["payment.created.v1"] != 1 || counts["payment.initiated.v1"] != 1 || counts["payment.received.v1"] != 1 {
		t.Fatalf("duplicate webhook changed lifecycle event counts: %+v", counts)
	}
	if pending, err := outboxRepo.ClaimPendingPaymentEvents(context.Background(), 10); err != nil || len(pending) != 0 {
		t.Fatalf("published payment outbox must drain: pending=%+v err=%v", pending, err)
	}

	ok, err = wRepo.HasProcessedReference(ctx, tenantA, "paystack", "ref-db-1")
	if err != nil {
		t.Fatalf("has processed reference: %v", err)
	}
	if !ok {
		t.Fatal("expected processed reference recorded")
	}

	events, _, err := wRepo.List(ctx, tenantA, ports.WebhookEventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 audit records (original + redelivery), got %d", len(events))
	}
	for _, e := range events {
		if !e.Processed {
			t.Fatal("all deliveries should be marked processed")
		}
	}
}

// TestService_VerifyPaymentReconcilesAgainstDB covers the manual reconciliation path
// and its tenant scoping against a real database.
func TestService_VerifyPaymentReconcilesAgainstDB(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, wRepo := newRepos(t)

	prov := &configurableProvider{ref: "ref-db-2", status: string(domain.PaymentStatusSuccess)}
	svc := newEnabledService(pRepo, txRepo, wRepo, prov, tenantA)

	p, err := svc.CreatePayment(ctx, actorWithPerms(tenantA, application.PermManage), application.CreatePaymentRequest{
		InvoiceID: invoiceA, AmountCents: 7000, Currency: "GHS", Provider: "paystack",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := svc.InitiatePayment(ctx, actorWithPerms(tenantA, application.PermInitiate), p.ID); err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	got, err := svc.VerifyPayment(ctx, actorWithPerms(tenantA, application.PermInitiate), p.ID)
	if err != nil {
		t.Fatalf("verify payment: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got.Status)
	}

	// Persisted, not just in memory.
	stored, err := pRepo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if stored.Status != string(domain.PaymentStatusSuccess) || stored.CompletedAt == nil {
		t.Fatalf("reconciled state not persisted: %+v", stored)
	}
	txs, _, err := txRepo.ListByPayment(ctx, tenantA, p.ID, ports.TransactionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if len(txs) != 1 || txs[0].Status != string(domain.TransactionStatusSuccess) {
		t.Fatalf("expected 1 success transaction, got %+v", txs)
	}

	// Repeated verify does not re-apply.
	if _, err := svc.VerifyPayment(ctx, actorWithPerms(tenantA, application.PermInitiate), p.ID); err != nil {
		t.Fatalf("second verify: %v", err)
	}
	txs, _, err = txRepo.ListByPayment(ctx, tenantA, p.ID, ports.TransactionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list transactions after repeated verify: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("repeated verify must not re-apply: got %d transactions", len(txs))
	}

	// Cross-tenant: tenant B cannot reconcile (or even see) tenant A's payment.
	bCtx := withTenant(context.Background(), tenantB)
	gatesB := flags.NewStaticSnapshot()
	gatesB.Set(tenantB, application.FeaturePayments, true)
	svcB := application.NewService(pRepo, txRepo, wRepo,
		application.WithFeatureGate(gatesB),
		application.WithPaymentProvider(prov),
	)
	if _, err := svcB.VerifyPayment(bCtx, actorWithPerms(tenantB, application.PermInitiate), p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for cross-tenant verify, got %v", err)
	}
}
