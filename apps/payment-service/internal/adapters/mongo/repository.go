package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const PaymentCollection = "payments"
const WebhookCollection = "webhook_events"

type PaymentRepository struct {
	payments *pmongo.Collection
	outbox   *pmongo.ClaimableOutbox
}
type TransactionRepository struct{ payments *pmongo.Collection }
type WebhookEventRepository struct{ webhooks *pmongo.Collection }

var (
	_ ports.PaymentRepository        = (*PaymentRepository)(nil)
	_ ports.ReconciliationRepository = (*PaymentRepository)(nil)
	_ ports.LifecycleRepository      = (*PaymentRepository)(nil)
	_ ports.OutboxRepository         = (*PaymentRepository)(nil)
	_ ports.TransactionRepository    = (*TransactionRepository)(nil)
	_ ports.WebhookEventRepository   = (*WebhookEventRepository)(nil)
)

func NewPaymentRepository(s *pmongo.Store) *PaymentRepository {
	c := s.Collection(PaymentCollection)
	return &PaymentRepository{c, pmongo.NewClaimableOutbox(c, time.Minute)}
}
func NewTransactionRepository(s *pmongo.Store) *TransactionRepository {
	return &TransactionRepository{s.Collection(PaymentCollection)}
}
func NewWebhookEventRepository(s *pmongo.Store) *WebhookEventRepository {
	return &WebhookEventRepository{s.Collection(WebhookCollection)}
}
func (r *PaymentRepository) Create(ctx context.Context, t string, p *domain.Payment) error {
	return insert(ctx, r.payments, t, p.ID, p)
}
func (r *PaymentRepository) GetByID(ctx context.Context, t, id string) (*domain.Payment, error) {
	return get[domain.Payment](ctx, r.payments, t, bson.M{"_id": id})
}
func (r *PaymentRepository) GetByProviderReference(ctx context.Context, t, provider, reference string) (*domain.Payment, error) {
	return get[domain.Payment](ctx, r.payments, t, bson.M{"data.provider": provider, "data.providerreference": reference})
}
func (r *PaymentRepository) List(ctx context.Context, t string, f ports.PaymentFilter) ([]*domain.Payment, string, error) {
	q := bson.M{}
	equal(q, "status", f.Status)
	equal(q, "provider", f.Provider)
	equal(q, "invoiceid", f.InvoiceID)
	return list[domain.Payment](ctx, r.payments, t, q, f.Limit, f.Cursor)
}
func (r *PaymentRepository) Update(ctx context.Context, t string, p *domain.Payment) error {
	return update(ctx, r.payments, t, p.ID, p)
}
func (r *PaymentRepository) Delete(ctx context.Context, t, id string) error {
	return remove(ctx, r.payments, t, id)
}
func (r *PaymentRepository) CommitPaymentLifecycle(ctx context.Context, t string, p *domain.Payment, mutation, kind string, payload map[string]any) error {
	e, err := event(t, kind, payload)
	if err != nil {
		return err
	}
	switch mutation {
	case ports.PaymentMutationCreate:
		return insert(ctx, r.payments, t, p.ID, p, e)
	case ports.PaymentMutationUpdate:
		return update(ctx, r.payments, t, p.ID, p, e)
	case ports.PaymentMutationDelete:
		return remove(ctx, r.payments, t, p.ID, e)
	}
	return fmt.Errorf("unsupported payment mutation %q", mutation)
}

