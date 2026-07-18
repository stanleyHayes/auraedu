package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
)

// DefaultPaystackBaseURL is the production Paystack API endpoint.
const DefaultPaystackBaseURL = "https://api.paystack.co"

// defaultPaystackTimeout bounds each Paystack API call.
const defaultPaystackTimeout = 10 * time.Second

// maxPaystackResponseBytes caps response bodies so a misbehaving endpoint cannot exhaust memory.
const maxPaystackResponseBytes = 4 << 20

// PaystackConfig configures the Paystack provider adapter.
type PaystackConfig struct {
	// SecretKey is the Paystack secret key (required). It is only ever sent in the
	// Authorization header and is never logged or included in errors.
	SecretKey string
	// BaseURL overrides the API endpoint (defaults to DefaultPaystackBaseURL).
	BaseURL string
	// Timeout bounds each API call (defaults to 10s). Ignored when HTTPClient is set.
	Timeout time.Duration
	// HTTPClient optionally injects a custom client (tests).
	HTTPClient *http.Client
}

// PaystackProvider is a Paystack-backed implementation of ports.PaymentProvider.
type PaystackProvider struct {
	secretKey string
	baseURL   string
	client    *http.Client
}

// Error is a structured Paystack API failure. It never carries the secret key.
type Error struct {
	Op         string // "initialize" or "verify"
	StatusCode int    // HTTP status, 0 for transport-level failures
	Message    string
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("paystack: %s failed: http %d: %s", e.Op, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("paystack: %s failed: %s", e.Op, e.Message)
}

// NewPaystackProvider constructs the adapter, validating configuration.
func NewPaystackProvider(cfg PaystackConfig) (*PaystackProvider, error) {
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("paystack: secret key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultPaystackBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultPaystackTimeout
		}
		client = &http.Client{Timeout: timeout}
	}
	return &PaystackProvider{secretKey: cfg.SecretKey, baseURL: baseURL, client: client}, nil
}

type paystackEnvelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type initializeRequest struct {
	Amount    int               `json:"amount"` // subunits (pesewas/kobo) — matches our amount_cents
	Currency  string            `json:"currency"`
	Reference string            `json:"reference"`
	Email     string            `json:"email,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type initializeData struct {
	AuthorizationURL string `json:"authorization_url"`
	AccessCode       string `json:"access_code"`
	Reference        string `json:"reference"`
}

type verifyData struct {
	Status    string `json:"status"`
	Reference string `json:"reference"`
}

// Initiate starts a Paystack transaction (POST /transaction/initialize) and returns
// the provider reference and checkout (authorization) URL. The payment ID is used as
// the Paystack reference so webhook and verify reconciliation are deterministic, and
// tenant/invoice IDs ride in metadata for webhook resolution.
func (p *PaystackProvider) Initiate(ctx context.Context, payment domain.Payment) (string, string, error) {
	body := initializeRequest{
		Amount:    payment.AmountCents,
		Currency:  payment.Currency,
		Reference: payment.ID,
		Email:     metadataField(payment.Metadata, "email"),
		Metadata: map[string]string{
			"payment_id": payment.ID,
			"tenant_id":  payment.TenantID,
			"invoice_id": payment.InvoiceID,
		},
	}
	var data initializeData
	if err := p.do(ctx, "initialize", http.MethodPost, "/transaction/initialize", body, &data); err != nil {
		return "", "", err
	}
	if data.Reference == "" {
		return "", "", &Error{Op: "initialize", Message: "response missing reference"}
	}
	return data.Reference, data.AuthorizationURL, nil
}

// Verify checks the final status of a transaction (GET /transaction/verify/:reference).
// Paystack "success" maps to the domain success status; every other terminal or
// intermediate state (failed, abandoned, reversed, ongoing) maps to failed, matching
// the binary reconciliation semantics of ports.PaymentProvider.
func (p *PaystackProvider) Verify(ctx context.Context, reference string) (string, error) {
	if strings.TrimSpace(reference) == "" {
		return "", &Error{Op: "verify", Message: "reference is required"}
	}
	var data verifyData
	path := "/transaction/verify/" + strings.TrimSpace(reference)
	if err := p.do(ctx, "verify", http.MethodGet, path, nil, &data); err != nil {
		return "", err
	}
	if data.Status == "success" {
		return string(domain.PaymentStatusSuccess), nil
	}
	return string(domain.PaymentStatusFailed), nil
}

// do executes one API call: encodes the request body, authenticates, enforces the
// response contract and maps failures to *Error. The secret key only appears in the
// Authorization header and is never copied into errors.
func (p *PaystackProvider) do(ctx context.Context, op, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("paystack: %s: encode request: %w", op, err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, rdr)
	if err != nil {
		return fmt.Errorf("paystack: %s: build request: %w", op, err)
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return &Error{Op: op, Message: "transport error: " + err.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPaystackResponseBytes))
	if err != nil {
		return &Error{Op: op, StatusCode: resp.StatusCode, Message: "read response: " + err.Error()}
	}

	var env paystackEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return &Error{Op: op, StatusCode: resp.StatusCode, Message: "invalid JSON response"}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := env.Message
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return &Error{Op: op, StatusCode: resp.StatusCode, Message: msg}
	}
	if !env.Status {
		msg := env.Message
		if msg == "" {
			msg = "paystack reported failure"
		}
		return &Error{Op: op, StatusCode: resp.StatusCode, Message: msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &Error{Op: op, StatusCode: resp.StatusCode, Message: "decode response data: " + err.Error()}
		}
	}
	return nil
}

// metadataField extracts a single string field from a payment metadata document.
func metadataField(metadata json.RawMessage, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(metadata, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
