// Package mongo persists file aggregates in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Tenant isolation comes from platform/mongo.Scope rather than row-level
// security: a query that is not scoped cannot be written. Lifecycle events are
// embedded in the aggregate they describe, which makes the domain change and
// its event a single atomic write on a tier with no multi-document transactions.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	FileCollection     = "file_uploads"
	UsageCollection    = "file_usage"
	outboxLease        = 5 * time.Minute
	eventSource        = "file-service"
	normalizeLimit     = false
)

type Repository struct {
	files   *pmongo.Collection
	usage   *pmongo.Collection
	outbox  *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	files := store.Collection(FileCollection)
	return &Repository{
		files:  files,
		usage:  store.Collection(UsageCollection),
		outbox: pmongo.NewClaimableOutbox(files, outboxLease),
	}
}

// fileDoc mirrors the file_uploads table so both adapters describe the same record.
type fileDoc struct {
	ID               string         `bson:"_id"`
	TenantID         string         `bson:"tenant_id"`
	OriginalFilename string         `bson:"original_filename"`
	StoragePath      string         `bson:"storage_path"`
	StorageBackend   string         `bson:"storage_backend"`
	ContentType      string         `bson:"content_type"`
	SizeBytes        int64          `bson:"size_bytes"`
	Checksum         string         `bson:"checksum"`
	OwnerID          string         `bson:"owner_id"`
	Purpose          string         `bson:"purpose"`
	Status           string         `bson:"status"`
	SecureURL        string         `bson:"secure_url,omitempty"`
	Metadata         map[string]any `bson:"metadata,omitempty"`
	CreatedAt        time.Time      `bson:"created_at"`
	UpdatedAt        time.Time      `bson:"updated_at"`
}

func (d fileDoc) toDomain() *domain.FileUpload {
	return &domain.FileUpload{
		ID:               d.ID,
		TenantID:         d.TenantID,
		OriginalFilename: d.OriginalFilename,
		StoragePath:      d.StoragePath,
		StorageBackend:   d.StorageBackend,
		ContentType:      d.ContentType,
		SizeBytes:        d.SizeBytes,
		Checksum:         d.Checksum,
		OwnerID:          d.OwnerID,
		Purpose:          d.Purpose,
		Status:           d.Status,
		SecureURL:        d.SecureURL,
		Metadata:         d.Metadata,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

func fileFields(f *domain.FileUpload) bson.M {
	return bson.M{
		"original_filename": f.OriginalFilename,
		"storage_path":      f.StoragePath,
		"storage_backend":   f.StorageBackend,
		"content_type":      f.ContentType,
		"size_bytes":        f.SizeBytes,
		"checksum":          f.Checksum,
		"owner_id":          f.OwnerID,
		"purpose":           f.Purpose,
		"status":            f.Status,
		"secure_url":        f.SecureURL,
		"metadata":          f.Metadata,
		"created_at":        f.CreatedAt,
		"updated_at":        f.UpdatedAt,
	}
}

func (r *Repository) Create(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: create: %w", err)
	}
	doc := fileFields(f)
	doc["_id"] = f.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("file: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.FileUpload, error) {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("file: get: %w", err)
	}
	var doc fileDoc
	if err := scope.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("file: get: %w", err)
	}
	return doc.toDomain(), nil
}

// List pages by (created_at, id) ascending, matching the Postgres adapter's order.
func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.FileUpload, string, error) {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("file: list: %w", err)
	}

	// Normalize limit like the Postgres adapter does.
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := bson.M{}
	if cursor != "" {
		// Find the cursor document to get its created_at for comparison.
		// This is keyset pagination: (created_at, id) > (cursor_created_at, cursor_id)
		var cursorDoc fileDoc
		if err := scope.FindOne(ctx, bson.M{"_id": cursor}).Decode(&cursorDoc); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				// Cursor not found, return empty page
				return []*domain.FileUpload{}, "", nil
			}
			return nil, "", fmt.Errorf("file: list: decode cursor: %w", err)
		}
		query["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
			bson.M{
				"created_at": bson.M{"$eq": cursorDoc.CreatedAt},
				"_id":        bson.M{"$gt": cursor},
			},
		}
	}

	cur, err := scope.Find(ctx, query,
		options.Find().SetSort(bson.D{
			{Key: "created_at", Value: 1},
			{Key: "_id", Value: 1},
		}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("file: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.FileUpload
	for cur.Next(ctx) {
		var doc fileDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("file: list decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("file: list rows: %w", err)
	}

	var nextCursor string
	if len(out) == limit && len(out) > 0 {
		nextCursor = out[len(out)-1].ID
	}
	return out, nextCursor, nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: update: %w", err)
	}
	res, err := scope.UpdateOne(ctx, bson.M{"_id": f.ID}, bson.M{"$set": fileFields(f)})
	if err != nil {
		return fmt.Errorf("file: update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: delete: %w", err)
	}
	res, err := scope.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("file: delete: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("file: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0",
		Type:        eventType,
		Source:      eventSource,
		ID:          uuid.NewString(),
		Time:        time.Now().UTC().Format(time.RFC3339),
		TenantID:    tenantID,
		Data:        encoded,
	}, nil
}

// CommitFileLifecycle applies the mutation and queues its event in one write.
//
// A delete is the exception: removing the document would remove the event with
// it, so the record is tombstoned instead and the relay clears it once the
// event is published.
func (r *Repository) CommitFileLifecycle(ctx context.Context, tenantID string, file *domain.FileUpload, mutation, eventType string, payload map[string]any) error {
	scope, err := r.files.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: lifecycle: %w", err)
	}
	event, err := lifecycleEvent(tenantID, eventType, payload)
	if err != nil {
		return err
	}

	switch mutation {
	case ports.FileMutationCreate:
		doc := fileFields(file)
		doc["_id"] = file.ID
		if _, err := scope.InsertWithEvents(ctx, doc, event); err != nil {
			return fmt.Errorf("file: create: %w", err)
		}
	case ports.FileMutationUpdate:
		res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": file.ID}, bson.M{"$set": fileFields(file)}, event)
		if err != nil {
			return fmt.Errorf("file: update: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	case ports.FileMutationDelete:
		res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": file.ID, "deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
		if err != nil {
			return fmt.Errorf("file: delete: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	default:
		return fmt.Errorf("file: unsupported lifecycle mutation %q", mutation)
	}
	return nil
}

func claimed(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID:          c.Event.ID,
			TenantID:    c.Event.TenantID,
			EventType:   c.Event.Type,
			Payload:     json.RawMessage(c.Event.Data),
			CleanupPath: "", // MongoDB doesn't track cleanup path in events yet
		})
	}
	return out
}

func (r *Repository) ClaimPendingFileEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	events, err := r.outbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("file: claim outbox: %w", err)
	}
	return claimed(events), nil
}

