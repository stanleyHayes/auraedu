package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/platform/tenancy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The transactional outbox, without transactions.
//
// On PostgreSQL the domain row and its outbox row commit together, so an event
// cannot exist for a change that rolled back, and a change cannot be invisible
// to consumers. On standalone MongoDB, a separate outbox collection requires
// two writes without a transaction and can lose or invent an event at a crash
// between them. Replica sets support transactions, but embedding also works on
// standalone servers.
//
// Single-document writes ARE atomic on every MongoDB tier, so the event is kept
// inside the aggregate it describes: the domain change and its events are one
// write. A relay then publishes pending events and pulls them from the document.
//
// This is at-least-once, not exactly-once — a crash after publishing but before
// the pull re-delivers. Every event therefore carries a stable id used as the
// idempotency key, which is what consumers already deduplicate on.

// PendingField is the document array holding events awaiting publication.
const PendingField = "_pending_events"

// PendingEvent is a CloudEvent queued inside its aggregate document.
type PendingEvent struct {
	ID             string `bson:"id"`
	Type           string `bson:"type"`
	Source         string `bson:"source"`
	Subject        string `bson:"subject,omitempty"`
	Time           string `bson:"time,omitempty"`
	TenantID       string `bson:"tenant_id"`
	IdempotencyKey string `bson:"idempotency_key,omitempty"`
	Data           []byte `bson:"data,omitempty"`
}

func toPending(event tenancy.CloudEvent) (PendingEvent, error) {
	if err := event.Validate(); err != nil {
		return PendingEvent{}, fmt.Errorf("mongo outbox: %w", err)
	}
	key := event.IdempotencyKey
	if key == "" {
		// The event id is already unique per emission, so it is the natural
		// dedupe key for the redelivery this design accepts.
		key = event.ID
	}
	return PendingEvent{
		ID: event.ID, Type: event.Type, Source: event.Source, Subject: event.Subject,
		Time: event.Time, TenantID: event.TenantID, IdempotencyKey: key,
		Data: []byte(event.Data),
	}, nil
}

func (p PendingEvent) CloudEvent() tenancy.CloudEvent {
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: p.Type, Source: p.Source, Subject: p.Subject,
		ID: p.ID, Time: p.Time, TenantID: p.TenantID,
		IdempotencyKey: p.IdempotencyKey, Data: json.RawMessage(p.Data),
	}
}

// InsertWithEvents writes a document and the events it emits as ONE atomic
// single-document write, which is the property the PostgreSQL adapters get from
// a transaction.
func (s *Scope) InsertWithEvents(ctx context.Context, doc bson.M, events ...tenancy.CloudEvent) (*mongo.InsertOneResult, error) {
	pending, err := s.pendingFor(events)
	if err != nil {
		return nil, err
	}
	stamped, err := s.stamp(doc)
	if err != nil {
		return nil, err
	}
	if len(pending) > 0 {
		stamped[PendingField] = pending
	}
	return s.coll.InsertOne(ctx, stamped)
}

// UpdateWithEvents applies an update and queues its events in the same atomic
// single-document write.
func (s *Scope) UpdateWithEvents(ctx context.Context, query bson.M, update bson.M, events ...tenancy.CloudEvent) (*mongo.UpdateResult, error) {
	if err := guardUpdate(update, s.tenantKey()); err != nil {
		return nil, err
	}
	pending, err := s.pendingFor(events)
	if err != nil {
		return nil, err
	}
	merged := bson.M{}
	for k, v := range update {
		merged[k] = v
	}
	if len(pending) > 0 {
		push, err := copyOperatorDocument(merged["$push"])
		if err != nil {
			return nil, err
		}
		if _, taken := push[PendingField]; taken {
			return nil, errors.New("mongo outbox: update already pushes to " + PendingField)
		}
		push[PendingField] = bson.M{"$each": pending}
		merged["$push"] = push
	}
	return s.coll.UpdateOne(ctx, s.filter(query), merged)
}

