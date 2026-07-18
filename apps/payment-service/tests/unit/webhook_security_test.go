package unit

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	svchttp "github.com/auraedu/payment-service/internal/adapters/http"
	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

const (
	paystackSecret    = "sk_test_webhook_secret"
	flutterwaveSecret = "flw_verif_hash_secret"
)

func newSecureService(pRepo *fakePaymentRepo, txRepo *fakeTxRepo, wRepo *fakeWebhookRepo, pub *fakePublisher, prov *stubProvider) *application.Service {
	return application.NewService(pRepo, txRepo, wRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(enabledGates(unitTenantA, application.FeaturePayments)),
		application.WithWebhookSecrets(paystackSecret, flutterwaveSecret),
	)
}

// seedProcessingPayment creates + initiates a payment so provider reference ref exists.
func seedProcessingPayment(t *testing.T, svc *application.Service, tenantID string) *domain.Payment {
	t.Helper()
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
	p, err := svc.CreatePayment(ctx, unitActor(tenantID, application.PermManage), application.CreatePaymentRequest{
		InvoiceID:   unitInvoice,
		AmountCents: 10000,
		Currency:    "GHS",
		Provider:    "paystack",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	p, err = svc.InitiatePayment(ctx, unitActor(tenantID, application.PermInitiate), p.ID)
	if err != nil {
		t.Fatalf("initiate payment: %v", err)
	}
	return p
}

func paystackSignature(secret string, payload []byte) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestProcessWebhook_PaystackValidSignatureAccepted(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newSecureService(pRepo, txRepo, wRepo, pub, prov)
	seedProcessingPayment(t, svc, unitTenantA)

	payload := []byte(`{"event":"charge.success","data":{"reference":"ref-1","metadata":{"tenant_id":"` + unitTenantA + `"}}}`)
	_, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "paystack",
		Payload:   payload,
		Signature: paystackSignature(paystackSecret, payload),
	})
	if err != nil {
		t.Fatalf("expected valid signature to be accepted, got %v", err)
	}
}

func TestProcessWebhook_PaystackInvalidSignatureRejected401(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newSecureService(pRepo, txRepo, wRepo, pub, prov)

	payload := []byte(`{"reference":"ref-1","tenant_id":"` + unitTenantA + `"}`)
	_, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "paystack",
		Payload:   payload,
		Signature: paystackSignature("wrong-secret", payload),
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	// Nothing must have been persisted or published.
	if len(wRepo.events) != 0 || txRepo.count() != 0 || pub.count("payment.received.v1") != 0 {
		t.Fatal("rejected webhook must not mutate state")
	}
}

func TestProcessWebhook_PaystackMissingSignatureRejected(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newSecureService(pRepo, txRepo, wRepo, pub, prov)

	payload := []byte(`{"reference":"ref-1","tenant_id":"` + unitTenantA + `"}`)
	_, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "paystack",
		Payload:   payload,
		Signature: "",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestProcessWebhook_NoSecretConfigured_DevModeAccepts(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	// No WithWebhookSecrets: dev mode.
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(enabledGates(unitTenantA, application.FeaturePayments)),
	)
	seedProcessingPayment(t, svc, unitTenantA)

	payload := []byte(`{"reference":"ref-1","tenant_id":"` + unitTenantA + `"}`)
	if _, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "paystack",
		Payload:   payload,
		Signature: "",
	}); err != nil {
		t.Fatalf("dev mode should accept unsigned webhook, got %v", err)
	}
}

func TestProcessWebhook_FlutterwaveVerifHash(t *testing.T) {
	build := func() *application.Service {
		pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
		prov := &stubProvider{ref: "tx-ref-9", verifyStatus: string(domain.PaymentStatusSuccess)}
		svc := newSecureService(pRepo, txRepo, wRepo, pub, prov)
		// Seed payment under the flutterwave provider name.
		ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: unitTenantA})
		p, err := svc.CreatePayment(ctx, unitActor(unitTenantA, application.PermManage), application.CreatePaymentRequest{
			InvoiceID:   unitInvoice,
			AmountCents: 5000,
			Currency:    "GHS",
			Provider:    "flutterwave",
		})
		if err != nil {
			t.Fatalf("create payment: %v", err)
		}
		if _, err := svc.InitiatePayment(ctx, unitActor(unitTenantA, application.PermInitiate), p.ID); err != nil {
			t.Fatalf("initiate payment: %v", err)
		}
		return svc
	}

	payload := []byte(`{"event":"charge.completed","data":{"tx_ref":"tx-ref-9"},"tenant_id":"` + unitTenantA + `"}`)

	svc := build()
	if _, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "flutterwave",
		Payload:   payload,
		Signature: flutterwaveSecret,
	}); err != nil {
		t.Fatalf("valid verif-hash should be accepted, got %v", err)
	}

	svc = build()
	_, err := svc.ProcessWebhook(context.Background(), application.ProcessWebhookRequest{
		Provider:  "flutterwave",
		Payload:   payload,
		Signature: "attacker-controlled-hash",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// TestWebhookHandler_InvalidSignatureReturns401 exercises the full HTTP path:
// raw body → HMAC verify → 401 before any processing.
func TestWebhookHandler_InvalidSignatureReturns401(t *testing.T) {
	pRepo, txRepo, wRepo, pub := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, &fakePublisher{}
	prov := &stubProvider{ref: "ref-1", verifyStatus: string(domain.PaymentStatusSuccess)}
	svc := newSecureService(pRepo, txRepo, wRepo, pub, prov)

	mux := http.NewServeMux()
	svchttp.NewHandler(svc).Register(mux)

	payload := `{"reference":"ref-1","tenant_id":"` + unitTenantA + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/paystack", strings.NewReader(payload))
	req.Header.Set("X-Paystack-Signature", "deadbeef")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", rec.Code, rec.Body.String())
	}

	// Same body with the correct signature passes verification (404 here because no
	// payment matches — proof the request got past the signature check).
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/paystack", strings.NewReader(payload))
	req.Header.Set("X-Paystack-Signature", paystackSignature(paystackSecret, []byte(payload)))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("valid signature must not 401: %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown reference, got %d (%s)", rec.Code, rec.Body.String())
	}
}