// CommitReconciliation stores the ledger, provider outcome and event in one payment document. The
// predicate serializes competing webhooks/verifications: success never regresses,
// each outcome is recorded once, and a failed outcome may be corrected to success.
func (r *PaymentRepository) CommitReconciliation(ctx context.Context,
	t string,
	p *domain.Payment,
	tx *domain.Transaction,
	kind string,
	payload map[string]any) error {
	if tx.PaymentID != p.ID {
		return domain.ErrValidation
	}
	s, err := r.payments.ScopeTo(t)
	if err != nil {
		return err
	}
	e, err := event(t, kind, payload)
	if err != nil {
		return err
	}
	d, err := dataFields(p, t)
	if err != nil {
		return err
	}
	entry, err := dataFields(tx, t)
	if err != nil {
		return err
	}
	q := live(bson.M{"_id": p.ID, "data.status": bson.M{"$nin": bson.A{p.Status, string(domain.PaymentStatusSuccess)}}})
	res, err := s.UpdateWithEvents(ctx, q, bson.M{"$set": bson.M{"data": d}, "$push": bson.M{"transactions": entry}}, e)
	if err != nil {
		return mapped(err)
	}
	if res.MatchedCount == 1 {
		return nil
	}
	existing, err := r.GetByID(ctx, t, p.ID)
	if err != nil {
		return err
	}
	if existing.Status == p.Status || existing.Status == string(domain.PaymentStatusSuccess) {
		*p = *existing
		return nil
	}
	return domain.ErrConflict
}
func (r *PaymentRepository) ClaimPendingPaymentEvents(ctx context.Context, n int) ([]ports.OutboxEvent, error) {
	items, err := r.outbox.Claim(ctx, n)
	if err != nil {
		return nil, err
	}
	out := make([]ports.OutboxEvent, 0, len(items))
	for _, i := range items {
		created, err := time.Parse(time.RFC3339Nano, i.Event.Time)
		if err != nil {
			return nil, fmt.Errorf("invalid outbox event time: %w", err)
		}
		out = append(out,
			ports.OutboxEvent{ID: i.Event.ID,
				TenantID:  i.Event.TenantID,
				EventType: i.Event.Type,
				Payload:   json.RawMessage(i.Event.Data),
				CreatedAt: created})
	}
	return out, nil
}
func (r *PaymentRepository) MarkPaymentEventPublished(ctx context.Context, id string) error {
	return r.outbox.MarkPublishedByEventID(ctx, id)
}
func (r *PaymentRepository) MarkPaymentEventFailed(ctx context.Context, id, reason string) error {
	return r.outbox.MarkFailedByEventID(ctx, id, reason)
}
func (r *TransactionRepository) Create(ctx context.Context, t string, tx *domain.Transaction) error {
	s, err := r.payments.ScopeTo(t)
	if err != nil {
		return err
	}
	d, err := dataFields(tx, t)
	if err != nil {
		return err
	}
	res, err := s.UpdateOne(ctx, live(bson.M{"_id": tx.PaymentID, "transactions.id": bson.M{"$ne": tx.ID}}), bson.M{"$push": bson.M{"transactions": d}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		if _, err := get[domain.Payment](ctx, r.payments, t, bson.M{"_id": tx.PaymentID}); err != nil {
			return err
		}
		return domain.ErrConflict
	}
	return nil
}
func (r *TransactionRepository) entries(ctx context.Context, t string, q bson.M) ([]*domain.Transaction, error) {
	s, err := r.payments.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Transactions []*domain.Transaction `bson:"transactions"`
	}
	if err = s.FindOne(ctx, live(q)).Decode(&doc); err != nil {
		return nil, mapped(err)
	}
	return doc.Transactions, nil
}
func (r *TransactionRepository) GetByID(ctx context.Context, t, id string) (*domain.Transaction, error) {
	entries, err := r.entries(ctx, t, bson.M{"transactions.id": id})
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *TransactionRepository) ListByPayment(ctx context.Context, t, id string, f ports.TransactionFilter) ([]*domain.Transaction, string, error) {
	entries, err := r.entries(ctx, t, bson.M{"_id": id})
	if err != nil {
		return nil, "", err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	if f.Cursor != "" {
		found := false
		for i, e := range entries {
			if e.ID == f.Cursor {
				entries = entries[i+1:]
				found = true
				break
			}
		}
		if !found {
			return []*domain.Transaction{}, "", nil
		}
	}
	n := f.Limit
	if n <= 0 || n > 100 {
		n = 25
	}
	next := ""
	if len(entries) >= n {
		entries = entries[:n]
		next = entries[n-1].ID
	}
	return entries, next, nil
}
func (r *WebhookEventRepository) Create(ctx context.Context, t string, w *domain.WebhookEvent) error {
	return insert(ctx, r.webhooks, t, w.ID, w)
}
func (r *WebhookEventRepository) GetByID(ctx context.Context, t, id string) (*domain.WebhookEvent, error) {
	return get[domain.WebhookEvent](ctx, r.webhooks, t, bson.M{"_id": id})
}
func (r *WebhookEventRepository) Update(ctx context.Context, t string, w *domain.WebhookEvent) error {
	return update(ctx, r.webhooks, t, w.ID, w)
}
func (r *WebhookEventRepository) List(ctx context.Context, t string, f ports.WebhookEventFilter) ([]*domain.WebhookEvent, string, error) {
	q := bson.M{}
	equal(q, "provider", f.Provider)
	equal(q, "eventtype", f.EventType)
	return list[domain.WebhookEvent](ctx, r.webhooks, t, q, f.Limit, f.Cursor)
}
func (r *WebhookEventRepository) HasProcessedReference(ctx context.Context, t, provider, reference string) (bool, error) {
	s, err := r.webhooks.ScopeTo(t)
	if err != nil {
		return false, err
	}
	cur, err := s.Find(ctx, bson.M{"data.provider": provider, "data.processed": true})
	if err != nil {
		return false, err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Warn("close Mongo cursor", "error", err)
		}
	}()
	for cur.Next(ctx) {
		var d record[domain.WebhookEvent]
		if err = cur.Decode(&d); err != nil {
			return false, err
		}
		var p struct {
			Reference         *string `json:"reference"`
			ProviderReference *string `json:"provider_reference"`
			Data              struct {
				Reference *string `json:"reference"`
			} `json:"data"`
		}
		if err = json.Unmarshal(d.Data.Payload, &p); err != nil {
			return false, err
		}
		ref := p.Reference
		if ref == nil {
			ref = p.ProviderReference
		}
		if ref == nil {
			ref = p.Data.Reference
		}
		if ref != nil && *ref == reference {
			return true, nil
		}
	}
	return false, cur.Err()
}
func EnsureIndexes(ctx context.Context, s *pmongo.Store) error {
	return indexes(ctx, s, map[string][]mongo.IndexModel{
		PaymentCollection: {{Keys: bson.D{{Key: "tenant_id",
			Value: 1},
			{Key: "data.provider",
				Value: 1},
			{Key: "data.providerreference",
				Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"data.providerreference": bson.M{"$type": "string"}})},
			{Keys: bson.D{{Key: "tenant_id",
				Value: 1},
				{Key: "data.createdat",
					Value: 1},
				{Key: "_id",
					Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id",
				Value: 1},
				{Key: "transactions.id",
					Value: 1}}}},

		WebhookCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "data.provider", Value: 1}, {Key: "data.processed", Value: 1}}}},
	})
}
