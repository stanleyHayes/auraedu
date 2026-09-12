// Package mongo provides the MongoDB implementation of the audit repository port.
//
// It sits beside the Postgres adapter rather than replacing it: both satisfy
// ports.Repository, and platform/store decides which one a service wires in.
//
// Where the Postgres adapter leans on row-level security to bound a query to its
// tenant, this one goes through platform/mongo.Scope, which carries the tenant
// filter on every operation and offers no way to issue an unscoped query.
package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CollectionName mirrors the audit_logs table.
const CollectionName = "audit_logs"

type Repository struct {
	coll *pmongo.Collection
}

var _ ports.Repository = (*Repository)(nil)

func NewRepository(store *pmongo.Store) *Repository {
	return &Repository{coll: store.Collection(CollectionName)}
}

// auditDoc is the stored shape. Field names match the Postgres columns so the
// two adapters describe the same record and a migration between them is a copy.
type auditDoc struct {
	ID            string    `bson:"_id"`
	TenantID      string    `bson:"tenant_id"`
	EventID       string    `bson:"event_id"`
	EventType     string    `bson:"event_type"`
	SourceService string    `bson:"source_service"`
	Timestamp     time.Time `bson:"timestamp"`
	ReceivedAt    time.Time `bson:"received_at"`
	Payload       string    `bson:"payload,omitempty"`
	ActorID       string    `bson:"actor_id,omitempty"`
	Action        string    `bson:"action"`
	ResourceType  string    `bson:"resource_type,omitempty"`
	ResourceID    string    `bson:"resource_id,omitempty"`
}

func (d auditDoc) toDomain() (*domain.AuditLog, error) {
	id, err := uuid.Parse(d.ID)
	if err != nil {
		return nil, fmt.Errorf("audit: stored id %q is not a uuid: %w", d.ID, err)
	}
	log := &domain.AuditLog{
		ID: id, TenantID: d.TenantID, EventID: d.EventID, EventType: d.EventType,
		SourceService: d.SourceService, Timestamp: d.Timestamp, ReceivedAt: d.ReceivedAt,
		ActorID: d.ActorID, Action: d.Action,
		ResourceType: d.ResourceType, ResourceID: d.ResourceID,
	}
	if d.Payload != "" {
		log.Payload = json.RawMessage(d.Payload)
	}
	return log, nil
}

// Insert persists an immutable audit log record.
func (r *Repository) Insert(ctx context.Context, log *domain.AuditLog) error {
	scope, err := r.coll.Scope(ctx)
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	doc := bson.M{
		"_id": log.ID.String(), "event_id": log.EventID, "event_type": log.EventType,
		"source_service": log.SourceService, "timestamp": log.Timestamp,
		"received_at": log.ReceivedAt, "actor_id": log.ActorID, "action": log.Action,
		"resource_type": log.ResourceType, "resource_id": log.ResourceID,
	}
	if len(log.Payload) > 0 {
		doc["payload"] = string(log.Payload)
	}
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

// List returns a tenant-scoped page ordered newest-first by id (UUID v7, whose
// string form sorts by time), matching the Postgres adapter's ordering.
func (r *Repository) List(ctx context.Context, tenantID string, filter domain.ListFilter, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	scope, err := r.coll.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("audit: list: %w", err)
	}
	return page(ctx, scope, filter, limit, cursor)
}

// ListAll returns a cross-tenant page for platform super admins. The scope is
// derived from the actor in ctx, so a caller without platform authority is
// refused here exactly as the RLS policy refuses it on Postgres.
func (r *Repository) ListAll(ctx context.Context, filter domain.ListFilter, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	scope, err := r.coll.Scope(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("audit: list all: %w", err)
	}
	return page(ctx, scope, filter, limit, cursor)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func page(ctx context.Context, scope *pmongo.Scope, filter domain.ListFilter, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	limit = normalizeLimit(limit)

	query, err := buildQuery(filter, cursor)
	if err != nil {
		return nil, "", err
	}
	cur, err := scope.Find(ctx, query,
		options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("audit: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.AuditLog
	for cur.Next(ctx) {
		var doc auditDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("audit: decode: %w", err)
		}
		log, err := doc.toDomain()
		if err != nil {
			return nil, "", err
		}
		out = append(out, log)
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("audit: list rows: %w", err)
	}

	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID.String()
	}
	return out, next, nil
}

func buildQuery(filter domain.ListFilter, cursor string) (bson.M, error) {
	query := bson.M{}
	if filter.EventType != "" {
		query["event_type"] = filter.EventType
	}
	if filter.ActorID != "" {
		query["actor_id"] = filter.ActorID
	}
	if filter.SourceService != "" {
		query["source_service"] = filter.SourceService
	}
	if filter.From != nil || filter.To != nil {
		window := bson.M{}
		if filter.From != nil {
			window["$gte"] = *filter.From
		}
		if filter.To != nil {
			window["$lte"] = *filter.To
		}
		query["timestamp"] = window
	}
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid cursor: %w", err)
		}
		// Newest-first, so the next page continues below the last id seen.
		query["_id"] = bson.M{"$lt": id.String()}
	}
	return query, nil
}

// EnsureIndexes creates the indexes the queries above rely on. It replaces the
// CREATE INDEX statements in the Postgres migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	coll := store.Database().Collection(CollectionName)
	_, err := coll.Indexes().CreateMany(ctx, []mongodriver.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: -1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "event_type", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "actor_id", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "timestamp", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("audit: ensure indexes: %w", err)
	}
	return nil
}