func (r *Repository) MarkFileEventPublished(ctx context.Context, id string) error {
	if err := r.outbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("file: mark published: %w", err)
	}
	return r.clearPublishedTombstones(ctx)
}

func (r *Repository) MarkFileEventFailed(ctx context.Context, id, reason string) error {
	if err := r.outbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("file: mark failed: %w", err)
	}
	return nil
}

// clearPublishedTombstones removes deleted records whose delete event has been
// delivered. Holding the tombstone until then is what lets a delete stay atomic
// with its event.
func (r *Repository) clearPublishedTombstones(ctx context.Context) error {
	_, err := r.outbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("file: clear tombstones: %w", err)
	}
	return nil
}

// usageDoc mirrors the file_usage table.
type usageDoc struct {
	TenantID       string `bson:"_id"` // compound id is tenant:date
	Date           string `bson:"date"`
	BytesStored    int64  `bson:"bytes_stored"`
	BytesDelivered int64  `bson:"bytes_delivered"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (r *Repository) RecordStorage(ctx context.Context, tenantID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	scope, err := r.usage.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: record storage: %w", err)
	}

	now := time.Now().UTC()
	date := now.Format(time.DateOnly)
	docID := tenantID + ":" + date

	_, err = scope.UpdateOne(ctx, bson.M{"_id": docID}, bson.M{
		"$inc": bson.M{"bytes_stored": bytes},
		"$set": bson.M{"updated_at": now, "date": date},
		"$setOnInsert": bson.M{"tenant_id": tenantID},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("file: record storage: %w", err)
	}
	return nil
}

func (r *Repository) RecordDelivery(ctx context.Context, tenantID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	scope, err := r.usage.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("file: record delivery: %w", err)
	}

	now := time.Now().UTC()
	date := now.Format(time.DateOnly)
	docID := tenantID + ":" + date

	_, err = scope.UpdateOne(ctx, bson.M{"_id": docID}, bson.M{
		"$inc": bson.M{"bytes_delivered": bytes},
		"$set": bson.M{"updated_at": now, "date": date},
		"$setOnInsert": bson.M{"tenant_id": tenantID},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("file: record delivery: %w", err)
	}
	return nil
}

func (r *Repository) GetUsage(ctx context.Context, tenantID string, limit int) ([]*ports.UsageRecord, error) {
	if limit <= 0 {
		limit = 30
	}
	scope, err := r.usage.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("file: get usage: %w", err)
	}

	cur, err := scope.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("file: get usage: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*ports.UsageRecord
	for cur.Next(ctx) {
		var doc usageDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("file: scan usage: %w", err)
		}
		out = append(out, &ports.UsageRecord{
			TenantID:       doc.TenantID,
			Date:           doc.Date,
			BytesStored:    doc.BytesStored,
			BytesDelivered: doc.BytesDelivered,
		})
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("file: usage rows: %w", err)
	}
	return out, nil
}

// EnsureIndexes replaces the CREATE INDEX statements in the Postgres migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]bson.D{
		FileCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "purpose", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
		},
		UsageCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "date", Value: -1}},
		},
	}
	for name, keys := range specs {
		models := make([]mongo.IndexModel, 0, len(keys))
		for _, k := range keys {
			models = append(models, mongo.IndexModel{Keys: k})
		}
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("file: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}
