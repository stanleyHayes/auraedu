// Package application implements the payment service use cases.
package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead      = "payments.read"
	PermInitiate  = "payments.initiate"
	PermManage    = "payments.manage"
	PermConfigure = "payments.configure"
)

// FeaturePayments is the feature flag key for payment gateway integrations.
const FeaturePayments = "online_payments"

// Service holds the payment use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	paymentRepo              ports.PaymentRepository
	transactionRepo          ports.TransactionRepository
	webhookRepo              ports.WebhookEventRepository
	pub                      ports.EventPublisher
	provider                 ports.PaymentProvider
	gates                    flags.Gate
	paystackWebhookSecret    string
	flutterwaveWebhookSecret string
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithPaymentProvider sets the payment provider adapter.
func WithPaymentProvider(p ports.PaymentProvider) Option { return func(s *Service) { s.provider = p } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithWebhookSecrets sets the per-provider webhook signature secrets. An empty secret
// keeps dev-mode behavior: webhooks for that provider are accepted with a warning log.
func WithWebhookSecrets(paystackSecret, flutterwaveSecret string) Option {
	return func(s *Service) {
		s.paystackWebhookSecret = paystackSecret
		s.flutterwaveWebhookSecret = flutterwaveSecret
	}
}

type noopPublisher struct{}

func (noopPublisher) PublishPayment(context.Context, string, *domain.Payment, map[string]any) error {
	return nil
}

type noopProvider struct{}

func (noopProvider) Initiate(context.Context, domain.Payment) (string, string, error) {
	return "", "", errors.New("payments: no payment provider configured")
}
func (noopProvider) Verify(context.Context, string) (string, error) {
	return "", errors.New("payments: no payment provider configured")
}

// NewService constructs the application service.
func NewService(
	paymentRepo ports.PaymentRepository,
	transactionRepo ports.TransactionRepository,
	webhookRepo ports.WebhookEventRepository,
	opts ...Option,
) *Service {
	s := &Service{
		paymentRepo:     paymentRepo,
		transactionRepo: transactionRepo,
		webhookRepo:     webhookRepo,
		pub:             noopPublisher{},
		provider:        noopProvider{},
		gates:           flags.NewStaticSnapshot(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- Payment use cases ---

// CreatePaymentRequest is the input for creating a payment.
type CreatePaymentRequest struct {
	InvoiceID   string
	AmountCents int
	Currency    string
	Provider    string
	Metadata    json.RawMessage
}

// UpdatePaymentRequest is the input for patching a payment.
type UpdatePaymentRequest struct {
	AmountCents       *int
	Currency          *string
	Provider          *string
	ProviderReference *string
	Status            *string
	Metadata          json.RawMessage
	CompletedAt       *string
}

// CreatePayment validates and persists a new Payment.
func (s *Service) CreatePayment(ctx context.Context, actor auth.Actor, req CreatePaymentRequest) (*domain.Payment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	p, err := domain.NewPayment(tenantID, req.InvoiceID, req.Provider, req.Currency, req.AmountCents, req.Metadata)
	if err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Create(ctx, tenantID, p); err != nil {
		return nil, err
	}
	if err := s.pub.PublishPayment(ctx, "payment.created.v1", p, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish payment.created event", "err", err)
	}
	return p, nil
}

// ListPayments returns a tenant-scoped page of payments.
func (s *Service) ListPayments(ctx context.Context, actor auth.Actor, filter ports.PaymentFilter) ([]*domain.Payment, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.paymentRepo.List(ctx, tenantID, filter)
}

// GetPayment returns a single payment.
func (s *Service) GetPayment(ctx context.Context, actor auth.Actor, id string) (*domain.Payment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.paymentRepo.GetByID(ctx, tenantID, id)
}

// UpdatePayment patches a payment.
func (s *Service) UpdatePayment(ctx context.Context, actor auth.Actor, id string, req UpdatePaymentRequest) (*domain.Payment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	p, err := s.paymentRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	var completedAt *string
	if req.CompletedAt != nil {
		completedAt = req.CompletedAt
	}

	patch := domain.PaymentPatch{
		AmountCents:       req.AmountCents,
		Currency:          req.Currency,
		Provider:          req.Provider,
		ProviderReference: req.ProviderReference,
		Status:            req.Status,
		Metadata:          req.Metadata,
	}
	if completedAt != nil {
		t, err := parseTime(*completedAt)
		if err != nil {
			return nil, fmt.Errorf("%w: completed_at must be RFC3339", domain.ErrValidation)
		}
		patch.CompletedAt = &t
	}

	changed, err := p.ApplyUpdate(patch)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return p, nil
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Update(ctx, tenantID, p); err != nil {
		return nil, err
	}
	if err := s.pub.PublishPayment(ctx, "payment.updated.v1", p, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish payment.updated event", "err", err)
	}
	return p, nil
}

// DeletePayment removes a payment.
func (s *Service) DeletePayment(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	p, err := s.paymentRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.paymentRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.PublishPayment(ctx, "payment.deleted.v1", p, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish payment.deleted event", "err", err)
	}
	return nil
}

// InitiatePayment transitions a payment to processing, calls the provider adapter and publishes an event.
func (s *Service) InitiatePayment(ctx context.Context, actor auth.Actor, id string) (*domain.Payment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermInitiate)
	if err != nil {
		return nil, err
	}
	p, err := s.paymentRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p.Status != string(domain.PaymentStatusPending) {
		return nil, fmt.Errorf("%w: payment must be pending to initiate", domain.ErrValidation)
	}

	processing := string(domain.PaymentStatusProcessing)
	if _, err := p.ApplyUpdate(domain.PaymentPatch{Status: &processing}); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Update(ctx, tenantID, p); err != nil {
		return nil, err
	}

	ref, _, err := s.provider.Initiate(ctx, *p)
	if err != nil {
		return nil, fmt.Errorf("%w: provider initiate failed: %w", domain.ErrValidation, err)
	}
	if _, err := p.ApplyUpdate(domain.PaymentPatch{ProviderReference: &ref}); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Update(ctx, tenantID, p); err != nil {
		return nil, err
	}

	if err := s.pub.PublishPayment(ctx, "payment.initiated.v1", p, map[string]any{"provider_reference": ref}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish payment.initiated event", "err", err)
	}
	return p, nil
}

// --- Transaction use cases ---

// ListTransactionsByPayment returns transactions for a payment.
func (s *Service) ListTransactionsByPayment(
	ctx context.Context,
	actor auth.Actor,
	paymentID string,
	filter ports.TransactionFilter,
) ([]*domain.Transaction, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.transactionRepo.ListByPayment(ctx, tenantID, paymentID, filter)
}

// GetTransaction returns a single transaction.
func (s *Service) GetTransaction(ctx context.Context, actor auth.Actor, id string) (*domain.Transaction, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.transactionRepo.GetByID(ctx, tenantID, id)
}

// --- Webhook event use cases ---

// CreateWebhookEventRequest is the input for creating a webhook event record.
type CreateWebhookEventRequest struct {
	Provider  string
	EventType string
	Payload   json.RawMessage
	Signature *string
}

// CreateWebhookEvent persists a webhook event record (admin/audit use case).
func (s *Service) CreateWebhookEvent(ctx context.Context, actor auth.Actor, req CreateWebhookEventRequest) (*domain.WebhookEvent, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermConfigure)
	if err != nil {
		return nil, err
	}
	w, err := domain.NewWebhookEvent(req.Provider, req.EventType, req.Payload, req.Signature)
	if err != nil {
		return nil, err
	}
	w.SetTenant(tenantID)
	if err := s.webhookRepo.Create(ctx, tenantID, w); err != nil {
		return nil, err
	}
	return w, nil
}

// ListWebhookEvents returns a tenant-scoped page of webhook events.
func (s *Service) ListWebhookEvents(ctx context.Context, actor auth.Actor, filter ports.WebhookEventFilter) ([]*domain.WebhookEvent, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.webhookRepo.List(ctx, tenantID, filter)
}

// GetWebhookEvent returns a single webhook event.
func (s *Service) GetWebhookEvent(ctx context.Context, actor auth.Actor, id string) (*domain.WebhookEvent, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.webhookRepo.GetByID(ctx, tenantID, id)
}

// --- Webhook processing use case ---

// ProcessWebhookRequest is the input for processing a provider webhook.
type ProcessWebhookRequest struct {
	Provider  string
	Payload   json.RawMessage
	Signature string
}

// ProcessWebhook verifies the webhook signature, persists the event, reconciles the
// payment/transaction state and emits a domain event. Processing is idempotent per
// (tenant, provider, reference): a redelivered webhook is recorded for audit but never
// re-applied.
func (s *Service) ProcessWebhook(ctx context.Context, req ProcessWebhookRequest) (*domain.Payment, error) {
	if err := s.validateWebhook(req); err != nil {
		return nil, err
	}
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	if err := s.verifyWebhookSignature(ctx, req); err != nil {
		return nil, err
	}

	eventType, providerReference, tenantID, err := parseWebhookPayload(req.Payload)
	if err != nil {
		return nil, err
	}

	// Webhooks are unauthenticated: the tenant comes from the (now signature-verified)
	// payload, and Postgres RLS requires it on the session. Scope the ctx accordingly.
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})

	webhook, err := s.buildWebhookEvent(req.Provider, eventType, req.Payload, req.Signature, tenantID)
	if err != nil {
		return nil, err
	}

	payment, err := s.resolvePaymentByReference(ctx, tenantID, req.Provider, providerReference)
	if err != nil {
		return nil, err
	}

	// Idempotency guard: a processed event for this reference means the outcome was
	// already applied. Record the redelivery for audit and return current state.
	processed, err := s.webhookRepo.HasProcessedReference(ctx, tenantID, req.Provider, providerReference)
	if err != nil {
		return nil, err
	}
	if processed {
		webhook.MarkProcessed()
		if err := s.webhookRepo.Create(ctx, tenantID, webhook); err != nil {
			slog.Default().ErrorContext(ctx, "failed to record duplicate webhook delivery", "err", err)
		}
		if payment == nil {
			return nil, fmt.Errorf("%w: no matching payment for reference %q", domain.ErrNotFound, providerReference)
		}
		return payment, nil
	}

	if err := s.webhookRepo.Create(ctx, tenantID, webhook); err != nil {
		return nil, err
	}

	if payment == nil {
		webhook.MarkProcessed()
		if err := s.webhookRepo.Update(ctx, tenantID, webhook); err != nil {
			slog.Default().ErrorContext(ctx, "failed to mark orphan webhook processed", "err", err)
		}
		return nil, fmt.Errorf("%w: no matching payment for reference %q", domain.ErrNotFound, providerReference)
	}

	status, err := s.provider.Verify(ctx, providerReference)
	if err != nil {
		return nil, err
	}

	if err := s.reconcilePayment(ctx, tenantID, payment, providerReference, status); err != nil {
		return nil, err
	}

	webhook.MarkProcessed()
	if err := s.webhookRepo.Update(ctx, tenantID, webhook); err != nil {
		slog.Default().ErrorContext(ctx, "failed to mark webhook processed", "err", err)
	}
	return payment, nil
}

