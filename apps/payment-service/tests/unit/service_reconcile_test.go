package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func newDevService(pRepo *fakePaymentRepo, txRepo *fakeTxRepo, wRepo *fakeWebhookRepo, pub *fakePublisher, prov *stubProvider) *application.Service {
	return application.NewService(pRepo, txRepo, wRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(enabledGates()),
	)
}

func webhookPayload(tenantID string) []byte {
	return []byte(`{"reference":"ref-1","tenant_id":"` + tenantID + `"}`)
}

func TestProcessWebhook_DuplicateDeliveryAppliesOnce(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	p := seedProcessingPayment(t, svc)

	req := application.ProcessWebhookRequest{Provider: "paystack", Payload: webhookPayload(unitTenantA)}
	if _, err := svc.ProcessWebhook(context.Background(), req); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	got, err := svc.ProcessWebhook(context.Background(), req)
	if err != nil {
		t.Fatalf("duplicate delivery: %v", err)
	}

	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got.Status)
	}
	if txRepo.count() != 1 {
		t.Fatalf("duplicate webhook must not double-apply: expected 1 transaction, got %d", txRepo.count())
	}
	if pub.count("payment.received.v1") != 1 {
		t.Fatalf("expected exactly 1 payment.received.v1, got %d", pub.count("payment.received.v1"))
	}
	// Both deliveries are recorded for audit; both end up processed.
	if len(wRepo.events) != 2 {
		t.Fatalf("expected 2 webhook event records, got %d", len(wRepo.events))
	}
	for _, w := range wRepo.events {
		if !w.Processed {
			t.Fatal("all webhook deliveries should be marked processed")
		}
	}
	_ = p
}

func TestProcessWebhook_LateFailureDoesNotRegressSuccess(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	seedProcessingPayment(t, svc)

	if _, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider: "paystack", Payload: webhookPayload(unitTenantA),
	}); err != nil {
		t.Fatalf("first webhook: %v", err)
	}

	// A second, different event for the same reference (provider now reporting failure)
	// must not regress the successful payment.
	prov.verifyStatus = string(domain.PaymentStatusFailed)
	got, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider: "paystack", Payload: webhookPayload(unitTenantA),
	})
	if err != nil {
		t.Fatalf("late failure webhook: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("success must be absorbing, got %q", got.Status)
	}
	if pub.count("payment.failed.v1") != 0 {
		t.Fatal("no payment.failed.v1 expected for an already successful payment")
	}
}

func TestVerifyPayment_ReconcilesAndIsIdempotent(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	p := seedProcessingPayment(t, svc)

	ctx := tenantCtx(unitTenantA)
	actor := unitActor(unitTenantA, application.PermInitiate)

	got, err := svc.VerifyPayment(ctx, actor, p.ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("expected completed_at set on success")
	}
	if txRepo.count() != 1 {
		t.Fatalf("expected 1 transaction, got %d", txRepo.count())
	}
	if pub.count("payment.received.v1") != 1 {
		t.Fatalf("expected 1 payment.received.v1, got %d", pub.count("payment.received.v1"))
	}
	ev := pub.events[len(pub.events)-1]
	if ev.meta["provider_reference"] != "ref-1" {
		t.Fatalf("expected provider_reference meta, got %v", ev.meta)
	}

	// Repeated verify is a no-op: no extra transaction, no extra event.
	got, err = svc.VerifyPayment(ctx, actor, p.ID)
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got.Status)
	}
	if prov.verifyCalls != 2 {
		t.Fatalf("expected provider verify called twice, got %d", prov.verifyCalls)
	}
	if txRepo.count() != 1 || pub.count("payment.received.v1") != 1 {
		t.Fatal("repeated verify must not re-apply the outcome")
	}
}

