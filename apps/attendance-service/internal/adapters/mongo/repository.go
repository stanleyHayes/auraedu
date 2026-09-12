// Package mongo persists attendance records in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Tenant isolation comes from platform/mongo.Scope rather than row-level
// security: a query that is not scoped cannot be written.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	RecordCollection = "attendance_records"
	outboxLease      = 5 * time.Minute
	eventSource      = "attendance-service"
)

type Repository struct {
	store   *pmongo.Store
	records *pmongo.Collection
	outbox  *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	records := store.Collection(RecordCollection)
	return &Repository{store: store, records: records, outbox: pmongo.NewClaimableOutbox(records, outboxLease)}
}

func live(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	merged["deleted_at"] = bson.M{"$exists": false}
	return merged
}

func notFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	return err
}

type recordDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	StudentID      string    `bson:"student_id"`
	AcademicYearID string    `bson:"academic_year_id"`
	ClassID        *string   `bson:"class_id,omitempty"`
	SubjectID      *string   `bson:"subject_id,omitempty"`
	Date           string    `bson:"date"`
	Status         string    `bson:"status"`
	Reason         *string   `bson:"reason,omitempty"`
	MarkedBy       string    `bson:"marked_by"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d recordDoc) toDomain() *domain.AttendanceRecord {
	date, _ := domain.NewDate(d.Date)
	return &domain.AttendanceRecord{
		ID: d.ID, TenantID: d.TenantID, StudentID: d.StudentID,
		AcademicYearID: d.AcademicYearID, ClassID: d.ClassID, SubjectID: d.SubjectID,
		Date: date, Status: d.Status, Reason: d.Reason, MarkedBy: d.MarkedBy,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func recordFields(r *domain.AttendanceRecord) bson.M {
	return bson.M{
		"student_id": r.StudentID, "academic_year_id": r.AcademicYearID,
		"class_id": r.ClassID, "subject_id": r.SubjectID, "date": r.Date.String(),
		"status": r.Status, "reason": r.Reason, "marked_by": r.MarkedBy,
		"created_at": r.CreatedAt, "updated_at": r.UpdatedAt,
	}
}

func (r *Repository) Create(ctx context.Context, tenantID string, rec *domain.AttendanceRecord) error {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := recordFields(rec)
	doc["_id"] = rec.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("attendance: create: %w", err)
	}
	return nil
}

// UpsertMany atomically marks the entire register.
func (r *Repository) UpsertMany(ctx context.Context, tenantID string, records []*domain.AttendanceRecord) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.records.ScopeTo(tenantID)
		if err != nil {
			return err
		}
		for _, rec := range records {
			if _, err := scope.UpsertOne(ctx, live(bson.M{
				"student_id": rec.StudentID, "academic_year_id": rec.AcademicYearID,
				"date": rec.Date.String(),
			}), bson.M{
				"$set": bson.M{
					"class_id": rec.ClassID, "subject_id": rec.SubjectID,
					"status": rec.Status, "reason": rec.Reason, "marked_by": rec.MarkedBy,
					"updated_at": rec.UpdatedAt,
				},
				"$setOnInsert": bson.M{
					"_id": rec.ID, "student_id": rec.StudentID,
					"academic_year_id": rec.AcademicYearID, "date": rec.Date.String(),
					"created_at": rec.CreatedAt,
				},
			}); err != nil {
				return fmt.Errorf("attendance: upsert many: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.AttendanceRecord, error) {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc recordDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) List(ctx context.Context, tenantID string, filter ports.ListFilter) ([]*domain.AttendanceRecord, string, error) {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := bson.M{}
	if filter.StudentID != "" {
		query["student_id"] = filter.StudentID
	}
	if len(filter.StudentIDs) > 0 {
		query["student_id"] = bson.M{"$in": filter.StudentIDs}
	}
	if filter.AcademicYearID != "" {
		query["academic_year_id"] = filter.AcademicYearID
	}
	if filter.Date != "" {
		query["date"] = filter.Date
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Cursor != "" {
		query["_id"] = bson.M{"$gt": filter.Cursor}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("attendance: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.AttendanceRecord
	for cur.Next(ctx) {
		var doc recordDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("attendance: decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, rec *domain.AttendanceRecord) error {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": rec.ID}), bson.M{"$set": recordFields(rec)})
	if err != nil {
		return fmt.Errorf("attendance: update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("attendance: delete: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- lifecycle and outbox ------------------------------------------------

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("attendance: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// CommitAttendanceLifecycle commits every record and event together.
func (r *Repository) CommitAttendanceLifecycle(ctx context.Context,
	tenantID,
	mutation string,
	records []*domain.AttendanceRecord,
	eventType string,
	payloads []map[string]any) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.records.ScopeTo(tenantID)
		if err != nil {
			return err
		}

		if err := r.mutateAttendance(ctx, tenantID, mutation, records); err != nil {
			return err
		}

		for i, rec := range records {
			payload := map[string]any{}
			if i < len(payloads) {
				payload = payloads[i]
			}
			event, err := lifecycleEvent(tenantID, eventType, payload)
			if err != nil {
				return err
			}
			query := bson.M{"_id": rec.ID}
			if mutation == ports.AttendanceMutationCreate || mutation == ports.AttendanceMutationBulkUpsert {
				query = live(bson.M{"student_id": rec.StudentID, "academic_year_id": rec.AcademicYearID, "date": rec.Date.String()})
			}
			res, err := scope.UpdateWithEvents(ctx, query, bson.M{}, event)
			if err != nil {
				return fmt.Errorf("attendance: queue lifecycle event: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}
		}
		return nil
	})
}

func (r *Repository) mutateAttendance(ctx context.Context, tenantID, mutation string, records []*domain.AttendanceRecord) error {
	scope, err := r.records.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	switch mutation {
	case ports.AttendanceMutationCreate, ports.AttendanceMutationBulkUpsert:
		if err := r.UpsertMany(ctx, tenantID, records); err != nil {
			return err
		}
	case ports.AttendanceMutationUpdate:
		for _, rec := range records {
			if err := r.Update(ctx, tenantID, rec); err != nil {
				return err
			}
		}
	case ports.AttendanceMutationDelete:
		for _, rec := range records {
			res, err := scope.UpdateOne(ctx, live(bson.M{"_id": rec.ID}),
				bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
			if err != nil {
				return fmt.Errorf("attendance: delete: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}
		}
	default:
		return fmt.Errorf("attendance: unsupported lifecycle mutation %q", mutation)
	}
	return nil
}

func (r *Repository) ClaimPendingAttendanceEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	claimed, err := r.outbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("attendance: claim outbox: %w", err)
	}
	out := make([]ports.OutboxEvent, 0, len(claimed))
	for _, c := range claimed {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID,
			EventType: c.Event.Type, Payload: json.RawMessage(c.Event.Data),
		})
	}
	return out, nil
}

func (r *Repository) MarkAttendanceEventPublished(ctx context.Context, id string) error {
	if err := r.outbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("attendance: mark published: %w", err)
	}
	if _, err := r.outbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}}); err != nil {
		return err
	}
	return nil
}

func (r *Repository) MarkAttendanceEventFailed(ctx context.Context, id, reason string) error {
	if err := r.outbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("attendance: mark failed: %w", err)
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX and UNIQUE statements in the
// PostgreSQL migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	coll := store.Database().Collection(RecordCollection)
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}, {Key: "date", Value: 1}}},
		{
			// One register entry per student, year and date. deleted_at is part
			// of the key because MongoDB cannot express "where deleted_at does
			// not exist" in a partial index: live records share a missing value
			// and so stay unique among themselves, while each tombstone carries
			// its own timestamp.
			Keys: bson.D{
				{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1},
				{Key: "academic_year_id", Value: 1}, {Key: "date", Value: 1},
				{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("attendance: ensure indexes: %w", err)
	}
	return nil
}