// VerifyPayment reconciles a payment against the provider's current status (operator
// action: GET /payments/{id}/verify). It is idempotent and emits the same transition
// events as webhook processing.
func (s *Service) VerifyPayment(ctx context.Context, actor auth.Actor, id string) (*domain.Payment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermInitiate)
	if err != nil {
		return nil, err
	}
	p, err := s.paymentRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p.ProviderReference == nil || strings.TrimSpace(*p.ProviderReference) == "" {
		return nil, fmt.Errorf("%w: payment has no provider reference; initiate it first", domain.ErrValidation)
	}
	status, err := s.provider.Verify(ctx, *p.ProviderReference)
	if err != nil {
		return nil, err
	}
	if err := s.reconcilePayment(ctx, tenantID, p, *p.ProviderReference, status); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) validateWebhook(req ProcessWebhookRequest) error {
	if strings.TrimSpace(req.Provider) == "" {
		return fmt.Errorf("%w: provider is required", domain.ErrValidation)
	}
	if len(req.Payload) == 0 {
		return fmt.Errorf("%w: payload is required", domain.ErrValidation)
	}
	if !json.Valid(req.Payload) {
		return fmt.Errorf("%w: payload must be valid JSON", domain.ErrValidation)
	}
	return nil
}

// verifyWebhookSignature authenticates the raw webhook payload per provider:
//   - paystack:    hex HMAC-SHA512 of the raw body with the secret, compared to X-Paystack-Signature
//   - flutterwave: the verif-hash header must equal the configured secret hash
//
// When no secret is configured for the provider, dev-mode behavior is kept (accept,
// log a warning). Invalid signatures are rejected with 401 via domain.ErrUnauthorized.
func (s *Service) verifyWebhookSignature(ctx context.Context, req ProcessWebhookRequest) error {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	signature := strings.TrimSpace(req.Signature)

	var secret string
	switch provider {
	case string(domain.ProviderPaystack):
		secret = s.paystackWebhookSecret
	case string(domain.ProviderFlutterwave):
		secret = s.flutterwaveWebhookSecret
	}
	if secret == "" {
		slog.Default().WarnContext(ctx, "webhook secret not configured; accepting unverified webhook (dev mode)",
			"provider", provider)
		return nil
	}

	switch provider {
	case string(domain.ProviderPaystack):
		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write(req.Payload)
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return fmt.Errorf("%w: invalid paystack webhook signature", domain.ErrUnauthorized)
		}
	case string(domain.ProviderFlutterwave):
		if subtle.ConstantTimeCompare([]byte(signature), []byte(secret)) != 1 {
			return fmt.Errorf("%w: invalid flutterwave webhook verif-hash", domain.ErrUnauthorized)
		}
	default:
		// Unknown/mock provider with a secret configured: require a non-empty signature.
		if signature == "" {
			return fmt.Errorf("%w: webhook signature required", domain.ErrUnauthorized)
		}
	}
	return nil
}