func TestVerifyPayment_FailedThenCorrectedToSuccess(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusFailed)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	p := seedProcessingPayment(t, svc)

	// Webhook reports failure first.
	if _, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider: "paystack", Payload: webhookPayload(unitTenantA),
	}); err != nil {
		t.Fatalf("failure webhook: %v", err)
	}
	if pub.count("payment.failed.v1") != 1 {
		t.Fatalf("expected 1 payment.failed.v1, got %d", pub.count("payment.failed.v1"))
	}
	ev := pub.events[len(pub.events)-1]
	if reason, ok := ev.meta["reason"].(string); !ok || reason == "" {
		t.Fatalf("payment.failed.v1 should carry a reason, got %v", ev.meta)
	}

	// Provider now says the charge actually succeeded: verify corrects the state.
	prov.verifyStatus = string(domain.PaymentStatusSuccess)
	got, err := svc.VerifyPayment(tenantCtx(unitTenantA), unitActor(unitTenantA, application.PermInitiate), p.ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected correction to success, got %q", got.Status)
	}
	if pub.count("payment.received.v1") != 1 {
		t.Fatal("correction is a transition and must emit payment.received.v1")
	}
}

func TestVerifyPayment_FailureTransition(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusFailed)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	p := seedProcessingPayment(t, svc)

	got, err := svc.VerifyPayment(tenantCtx(unitTenantA), unitActor(unitTenantA, application.PermInitiate), p.ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != string(domain.PaymentStatusFailed) {
		t.Fatalf("expected failed, got %q", got.Status)
	}
	if txRepo.count() != 1 {
		t.Fatalf("expected 1 failed transaction, got %d", txRepo.count())
	}
	if pub.count("payment.failed.v1") != 1 {
		t.Fatalf("expected 1 payment.failed.v1, got %d", pub.count("payment.failed.v1"))
	}
}

func TestVerifyPayment_RequiresInitiation(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)

	ctx := tenantCtx(unitTenantA)
	p, err := svc.CreatePayment(ctx, unitActor(unitTenantA, application.PermManage), application.CreatePaymentRequest{
		InvoiceID: unitInvoice, AmountCents: 10000, Currency: "GHS", Provider: "paystack",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := svc.VerifyPayment(ctx, unitActor(unitTenantA, application.PermInitiate), p.ID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for un-initiated payment, got %v", err)
	}
}

func TestVerifyPayment_TenantScoping(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	gates := flags.NewStaticSnapshot()
	gates.Set(unitTenantA, application.FeaturePayments, true)
	gates.Set(unitTenantB, application.FeaturePayments, true)
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(gates),
	)
	p := seedProcessingPayment(t, svc)

	// A tenant-B actor (valid in its own tenant) cannot see tenant A's payment:
	// the tenant-scoped lookup finds nothing and the provider is never called.
	_, err := svc.VerifyPayment(tenantCtx(unitTenantB), unitActor(unitTenantB, application.PermInitiate), p.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for cross-tenant payment id, got %v", err)
	}
	if prov.verifyCalls != 0 {
		t.Fatal("provider must not be called for a rejected request")
	}

	// A tenant-A actor presenting a tenant-B context is rejected outright.
	_, err = svc.VerifyPayment(tenantCtx(unitTenantB), unitActor(unitTenantA, application.PermInitiate), p.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for actor/tenant mismatch, got %v", err)
	}
}

func TestVerifyPayment_RequiresPermission(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newDevService(pRepo, txRepo, wRepo, pub, prov)
	p := seedProcessingPayment(t, svc)

	_, err := svc.VerifyPayment(tenantCtx(unitTenantA), unitActor(unitTenantA, application.PermRead), p.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden without payments.initiate, got %v", err)
	}
}

func TestProcessWebhook_TenantIsolation(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	gates := flags.NewStaticSnapshot()
	gates.Set(unitTenantA, application.FeaturePayments, true)
	gates.Set(unitTenantB, application.FeaturePayments, true)
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(gates),
	)
	p := seedProcessingPayment(t, svc)

	// A webhook claiming tenant B for a reference owned by tenant A finds nothing and
	// must not touch tenant A's payment.
	_, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider: "paystack", Payload: webhookPayload(unitTenantB),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found for cross-tenant reference, got %v", err)
	}
	payment, err := pRepo.GetByID(tenantCtx(unitTenantA), unitTenantA, p.ID)
	if err != nil {
		t.Fatalf("lookup payment: %v", err)
	}
	if payment.Status != string(domain.PaymentStatusProcessing) {
		t.Fatalf("tenant A payment must stay processing, got %q", payment.Status)
	}
	if txRepo.count() != 0 {
		t.Fatal("cross-tenant webhook must not create transactions")
	}
}
