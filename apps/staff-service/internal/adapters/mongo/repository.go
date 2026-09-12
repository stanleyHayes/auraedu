// Package mongo persists staff aggregates in MongoDB.
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
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	StaffCollection      = "staff"
	AssignmentCollection = "staff_assignments"
	outboxLease          = 5 * time.Minute
	eventSource          = "staff-service"
)

type Repository struct {
	staff        *pmongo.Collection
	assignments  *pmongo.Collection
	staffOutbox  *pmongo.ClaimableOutbox
	assignOutbox *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository           = (*Repository)(nil)
	_ ports.LifecycleRepository  = (*Repository)(nil)
	_ ports.OutboxRepository     = (*Repository)(nil)
	_ ports.AssignmentRepository = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	staff := store.Collection(StaffCollection)
	assignments := store.Collection(AssignmentCollection)
	return &Repository{
		staff:        staff,
		assignments:  assignments,
		staffOutbox:  pmongo.NewClaimableOutbox(staff, outboxLease),
		assignOutbox: pmongo.NewClaimableOutbox(assignments, outboxLease),
	}
}

// staffDoc mirrors the staff table so both adapters describe the same record.
type staffDoc struct {
	ID        string    `bson:"_id"`
	TenantID  string    `bson:"tenant_id"`
	FirstName string    `bson:"first_name"`
	LastName  string    `bson:"last_name"`
	StaffType string    `bson:"staff_type"`
	Email     *string   `bson:"email,omitempty"`
	UserID    *string   `bson:"user_id,omitempty"`
	StaffCode string    `bson:"staff_code"`
	Status    string    `bson:"status"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (d staffDoc) toDomain() *domain.Staff {
	return &domain.Staff{
		ID: d.ID, TenantID: d.TenantID, FirstName: d.FirstName, LastName: d.LastName,
		StaffType: d.StaffType, Email: d.Email, UserID: d.UserID, StaffCode: d.StaffCode,
		Status: d.Status, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// live excludes tombstoned records. A delete keeps its document until the
// delete event has been published, so without this every read would keep
// returning records the Postgres adapter has already removed.
func live(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	merged["deleted_at"] = bson.M{"$exists": false}
	return merged
}

func staffFields(s *domain.Staff) bson.M {
	return bson.M{
		"first_name": s.FirstName, "last_name": s.LastName, "staff_type": s.StaffType,
		"email": s.Email, "user_id": s.UserID, "staff_code": s.StaffCode,
		"status": s.Status, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}

func (r *Repository) Create(ctx context.Context, tenantID string, s *domain.Staff) error {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: create: %w", err)
	}
	doc := staffFields(s)
	doc["_id"] = s.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("staff: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Staff, error) {
	return r.getOne(ctx, tenantID, bson.M{"_id": id})
}

func (r *Repository) GetByUserID(ctx context.Context, tenantID, userID string) (*domain.Staff, error) {
	return r.getOne(ctx, tenantID, bson.M{"user_id": userID})
}

func (r *Repository) getOne(ctx context.Context, tenantID string, query bson.M) (*domain.Staff, error) {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("staff: get: %w", err)
	}
	var doc staffDoc
	if err := scope.FindOne(ctx, live(query)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("staff: get: %w", err)
	}
	return doc.toDomain(), nil
}

// List pages by id ascending, matching the Postgres adapter's keyset order.
func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Staff, string, error) {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("staff: list: %w", err)
	}
	query := bson.M{}
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("staff: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Staff
	for cur.Next(ctx) {
		var doc staffDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("staff: list decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("staff: list rows: %w", err)
	}
	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) Update(ctx context.Context, tenantID string, s *domain.Staff) error {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: update: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": s.ID}), bson.M{"$set": staffFields(s)})
	if err != nil {
		return fmt.Errorf("staff: update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: delete: %w", err)
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("staff: delete: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("staff: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// CommitStaffLifecycle applies the mutation and queues its event in one write.
//
// A delete is the exception: removing the document would remove the event with
// it, so the record is tombstoned instead and the relay clears it once the
// event is published.
func (r *Repository) CommitStaffLifecycle(ctx context.Context, tenantID string, staff *domain.Staff, mutation, eventType string, payload map[string]any) error {
	scope, err := r.staff.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: lifecycle: %w", err)
	}
	event, err := lifecycleEvent(tenantID, eventType, payload)
	if err != nil {
		return err
	}

	switch mutation {
	case ports.StaffMutationCreate:
		doc := staffFields(staff)
		doc["_id"] = staff.ID
		if _, err := scope.InsertWithEvents(ctx, doc, event); err != nil {
			return fmt.Errorf("staff: create: %w", err)
		}
	case ports.StaffMutationUpdate:
		res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": staff.ID}),
			bson.M{"$set": staffFields(staff)}, event)
		if err != nil {
			return fmt.Errorf("staff: update: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	case ports.StaffMutationDelete:
		res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": staff.ID, "deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
		if err != nil {
			return fmt.Errorf("staff: delete: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	default:
		return fmt.Errorf("staff: unsupported lifecycle mutation %q", mutation)
	}
	return nil
}

func claimed(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID,
			EventType: c.Event.Type, Payload: json.RawMessage(c.Event.Data),
		})
	}
	return out
}

func (r *Repository) ClaimPendingStaffEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	fromStaff, err := r.staffOutbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("staff: claim outbox: %w", err)
	}
	out := claimed(fromStaff)
	if len(out) >= limit {
		return out, nil
	}
	fromAssignments, err := r.assignOutbox.Claim(ctx, limit-len(out))
	if err != nil {
		return nil, fmt.Errorf("staff: claim assignment outbox: %w", err)
	}
	return append(out, claimed(fromAssignments)...), nil
}

// MarkStaffEventPublished acknowledges an event from either collection. The port
// carries only the event id, so both are asked; acknowledging an event that is
// already gone is normal for an at-least-once outbox.
func (r *Repository) MarkStaffEventPublished(ctx context.Context, id string) error {
	if err := r.staffOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("staff: mark published: %w", err)
	}
	if err := r.assignOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("staff: mark assignment published: %w", err)
	}
	return r.clearPublishedTombstones(ctx)
}

func (r *Repository) MarkStaffEventFailed(ctx context.Context, id, reason string) error {
	if err := r.staffOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("staff: mark failed: %w", err)
	}
	if err := r.assignOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("staff: mark assignment failed: %w", err)
	}
	return nil
}

// clearPublishedTombstones removes deleted records whose delete event has been
// delivered. Holding the tombstone until then is what lets a delete stay atomic
// with its event.
func (r *Repository) clearPublishedTombstones(ctx context.Context) error {
	_, err := r.staffOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("staff: clear tombstones: %w", err)
	}
	return nil
}

func (r *Repository) CreateAssignment(ctx context.Context, tenantID string, assignment *domain.Assignment, payload map[string]any) error {
	scope, err := r.assignments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: create assignment: %w", err)
	}
	event, err := lifecycleEvent(tenantID, "staff.assigned.v1", payload)
	if err != nil {
		return err
	}
	doc := bson.M{
		"_id": assignment.ID, "staff_id": assignment.StaffID, "class_id": assignment.ClassID,
		"subject_id": assignment.SubjectID, "role": assignment.Role, "assigned_at": assignment.AssignedAt,
	}
	if _, err := scope.InsertWithEvents(ctx, doc, event); err != nil {
		return fmt.Errorf("staff: create assignment: %w", err)
	}
	return nil
}

type assignmentDoc struct {
	ID         string    `bson:"_id"`
	TenantID   string    `bson:"tenant_id"`
	StaffID    string    `bson:"staff_id"`
	ClassID    string    `bson:"class_id"`
	SubjectID  *string   `bson:"subject_id,omitempty"`
	Role       *string   `bson:"role,omitempty"`
	AssignedAt time.Time `bson:"assigned_at"`
}

func (d assignmentDoc) toDomain() *domain.Assignment {
	return &domain.Assignment{
		ID: d.ID, TenantID: d.TenantID, StaffID: d.StaffID, ClassID: d.ClassID,
		SubjectID: d.SubjectID, Role: d.Role, AssignedAt: d.AssignedAt,
	}
}

func (r *Repository) ListAssignments(ctx context.Context, tenantID, staffID string, limit int, cursor string) ([]*domain.Assignment, string, error) {
	scope, err := r.assignments.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("staff: list assignments: %w", err)
	}
	query := bson.M{"staff_id": staffID}
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	cur, err := scope.Find(ctx, query,
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("staff: list assignments: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	items := make([]*domain.Assignment, 0, limit)
	for cur.Next(ctx) {
		var doc assignmentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("staff: assignment decode: %w", err)
		}
		items = append(items, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("staff: assignment rows: %w", err)
	}
	var next string
	if len(items) == limit && len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return items, next, nil
}

func (r *Repository) DeleteAssignment(ctx context.Context, tenantID, staffID, id string) error {
	scope, err := r.assignments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("staff: delete assignment: %w", err)
	}
	res, err := scope.DeleteOne(ctx, bson.M{"_id": id, "staff_id": staffID})
	if err != nil {
		return fmt.Errorf("staff: delete assignment: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ListAssignmentClassIDs(ctx context.Context, tenantID, staffID string) ([]string, error) {
	return r.distinctAssignmentField(ctx, tenantID, staffID, "class_id")
}

func (r *Repository) ListAssignmentSubjectIDs(ctx context.Context, tenantID, staffID string) ([]string, error) {
	return r.distinctAssignmentField(ctx, tenantID, staffID, "subject_id")
}

func (r *Repository) distinctAssignmentField(ctx context.Context, tenantID, staffID, field string) ([]string, error) {
	scope, err := r.assignments.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("staff: assignment scope: %w", err)
	}
	cur, err := scope.Find(ctx, bson.M{"staff_id": staffID, field: bson.M{"$ne": nil}},
		options.Find().SetProjection(bson.M{field: 1}).SetSort(bson.D{{Key: field, Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("staff: list %s: %w", field, err)
	}
	defer func() { _ = cur.Close(ctx) }()

	seen := map[string]bool{}
	out := []string{}
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("staff: decode %s: %w", field, err)
		}
		value, ok := doc[field].(string)
		if !ok || value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("staff: %s rows: %w", field, err)
	}
	return out, nil
}

// EnsureIndexes replaces the CREATE INDEX statements in the Postgres migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]bson.D{
		StaffCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "staff_code", Value: 1}},
		},
		AssignmentCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "staff_id", Value: 1}, {Key: "_id", Value: 1}},
		},
	}
	for name, keys := range specs {
		models := make([]mongo.IndexModel, 0, len(keys))
		for _, k := range keys {
			models = append(models, mongo.IndexModel{Keys: k})
		}
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("staff: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}
