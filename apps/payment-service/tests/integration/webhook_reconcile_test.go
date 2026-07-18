package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/flags"
)

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

	ok, err := wRepo.HasProcessedReference(ctx, tenantA, "paystack", "ref-db-1")
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
	txs, _, _ = txRepo.ListByPayment(ctx, tenantA, p.ID, ports.TransactionFilter{Limit: 10})
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