func (s *Service) buildWebhookEvent(provider, eventType string, payload json.RawMessage, signature, tenantID string) (*domain.WebhookEvent, error) {
	webhook, err := domain.NewWebhookEvent(provider, eventType, payload, nil)
	if err != nil {
		return nil, err
	}
	if signature != "" {
		sig := signature
		webhook.Signature = &sig
	}
	webhook.SetTenant(tenantID)
	return webhook, nil
}

func (s *Service) resolvePaymentByReference(ctx context.Context, tenantID, provider, reference string) (*domain.Payment, error) {
	// Idempotency: skip processing if we have already handled this provider reference.
	existing, err := s.paymentRepo.GetByProviderReference(ctx, tenantID, provider, reference)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	payment, err := s.paymentRepo.GetByID(ctx, tenantID, reference)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return payment, nil
}

// reconcilePayment applies a provider-reported outcome to a payment, recording a
// transaction and emitting payment.received.v1 / payment.failed.v1 on the transition.
// It is the shared reconciliation path for webhooks and VerifyPayment, and is
// idempotent: already-applied outcomes are no-ops, and a successful payment is never
// regressed. A failed payment MAY be corrected to success (reconciliation fix-up).
func (s *Service) reconcilePayment(ctx context.Context, tenantID string, payment *domain.Payment, providerReference, status string) error {
	success := status == string(domain.PaymentStatusSuccess)

	target := string(domain.PaymentStatusFailed)
	if success {
		target = string(domain.PaymentStatusSuccess)
	}

	// Already in the target state: redelivery or repeated verify — do not re-apply.
	if payment.Status == target {
		return nil
	}
	// Success is absorbing: a late/stale failure must not regress a received payment.
	if payment.Status == string(domain.PaymentStatusSuccess) && !success {
		slog.Default().WarnContext(ctx, "ignoring failure outcome for already successful payment",
			"payment_id", payment.ID, "provider_reference", providerReference)
		return nil
	}

	if success {
		now := nowUTC()
		if _, err := payment.ApplyUpdate(domain.PaymentPatch{Status: &target, CompletedAt: &now}); err != nil {
			return err
		}
		if err := s.paymentRepo.Update(ctx, tenantID, payment); err != nil {
			return err
		}
		if err := s.recordTransaction(
			ctx, tenantID, payment.ID,
			string(domain.TransactionTypeCredit), string(domain.TransactionStatusSuccess),
			providerReference, payment.AmountCents,
		); err != nil {
			return err
		}
		if err := s.pub.PublishPayment(ctx, "payment.received.v1", payment, map[string]any{"provider_reference": providerReference}); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish payment.received event", "err", err)
		}
		return nil
	}

	if _, err := payment.ApplyUpdate(domain.PaymentPatch{Status: &target}); err != nil {
		return err
	}
	if err := s.paymentRepo.Update(ctx, tenantID, payment); err != nil {
		return err
	}
	if err := s.recordTransaction(
		ctx, tenantID, payment.ID,
		string(domain.TransactionTypeDebit), string(domain.TransactionStatusFailed),
		providerReference, payment.AmountCents,
	); err != nil {
		return err
	}
	if err := s.pub.PublishPayment(ctx, "payment.failed.v1", payment, map[string]any{
		"provider_reference": providerReference,
		"reason":             "provider reported status " + status,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish payment.failed event", "err", err)
	}
	return nil
}

