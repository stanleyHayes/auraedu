package unit

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

// --- Fakes for application-service unit tests (no database required) ---

const (
	unitTenantA = "11111111-1111-1111-1111-111111111111"
	unitTenantB = "22222222-2222-2222-2222-222222222222"
	unitInvoice = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

type fakePaymentRepo struct {
	mu      sync.Mutex
	byID    map[string]*domain.Payment
	byRef   map[string]string // tenant|provider|reference -> tenant|id
	updates int
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{byID: map[string]*domain.Payment{}, byRef: map[string]string{}}
}

func payKey(tenant, id string) string { return tenant + "|" + id }
func refKey(tenant, provider, ref string) string {
	return tenant + "|" + provider + "|" + ref
}

func (r *fakePaymentRepo) Create(_ context.Context, tenantID string, p *domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[payKey(tenantID, p.ID)] = p
	if p.ProviderReference != nil {
		r.byRef[refKey(tenantID, p.Provider, *p.ProviderReference)] = payKey(tenantID, p.ID)
	}
	return nil
}

func (r *fakePaymentRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byID[payKey(tenantID, id)]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakePaymentRepo) GetByProviderReference(_ context.Context, tenantID, provider, reference string) (*domain.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key, ok := r.byRef[refKey(tenantID, provider, reference)]; ok {
		return r.byID[key], nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakePaymentRepo) List(_ context.Context, tenantID string, _ ports.PaymentFilter) ([]*domain.Payment, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Payment
	for _, p := range r.byID {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, "", nil
}

func (r *fakePaymentRepo) Update(_ context.Context, tenantID string, p *domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[payKey(tenantID, p.ID)]; !ok {
		return domain.ErrNotFound
	}
	r.byID[payKey(tenantID, p.ID)] = p
	if p.ProviderReference != nil {
		r.byRef[refKey(tenantID, p.Provider, *p.ProviderReference)] = payKey(tenantID, p.ID)
	}
	r.updates++
	return nil
}

func (r *fakePaymentRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, payKey(tenantID, id))
	return nil
}

type fakeTxRepo struct {
	mu  sync.Mutex
	txs []*domain.Transaction
}

func (r *fakeTxRepo) Create(_ context.Context, _ string, t *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.txs = append(r.txs, t)
	return nil
}

func (r *fakeTxRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.txs {
		if t.ID == id && t.TenantID == tenantID {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeTxRepo) ListByPayment(_ context.Context, _, paymentID string, _ ports.TransactionFilter) ([]*domain.Transaction, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Transaction
	for _, t := range r.txs {
		if t.PaymentID == paymentID {
			out = append(out, t)
		}
	}
	return out, "", nil
}

func (r *fakeTxRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.txs)
}

type fakeWebhookRepo struct {
	mu     sync.Mutex
	events []*domain.WebhookEvent
}

func (r *fakeWebhookRepo) Create(_ context.Context, _ string, w *domain.WebhookEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, w)
	return nil
}

func (r *fakeWebhookRepo) GetByID(_ context.Context, tenantID, id string) (*domain.WebhookEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.events {
		if w.ID == id && w.TenantID == tenantID {
			return w, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeWebhookRepo) Update(_ context.Context, tenantID string, w *domain.WebhookEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.events {
		if e.ID == w.ID && e.TenantID == tenantID {
			r.events[i] = w
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *fakeWebhookRepo) List(_ context.Context, tenantID string, _ ports.WebhookEventFilter) ([]*domain.WebhookEvent, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.WebhookEvent
	for _, w := range r.events {
		if w.TenantID == tenantID {
			out = append(out, w)
		}
	}
	return out, "", nil
}

// HasProcessedReference mirrors the Postgres implementation: payload reference is
// extracted from the same locations parseWebhookPayload reads.
func (r *fakeWebhookRepo) HasProcessedReference(_ context.Context, tenantID, provider, reference string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.events {
		if w.TenantID == tenantID && w.Provider == provider && w.Processed && payloadReference(w.Payload) == reference {
			return true, nil
		}
	}
	return false, nil
}

func payloadReference(payload json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return ""
	}
	if v, ok := m["reference"].(string); ok {
		return v
	}
	if v, ok := m["provider_reference"].(string); ok {
		return v
	}
	if d, ok := m["data"].(map[string]any); ok {
		if v, ok := d["reference"].(string); ok {
			return v
		}
	}
	return ""
}

type publishedEvent struct {
	eventType string
	payment   domain.Payment
	meta      map[string]any
}

type fakePublisher struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (f *fakePublisher) PublishPayment(_ context.Context, eventType string, p *domain.Payment, meta map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, publishedEvent{eventType: eventType, payment: *p, meta: meta})
	return nil
}

func (f *fakePublisher) count(eventType string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.events {
		if e.eventType == eventType {
			n++
		}
	}
	return n
}

type stubProvider struct {
	ref          string
	verifyStatus string
	verifyCalls  int
}

func (s *stubProvider) Initiate(_ context.Context, p domain.Payment) (string, string, error) {
	return s.ref, "https://checkout.test/" + p.ID, nil
}

func (s *stubProvider) Verify(_ context.Context, _ string) (string, error) {
	s.verifyCalls++
	return s.verifyStatus, nil
}

func unitActor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func enabledGates(tenantID string, featureKey string) flags.Gate {
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantID, featureKey, true)
	return gates
}
