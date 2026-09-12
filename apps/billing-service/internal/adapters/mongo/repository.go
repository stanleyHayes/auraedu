package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/auth"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const PlanCollection = "billing_plans"
const SubscriptionCollection = "billing_subscriptions"
const InvoiceCollection = "billing_invoices"

// PlanRepository stores the global catalog, explicitly platform-owned as in PostgreSQL.
type PlanRepository struct {
	plans *pmongo.PlatformScope
	store *pmongo.Store
}
type SubscriptionRepository struct {
	subscriptions *pmongo.Collection
	store         *pmongo.Store
	plans         *PlanRepository
	outboxes      []*pmongo.ClaimableOutbox
}
type SaaSInvoiceRepository struct {
	invoices, subscriptions *pmongo.Collection
	store                   *pmongo.Store
}

var (
	_ ports.PlanRepository                  = (*PlanRepository)(nil)
	_ ports.SubscriptionRepository          = (*SubscriptionRepository)(nil)
	_ ports.SubscriptionLifecycleRepository = (*SubscriptionRepository)(nil)
	_ ports.OutboxRepository                = (*SubscriptionRepository)(nil)
	_ ports.SaaSInvoiceRepository           = (*SaaSInvoiceRepository)(nil)
	_ ports.InvoiceLifecycleRepository      = (*SaaSInvoiceRepository)(nil)
)

