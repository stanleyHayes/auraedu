package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var _ ports.DurableDeliveryRepository = (*MessageRepository)(nil)
var _ ports.DeliveryFeedbackRepository = (*MessageRepository)(nil)
var _ ports.OutboxRepository = (*MessageRepository)(nil)
var _ ports.ScheduledMessageRepository = (*MessageRepository)(nil)

func event(tenant, kind string, payload map[string]any) (tenancy.CloudEvent, error) {
	data, err := json.Marshal(payload)
	return tenancy.CloudEvent{SpecVersion: "1.0", ID: uuid.NewString(),
			Type: kind, Source: "notification-service", TenantID: tenant,
			Time: time.Now().UTC().Format(time.RFC3339), Data: data},
		err
}
func admin(ctx context.Context) context.Context {
	return auth.WithActor(tenancy.WithContext(ctx, tenancy.TenantContext{}), auth.Actor{PlatformAdmin: true, UserID: "notification-infrastructure"})
}
func (r *MessageRepository) CommitDeliveryOutcome(ctx context.Context,
	tenant string, m *domain.Message, pid string, create bool,
	kind string, payload map[string]any) error {
	m.TenantID = tenant
	scope, err := r.store.Collection("messages").ScopeTo(tenant)
	if err != nil {
		return err
	}
	e, err := event(tenant, kind, payload)
	if err != nil {
		return err
	}
	fields := bson.M{"body": m, "provider_message_id": pid}
	if create {
		fields["_id"] = m.ID
		fields["deleted"] = false
		_, err = scope.InsertWithEvents(ctx, fields, e)
		return mapped(err)
	}
	res, err := scope.UpdateWithEvents(ctx, live(m.ID), bson.M{"$set": fields}, e)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func outboxCollections() []string { return []string{"messages", "communication_journeys"} }

func (r *MessageRepository) ClaimPendingNotificationEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := make([]ports.OutboxEvent, 0)
	for _, name := range outboxCollections() {
		if len(out) >= limit {
			break
		}
		es, err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).Claim(ctx, limit-len(out))
		if err != nil {
			return nil, err
		}
		for _, e := range es {
			out = append(out, ports.OutboxEvent{ID: e.Event.ID, TenantID: e.Event.TenantID, EventType: e.Event.Type, Payload: e.Event.Data})
		}
	}
	return out, nil
}
func (r *MessageRepository) MarkNotificationEventPublished(ctx context.Context, id string) error {
	for _, name := range outboxCollections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkPublishedByEventID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *MessageRepository) MarkNotificationEventFailed(ctx context.Context, id, reason string) error {
	for _, name := range outboxCollections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkFailedByEventID(ctx, id, reason); err != nil {
			return err
		}
	}
	return nil
}
func (r *MessageRepository) ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	scope, err := r.store.Collection("messages").Scope(admin(ctx))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	q := live("")
	q["body.status"] = "pending"
	q["body.scheduledat"] = bson.M{"$ne": nil, "$lte": now}
	cur, err := scope.Find(ctx, q, options.Find().SetSort(bson.D{{Key: "body.scheduledat", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cur)
	out := make([]*domain.Message, 0)
	for cur.Next(ctx) {
		var d struct {
			ID     string         `bson:"_id"`
			Tenant string         `bson:"tenant_id"`
			Body   domain.Message `bson:"body"`
		}
		if err = cur.Decode(&d); err != nil {
			return nil, err
		}
		tenantScope, err := r.store.Collection("messages").ScopeTo(d.Tenant)
		if err != nil {
			return nil, err
		}
		until := now.Add(lease)
		res, err := tenantScope.UpdateOne(ctx, bson.M{"_id": d.ID,
			"deleted": false, "body.status": "pending", "body.scheduledat": d.Body.ScheduledAt},
			bson.M{"$set": bson.M{"body.scheduledat": until, "body.updatedat": now}})
		if err != nil {
			return nil, err
		}
		if res.MatchedCount == 1 {
			d.Body.ScheduledAt = &until
			d.Body.UpdatedAt = now
			out = append(out, &d.Body)
		}
	}
	return out, cur.Err()
}
func (r *MessageRepository) CancelByApplication(ctx context.Context, tenant, id string) error {
	scope, err := r.store.Collection("messages").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.UpdateMany(ctx, bson.M{"body.status": "pending",
		"body.metadata.application_id": id}, bson.M{"$set": bson.M{"body.status": "cancelled",
		"body.updatedat": time.Now().UTC()}})
	return err
}
func (r *MessageRepository) IsEmailSuppressed(ctx context.Context, tenant, hash string) (bool, error) {
	scope, err := r.store.Collection("notification_email_suppressions").ScopeTo(tenant)
	if err != nil {
		return false, err
	}
	n, err := scope.CountDocuments(ctx, bson.M{"address_hash": hash})
	return n > 0, err
}
func (r *MessageRepository) SuppressEmail(ctx context.Context, tenant, hash, reason, id string, at time.Time) error {
	return r.suppress(ctx, tenant, hash, reason, id, "auraedu", at)
}
func (r *MessageRepository) suppress(ctx context.Context, tenant, hash, reason, id, provider string, at time.Time) error {
	scope, err := r.store.Collection("notification_email_suppressions").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.UpsertOne(ctx, bson.M{"address_hash": hash},
		bson.M{"$set": bson.M{"reason": reason, "provider": provider,
			"last_event_id": id, "updated_at": time.Now().UTC()},
			"$min": bson.M{"suppressed_at": at}, "$setOnInsert": bson.M{"first_event_id": id}})
	return err
}
func rank(status string) int {
	return map[string]int{"accepted": 1, "delayed": 2, "delivered": 3, "failed": 4, "bounced": 5, "suppressed": 6, "complained": 7}[status]
}
func (r *MessageRepository) ApplyDeliveryFeedback(ctx context.Context, f ports.DeliveryFeedback) (bool, error) {
	applied := false
	err := r.tx(ctx, func(ctx context.Context) error {
		applied = false
		scope, err := r.store.Collection("messages").Scope(admin(ctx))
		if err != nil {
			return err
		}
		var d struct {
			Tenant string         `bson:"tenant_id"`
			Body   domain.Message `bson:"body"`
		}
		err = scope.FindOne(ctx, bson.M{"_id": f.MessageID, "deleted": false,
			"provider_message_id": f.ProviderMessageID, "body.provider": f.Provider,
			"body.metadata.delivery_address_hash": f.AddressHash}).Decode(&d)
		if err != nil {
			return mapped(err)
		}
		ledger, err := r.store.Collection("notification_delivery_events").ScopeTo(d.Tenant)
		if err != nil {
			return err
		}
		existing, err := ledger.CountDocuments(ctx, bson.M{"_id": f.ID})
		if err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		if _, err = ledger.InsertOne(ctx, bson.M{"_id": f.ID, "body": f}); err != nil {
			return err
		}
		if err := r.applyFeedbackProjection(ctx, d.Tenant, d.Body, f); err != nil {
			return err
		}

		applied = true
		return nil
	})
	if driver.IsDuplicateKeyError(err) {
		// A concurrent insertion is a replay only if its event actually committed.
		// A suppression-key conflict must remain retryable, never acknowledged.
		ledger, scopeErr := r.store.Collection("notification_delivery_events").Scope(admin(ctx))
		if scopeErr != nil {
			return false, scopeErr
		}
		n, readErr := ledger.CountDocuments(ctx, bson.M{"_id": f.ID})
		if readErr != nil {
			return false, readErr
		}
		if n > 0 {
			return false, nil
		}
	}
	return applied && err == nil, err
}
func (r *MessageRepository) NextJourneyDeliveryAllowedAt(ctx context.Context,
	tenant, journey, recipient string, window time.Duration,
	limit int) (*time.Time, error) {
	if window <= 0 || limit <= 0 {
		return nil, nil
	}
	scope, err := r.store.Collection("messages").ScopeTo(tenant)
	if err != nil {
		return nil, err
	}
	q := bson.M{"body.recipientid": recipient, "body.status": "sent", "body.metadata.journey_id": journey, "body.sentat": bson.M{"$gt": time.Now().Add(-window)}}
	n, err := scope.CountDocuments(ctx, q)
	if err != nil || n < int64(limit) {
		return nil, err
	}
	var d struct {
		Body domain.Message `bson:"body"`
	}
	if err = scope.FindOne(ctx, q, options.FindOne().SetSort(bson.D{{Key: "body.sentat", Value: 1}})).Decode(&d); err != nil {
		return nil, err
	}
	if d.Body.SentAt == nil {
		return nil, errors.New("sent notification missing sent_at")
	}
	at := d.Body.SentAt.Add(window)
	return &at, nil
}

func (r *MessageRepository) applyFeedbackProjection(ctx context.Context, tenant string, message domain.Message, f ports.DeliveryFeedback) error {
	m := message
	old := 0
	if m.DeliveryStatus != nil {
		old = rank(*m.DeliveryStatus)
	}
	if rank(f.Status) > old || (rank(f.Status) == old && (m.DeliveryStatusAt == nil || m.DeliveryStatusAt.Before(f.OccurredAt))) {
		target, err := r.store.Collection("messages").ScopeTo(tenant)
		if err != nil {
			return err
		}
		if _, err = target.UpdateOne(ctx, bson.M{"_id": f.MessageID},
			bson.M{"$set": bson.M{"body.deliverystatus": f.Status,
				"body.deliverystatusat": f.OccurredAt, "body.updatedat": time.Now().UTC()}}); err != nil {
			return err
		}
	}
	if f.Status == "bounced" || f.Status == "complained" || f.Status == "suppressed" {
		if err := r.suppress(ctx, tenant, f.AddressHash, f.Status, f.ID, f.Provider, f.OccurredAt); err != nil {
			return err
		}
	}
	return nil
}