func (s *Service) recordTransaction(ctx context.Context, tenantID, paymentID, txType, status, reference string, amountCents int) error {
	tx, err := domain.NewTransaction(tenantID, paymentID, txType, status, reference, amountCents)
	if err != nil {
		return err
	}
	return s.transactionRepo.Create(ctx, tenantID, tx)
}

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) {
		return "", domain.ErrForbidden
	}
	if !actor.Has(perm) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeaturePayments) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeaturePayments)
	}
	return tenantID, nil
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

func parseTime(v string) (time.Time, error) {
	return time.Parse(time.RFC3339, v)
}

// parseWebhookPayload extracts the event type, provider reference and tenant from a
// provider webhook body. It understands the generic shape ({reference, tenant_id}),
// the Paystack shape ({event, data: {reference, metadata: {tenant_id}}}) and the
// Flutterwave charge shape ({event, data: {tx_ref}}).
func parseWebhookPayload(payload json.RawMessage) (eventType, providerReference, tenantID string, err error) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", "", "", fmt.Errorf("%w: payload must be valid JSON", domain.ErrValidation)
	}
	eventType = stringField(data, "event")
	if eventType == "" {
		eventType = stringField(data, "event_type")
	}
	if ref, ok := data["reference"].(string); ok && ref != "" {
		providerReference = ref
	} else if ref, ok := data["provider_reference"].(string); ok && ref != "" {
		providerReference = ref
	} else if nested, ok := data["data"].(map[string]any); ok {
		if ref, ok := nested["reference"].(string); ok && ref != "" {
			providerReference = ref
		} else if ref, ok := nested["tx_ref"].(string); ok && ref != "" {
			providerReference = ref
		}
	}
	if tenant, ok := data["tenant_id"].(string); ok && tenant != "" {
		tenantID = tenant
	} else if tenant, ok := data["tenant"].(string); ok && tenant != "" {
		tenantID = tenant
	} else if nested, ok := data["data"].(map[string]any); ok {
		// Paystack carries our metadata bag on data.metadata.
		if meta, ok := nested["metadata"].(map[string]any); ok {
			if tenant, ok := meta["tenant_id"].(string); ok && tenant != "" {
				tenantID = tenant
			}
		}
	}
	if tenantID == "" {
		if meta, ok := data["metadata"].(map[string]any); ok {
			if tenant, ok := meta["tenant_id"].(string); ok && tenant != "" {
				tenantID = tenant
			}
		}
	}
	if eventType == "" {
		eventType = "charge.success"
	}
	if providerReference == "" {
		return "", "", "", fmt.Errorf("%w: provider reference not found in payload", domain.ErrValidation)
	}
	if tenantID == "" {
		return "", "", "", fmt.Errorf("%w: tenant_id not found in payload", domain.ErrValidation)
	}
	return eventType, providerReference, tenantID, nil
}

func nowUTC() time.Time { return time.Now().UTC() }

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