// UpsertWithEvents inserts or updates one document and queues its events in the
// same atomic single-document write. It exists because a flag that is set for
// the first time and one that is being changed must both stay atomic with the
// event announcing it; doing the upsert and the event as two calls would put a
// crash between them.
func (s *Scope) UpsertWithEvents(ctx context.Context, query bson.M, update bson.M, events ...tenancy.CloudEvent) (*mongo.UpdateResult, error) {
	pending, err := s.pendingFor(events)
	if err != nil {
		return nil, err
	}
	merged := bson.M{}
	for k, v := range update {
		merged[k] = v
	}
	if len(pending) > 0 {
		push, err := copyOperatorDocument(merged["$push"])
		if err != nil {
			return nil, err
		}
		if _, taken := push[PendingField]; taken {
			return nil, errors.New("mongo outbox: update already pushes to " + PendingField)
		}
		push[PendingField] = bson.M{"$each": pending}
		merged["$push"] = push
	}
	return s.UpsertOne(ctx, query, merged)
}

func (s *Scope) pendingFor(events []tenancy.CloudEvent) ([]PendingEvent, error) {
	pending := make([]PendingEvent, 0, len(events))
	for _, event := range events {
		// An event must belong to the scope that emitted it, or publishing it
		// would announce a change under another tenant's name.
		if s.tenantID != "" && event.TenantID != s.tenantID {
			return nil, fmt.Errorf("mongo outbox: event tenant %q does not match scope %q", event.TenantID, s.tenantID)
		}
		p, err := toPending(event)
		if err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, nil
}

// Publisher delivers an event to the bus. It must be safe to call more than once
// for the same event, because this outbox is at-least-once.
type Publisher interface {
	Publish(ctx context.Context, event tenancy.CloudEvent) error
}

// Relay drains events queued inside documents of one collection.
type Relay struct {
	coll      *Collection
	publisher Publisher
	batch     int64
}

func NewRelay(coll *Collection, publisher Publisher, batch int64) *Relay {
	if batch <= 0 {
		batch = 100
	}
	return &Relay{coll: coll, publisher: publisher, batch: batch}
}

// Drain publishes every pending event it finds and removes each one only after
// its publish succeeds, so a failure redelivers rather than drops. It returns
// the number of events published.
//
// It reads across tenants by design: the relay is infrastructure draining its
// own service's collection, not a request acting for a user.
func (r *Relay) Drain(ctx context.Context) (int, error) {
	cursor, err := r.coll.coll.Find(ctx,
		bson.M{PendingField: bson.M{"$exists": true, "$ne": bson.A{}}},
		options.Find().SetLimit(r.batch),
	)
	if err != nil {
		return 0, fmt.Errorf("mongo outbox: find pending: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	published := 0
	var failures []error
	for cursor.Next(ctx) {
		var doc struct {
			ID      any            `bson:"_id"`
			Pending []PendingEvent `bson:"_pending_events"`
		}
		if err := cursor.Decode(&doc); err != nil {
			failures = append(failures, fmt.Errorf("decode pending document: %w", err))
			continue
		}
		for _, pending := range doc.Pending {
			if err := r.publisher.Publish(ctx, pending.CloudEvent()); err != nil {
				// Leave it queued; the next drain retries it.
				failures = append(failures, fmt.Errorf("publish %s: %w", pending.ID, err))
				continue
			}
			if _, err := r.coll.coll.UpdateOne(ctx,
				bson.M{"_id": doc.ID},
				bson.M{"$pull": bson.M{PendingField: bson.M{"id": pending.ID}}},
			); err != nil {
				// Published but not cleared: it will be redelivered, which is why
				// the event carries a stable idempotency key.
				failures = append(failures, fmt.Errorf("clear %s after publish: %w", pending.ID, err))
				continue
			}
			published++
		}
	}
	if err := cursor.Err(); err != nil {
		failures = append(failures, fmt.Errorf("iterate pending: %w", err))
	}
	return published, errors.Join(failures...)
}

// copyOperatorDocument preserves BSON-supported document shapes without mutating
// the caller's map when the outbox adds its own array push.
func copyOperatorDocument(value any) (bson.M, error) {
	result := bson.M{}
	if value == nil {
		return result, nil
	}
	fields, err := documentFields(value)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		result[field.Key] = field.Value
	}
	return result, nil
}
