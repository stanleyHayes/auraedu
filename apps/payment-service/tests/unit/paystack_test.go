package unit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/payment-service/internal/adapters/provider"
	"github.com/auraedu/payment-service/internal/domain"
)

const testSecretKey = "unit-test-paystack-key"

func newTestPayment(t *testing.T) domain.Payment {
	t.Helper()
	p, err := domain.NewPayment("11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"paystack", "GHS", 10000, json.RawMessage(`{"email":"parent@example.com"}`))
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	return *p
}

func TestNewPaystackProvider_RequiresSecret(t *testing.T) {
	if _, err := provider.NewPaystackProvider(provider.PaystackConfig{}); err == nil {
		t.Fatal("expected error when secret key is empty")
	}
}

func TestPaystackProvider_Initiate_MapsRequestAndResponse(t *testing.T) {
	payment := newTestPayment(t)

	var gotAuth, gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": true,
			"message": "Authorization URL created",
			"data": {
				"authorization_url": "https://checkout.paystack.com/abc123",
				"access_code": "ac_123",
				"reference": "ref-xyz-001"
			}
		}`))
	}))
	defer srv.Close()

	p, err := provider.NewPaystackProvider(provider.PaystackConfig{SecretKey: testSecretKey, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ref, checkoutURL, err := p.Initiate(context.Background(), payment)
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	if ref != "ref-xyz-001" {
		t.Fatalf("expected reference ref-xyz-001, got %q", ref)
	}
	if checkoutURL != "https://checkout.paystack.com/abc123" {
		t.Fatalf("unexpected checkout URL %q", checkoutURL)
	}
	if gotMethod != http.MethodPost || gotPath != "/transaction/initialize" {
		t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer "+testSecretKey {
		t.Fatalf("unexpected authorization header %q", gotAuth)
	}
	if gotBody["amount"] != float64(10000) {
		t.Fatalf("expected amount 10000 subunits, got %v", gotBody["amount"])
	}
	if gotBody["currency"] != "GHS" {
		t.Fatalf("expected currency GHS, got %v", gotBody["currency"])
	}
	if gotBody["reference"] != payment.ID {
		t.Fatalf("expected payment ID as reference, got %v", gotBody["reference"])
	}
	if gotBody["email"] != "parent@example.com" {
		t.Fatalf("expected payer email from metadata, got %v", gotBody["email"])
	}
	meta, ok := gotBody["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata object, got %v", gotBody["metadata"])
	}
	if meta["tenant_id"] != payment.TenantID || meta["invoice_id"] != payment.InvoiceID || meta["payment_id"] != payment.ID {
		t.Fatalf("metadata missing reconciliation ids: %v", meta)
	}
}

func TestPaystackProvider_Initiate_APIFailureIsStructured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status": false, "message": "Invalid secret key"}`))
	}))
	defer srv.Close()

	p, err := provider.NewPaystackProvider(provider.PaystackConfig{SecretKey: testSecretKey, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, _, err = p.Initiate(context.Background(), newTestPayment(t))
	if err == nil {
		t.Fatal("expected error")
	}
	var perr *provider.Error
	if !errors.As(err, &perr) {
		t.Fatalf("expected *provider.Error, got %T: %v", err, err)
	}
	if perr.Op != "initialize" || perr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected structured error: %+v", perr)
	}
	if !strings.Contains(perr.Message, "Invalid secret key") {
		t.Fatalf("expected provider message, got %q", perr.Message)
	}
	if strings.Contains(err.Error(), testSecretKey) {
		t.Fatal("error must not leak the secret key")
	}
}

func TestPaystackProvider_Initiate_TransportError(t *testing.T) {
	p, err := provider.NewPaystackProvider(provider.PaystackConfig{
		SecretKey: testSecretKey,
		BaseURL:   "http://127.0.0.1:1", // nothing listening
		Timeout:   500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, _, err = p.Initiate(context.Background(), newTestPayment(t))
	var perr *provider.Error
	if !errors.As(err, &perr) {
		t.Fatalf("expected *provider.Error, got %T: %v", err, err)
	}
	if perr.StatusCode != 0 {
		t.Fatalf("transport error should have status 0, got %d", perr.StatusCode)
	}
	if strings.Contains(err.Error(), testSecretKey) {
		t.Fatal("error must not leak the secret key")
	}
}

func TestPaystackProvider_Verify_MapsStatus(t *testing.T) {
	cases := []struct {
		paystackStatus string
		want           string
	}{
		{"success", string(domain.PaymentStatusSuccess)},
		{"failed", string(domain.PaymentStatusFailed)},
		{"abandoned", string(domain.PaymentStatusFailed)},
		{"reversed", string(domain.PaymentStatusFailed)},
	}
	for _, tc := range cases {
		t.Run(tc.paystackStatus, func(t *testing.T) {
			var gotPath, gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status":  true,
					"message": "Verification successful",
					"data":    map[string]any{"status": tc.paystackStatus, "reference": "ref-1"},
				})
			}))
			defer srv.Close()

			p, err := provider.NewPaystackProvider(provider.PaystackConfig{SecretKey: testSecretKey, BaseURL: srv.URL})
			if err != nil {
				t.Fatalf("new provider: %v", err)
			}
			got, err := p.Verify(context.Background(), "ref-1")
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if got != tc.want {
				t.Fatalf("paystack %q: expected %q, got %q", tc.paystackStatus, tc.want, got)
			}
			if gotMethod != http.MethodGet || gotPath != "/transaction/verify/ref-1" {
				t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
			}
		})
	}
}

func TestPaystackProvider_Verify_RequiresReference(t *testing.T) {
	p, err := provider.NewPaystackProvider(provider.PaystackConfig{SecretKey: testSecretKey, BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := p.Verify(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty reference")
	}
}

func TestMockProvider_Untouched(t *testing.T) {
	m := provider.NewMockProvider()
	ref, url, err := m.Initiate(context.Background(), newTestPayment(t))
	if err != nil || ref == "" || url == "" {
		t.Fatalf("mock initiate: ref=%q url=%q err=%v", ref, url, err)
	}
	if got, _ := m.Verify(context.Background(), "ok_success_1"); got != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success, got %q", got)
	}
	if got, _ := m.Verify(context.Background(), "nope"); got != string(domain.PaymentStatusFailed) {
		t.Fatalf("expected failed, got %q", got)
	}
}