func NewPlanRepository(s *pmongo.Store) *PlanRepository {
	return &PlanRepository{s.Collection(PlanCollection).PlatformOwned(), s}
}
func NewSubscriptionRepository(s *pmongo.Store) *SubscriptionRepository {
	return &SubscriptionRepository{s.Collection(SubscriptionCollection),
		s,
		NewPlanRepository(s),
		[]*pmongo.ClaimableOutbox{pmongo.NewClaimableOutbox(s.Collection(SubscriptionCollection),
			time.Minute),
			pmongo.NewClaimableOutbox(s.Collection(InvoiceCollection),
				time.Minute)}}
}
func NewSaaSInvoiceRepository(s *pmongo.Store) *SaaSInvoiceRepository {
	return &SaaSInvoiceRepository{s.Collection(InvoiceCollection), s.Collection(SubscriptionCollection), s}
}
func (r *PlanRepository) Create(ctx context.Context, p *domain.Plan) error {
	_, err := r.plans.InsertOne(ctx, bson.M{"_id": p.ID, "data": p})
	return mapped(err)
}
func (r *PlanRepository) find(ctx context.Context, q bson.M) (*domain.Plan, error) {
	var d record[domain.Plan]
	if err := r.plans.FindOne(ctx, live(q)).Decode(&d); err != nil {
		return nil, mapped(err)
	}
	return &d.Data, nil
}
func (r *PlanRepository) GetByID(ctx context.Context, id string) (*domain.Plan, error) {
	return r.find(ctx, bson.M{"_id": id})
}
func (r *PlanRepository) GetByCode(ctx context.Context, code string) (*domain.Plan, error) {
	return r.find(ctx, bson.M{"data.code": code})
}
func (r *PlanRepository) Update(ctx context.Context, p *domain.Plan) error {
	return changed(r.plans.UpdateOne(ctx, live(bson.M{"_id": p.ID}), bson.M{"$set": bson.M{"data": p}}))
}
func (r *PlanRepository) Delete(ctx context.Context, id string) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		result, err := r.plans.DeleteOne(ctx, live(bson.M{"_id": id}))
		if err != nil {
			return err
		}
		if result.DeletedCount != 1 {
			return domain.ErrNotFound
		}
		scope, err := r.store.Collection(SubscriptionCollection).Scope(auth.WithActor(ctx, auth.Actor{PlatformAdmin: true, Role: auth.RolePlatformSuperAdmin}))
		if err != nil {
			return err
		}
		n, err := scope.CountDocuments(ctx, live(bson.M{"data.planid": id}))
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrConflict
		}
		return nil
	})
}
func (r *PlanRepository) List(ctx context.Context, f ports.PlanFilter) ([]*domain.Plan, string, error) {
	q := bson.M{}
	equal(q, "status", f.Status)
	if f.Cursor != "" {
		p, err := r.GetByID(ctx, f.Cursor)
		if err != nil {
			return nil, "", err
		}
		q["$or"] = bson.A{bson.M{"data.createdat": bson.M{"$gt": p.CreatedAt}}, bson.M{"data.createdat": p.CreatedAt, "_id": bson.M{"$gt": p.ID}}}
	}
	n := f.Limit
	if n <= 0 || n > 100 {
		n = 25
	}
	cur, err := r.plans.Find(ctx, live(q), options.Find().SetSort(bson.D{{Key: "data.createdat", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(n)))
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Warn("close Mongo cursor", "error", err)
		}
	}()
	out := []*domain.Plan{}
	for cur.Next(ctx) {
		var d record[domain.Plan]
		if err = cur.Decode(&d); err != nil {
			return nil, "", err
		}
		out = append(out, &d.Data)
	}
	next := ""
	if len(out) == n {
		next = out[n-1].ID
	}
	return out, next, cur.Err()
}
func (r *SubscriptionRepository) Create(ctx context.Context, t string, s *domain.Subscription) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := r.plans.touch(ctx, s.PlanID); err != nil {
			return err
		}
		return insert(ctx, r.subscriptions, t, s.ID, s)
	})
}
func (r *SubscriptionRepository) GetByID(ctx context.Context, t, id string) (*domain.Subscription, error) {
	return get[domain.Subscription](ctx, r.subscriptions, t, bson.M{"_id": id})
}
func (r *SubscriptionRepository) List(ctx context.Context, t string, f ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	q := bson.M{}
	equal(q, "status", f.Status)
	equal(q, "planid", f.PlanID)
	return list[domain.Subscription](ctx, r.subscriptions, t, q, f.Limit, f.Cursor)
}
func (r *SubscriptionRepository) Update(ctx context.Context, t string, s *domain.Subscription) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := r.plans.touch(ctx, s.PlanID); err != nil {
			return err
		}
		return update(ctx, r.subscriptions, t, s.ID, s)
	})
}
func (r *SubscriptionRepository) Delete(ctx context.Context, t, id string) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := remove(ctx, r.subscriptions, t, id); err != nil {
			return err
		}
		scope, err := r.store.Collection(InvoiceCollection).ScopeTo(t)
		if err != nil {
			return err
		}
		n, err := scope.CountDocuments(ctx, live(bson.M{"data.subscriptionid": id}))
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrConflict
		}
		return nil
	})
}
func lifecycle(t string, events []ports.LifecycleEvent) ([]tenancy.CloudEvent, error) {
	if len(events) == 0 {
		return nil, domain.ErrValidation
	}
	out := make([]tenancy.CloudEvent, 0, len(events))
	for _, e := range events {
		if e.TenantID != t {
			return nil, domain.ErrForbidden
		}
		v, err := event(t, e.EventType, e.Payload)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *SubscriptionRepository) CommitSubscriptionLifecycle(ctx context.Context,
	t,
	mutation string,
	s *domain.Subscription,
	events []ports.LifecycleEvent) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if s == nil {
			return domain.ErrValidation
		}
		if err := r.plans.touch(ctx, s.PlanID); err != nil {
			return err
		}
		es, err := lifecycle(t, events)
		if err != nil {
			return err
		}
		switch mutation {
		case ports.BillingMutationSubscriptionCreate:
			return insert(ctx, r.subscriptions, t, s.ID, s, es...)
		case ports.BillingMutationSubscriptionUpdate:
			return update(ctx, r.subscriptions, t, s.ID, s, es...)
		}
		return fmt.Errorf("unsupported billing mutation %q", mutation)
	})
}
func (r *SaaSInvoiceRepository) checkSubscription(ctx context.Context, t, id string) error {
	scope, err := r.subscriptions.ScopeTo(t)
	if err != nil {
		return err
	}
	return changed(scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$inc": bson.M{"_reference_version": 1}}))
}
func (r *SaaSInvoiceRepository) Create(ctx context.Context, t string, i *domain.SaaSInvoice) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := r.checkSubscription(ctx, t, i.SubscriptionID); err != nil {
			return err
		}
		return insert(ctx, r.invoices, t, i.ID, i)
	})
}
func (r *SaaSInvoiceRepository) GetByID(ctx context.Context, t, id string) (*domain.SaaSInvoice, error) {
	return get[domain.SaaSInvoice](ctx, r.invoices, t, bson.M{"_id": id})
}
func (r *SaaSInvoiceRepository) List(ctx context.Context, t string, f ports.SaaSInvoiceFilter) ([]*domain.SaaSInvoice, string, error) {
	q := bson.M{}
	equal(q, "status", f.Status)
	equal(q, "subscriptionid", f.SubscriptionID)
	return list[domain.SaaSInvoice](ctx, r.invoices, t, q, f.Limit, f.Cursor)
}
func (r *SaaSInvoiceRepository) Update(ctx context.Context, t string, i *domain.SaaSInvoice) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := r.checkSubscription(ctx, t, i.SubscriptionID); err != nil {
			return err
		}
		return update(ctx, r.invoices, t, i.ID, i)
	})
}
func (r *SaaSInvoiceRepository) Delete(ctx context.Context, t, id string) error {
	return remove(ctx, r.invoices, t, id)
}
func (r *SaaSInvoiceRepository) CommitInvoiceLifecycle(ctx context.Context, t, mutation string, i *domain.SaaSInvoice, events []ports.LifecycleEvent) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if i == nil || mutation != ports.BillingMutationInvoiceCreate {
			return domain.ErrValidation
		}
		if err := r.checkSubscription(ctx, t, i.SubscriptionID); err != nil {
			return err
		}
		es, err := lifecycle(t, events)
		if err != nil {
			return err
		}
		return insert(ctx, r.invoices, t, i.ID, i, es...)
	})
}
func (r *SubscriptionRepository) ClaimPendingBillingEvents(ctx context.Context, n int) ([]ports.OutboxEvent, error) {
	if n <= 0 || n > 100 {
		n = 25
	}
	out := []ports.OutboxEvent{}
	for _, o := range r.outboxes {
		if len(out) == n {
			break
		}
		items, err := o.Claim(ctx, n-len(out))
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			out = append(out, ports.OutboxEvent{ID: i.Event.ID, TenantID: i.Event.TenantID, EventType: i.Event.Type, Payload: json.RawMessage(i.Event.Data)})
		}
	}
	return out, nil
}
func (r *SubscriptionRepository) MarkBillingEventPublished(ctx context.Context, id string) error {
	for _, o := range r.outboxes {
		if err := o.MarkPublishedByEventID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *SubscriptionRepository) MarkBillingEventFailed(ctx context.Context, id, reason string) error {
	for _, o := range r.outboxes {
		if err := o.MarkFailedByEventID(ctx, id, reason); err != nil {
			return err
		}
	}
	return nil
}
func EnsureIndexes(ctx context.Context, s *pmongo.Store) error {
	return indexes(ctx, s, map[string][]mongo.IndexModel{
		PlanCollection: {{Keys: bson.D{{Key: "data.code", Value: 1}}, Options: options.Index().SetUnique(true)}},
		SubscriptionCollection: {{Keys: bson.D{{Key: "tenant_id",
			Value: 1}},
			Options: options.Index().SetName("single_trial_per_tenant").SetUnique(true).SetPartialFilterExpression(bson.M{"data.status": "trialing",
				"deleted_at": nil})},
			{Keys: bson.D{{Key: "tenant_id",
				Value: 1},
				{Key: "data.createdat",
					Value: 1},
				{Key: "_id",
					Value: 1}}}},

		InvoiceCollection: {{Keys: bson.D{{Key: "tenant_id",
			Value: 1},
			{Key: "data.subscriptionid",
				Value: 1},
			{Key: "data.createdat",
				Value: 1},
			{Key: "_id",
				Value: 1}}}},
	})
}

func (r *PlanRepository) touch(ctx context.Context, id string) error {
	return changed(r.plans.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$inc": bson.M{"_reference_version": 1}}))
}
