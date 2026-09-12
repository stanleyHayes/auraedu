// Package mongo persists students, guardians and their links in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Tenant isolation comes from platform/mongo.Scope rather than row-level
// security: a query that is not scoped cannot be written. Lifecycle events are
// embedded in the aggregate they describe, which makes the domain change and
// its event a single atomic write. Operations spanning aggregates use
// replica-set transactions to preserve their shared invariants.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	StudentCollection         = "students"
	EnrollmentCollection      = "enrollments"
	GuardianCollection        = "guardians"
	StudentGuardianCollection = "student_guardians"
	outboxLease               = 5 * time.Minute
	eventSource               = "student-service"
)

type Repository struct {
	store            *pmongo.Store
	enrollmentOutbox *pmongo.ClaimableOutbox
	students         *pmongo.Collection
	enrollments      *pmongo.Collection
	guardians        *pmongo.Collection
	links            *pmongo.Collection

	studentOutbox  *pmongo.ClaimableOutbox
	guardianOutbox *pmongo.ClaimableOutbox
	linkOutbox     *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	students := store.Collection(StudentCollection)
	guardians := store.Collection(GuardianCollection)
	links := store.Collection(StudentGuardianCollection)
	return &Repository{
		store: store, enrollmentOutbox: pmongo.NewClaimableOutbox(store.Collection(EnrollmentCollection), outboxLease),
		students: students, enrollments: store.Collection(EnrollmentCollection),
		guardians: guardians, links: links,
		studentOutbox:  pmongo.NewClaimableOutbox(students, outboxLease),
		guardianOutbox: pmongo.NewClaimableOutbox(guardians, outboxLease),
		linkOutbox:     pmongo.NewClaimableOutbox(links, outboxLease),
	}
}

// live excludes tombstoned records. A delete keeps its document until its event
// has been published, so without this every read would keep returning records
// the Postgres adapter has already removed.
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

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("student: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// ---- students ------------------------------------------------------------

type studentDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	FirstName      string    `bson:"first_name"`
	LastName       string    `bson:"last_name"`
	StudentCode    string    `bson:"student_code"`
	DateOfBirth    *string   `bson:"date_of_birth,omitempty"`
	Gender         *string   `bson:"gender,omitempty"`
	Status         string    `bson:"status"`
	ClassID        *string   `bson:"class_id,omitempty"`
	AcademicYearID *string   `bson:"academic_year_id,omitempty"`
	UserID         *string   `bson:"user_id,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d studentDoc) toDomain() *domain.Student {
	return &domain.Student{
		ID: d.ID, TenantID: d.TenantID, FirstName: d.FirstName, LastName: d.LastName,
		StudentCode: d.StudentCode, DateOfBirth: d.DateOfBirth, Gender: d.Gender,
		Status: d.Status, ClassID: d.ClassID, AcademicYearID: d.AcademicYearID,
		UserID: d.UserID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func studentFields(s *domain.Student) bson.M {
	return bson.M{
		"first_name": s.FirstName, "last_name": s.LastName, "student_code": s.StudentCode,
		"date_of_birth": s.DateOfBirth, "gender": s.Gender, "status": s.Status,
		"class_id": s.ClassID, "academic_year_id": s.AcademicYearID, "user_id": s.UserID,
		"created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}

func (r *Repository) create(ctx context.Context, tenantID string, s *domain.Student) error {
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := studentFields(s)
	doc["_id"] = s.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("student: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.Student, error) {
	return r.student(ctx, tenantID, bson.M{"_id": id})
}

func (r *Repository) GetStudentByUserID(ctx context.Context, tenantID, userID string) (*domain.Student, error) {
	return r.student(ctx, tenantID, bson.M{"user_id": userID})
}

func (r *Repository) student(ctx context.Context, tenantID string, query bson.M) (*domain.Student, error) {
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc studentDoc
	if err := scope.FindOne(ctx, live(query)).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) List(ctx context.Context, tenantID string, classID *string, limit int, cursor string) ([]*domain.Student, string, error) {
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	query := bson.M{}
	if classID != nil {
		query["class_id"] = *classID
	}
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("student: list: %w", err)
	}
	out, err := decodeStudents(ctx, cur)
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func decodeStudents(ctx context.Context, cur *mongo.Cursor) ([]*domain.Student, error) {
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.Student
	for cur.Next(ctx) {
		var doc studentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("student: decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, cur.Err()
}

func (r *Repository) ListStudentIDsByClassIDs(ctx context.Context, tenantID string, classIDs []string) ([]string, error) {
	if len(classIDs) == 0 {
		return []string{}, nil
	}
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, live(bson.M{"class_id": bson.M{"$in": classIDs}}),
		options.Find().SetProjection(bson.M{"_id": 1}).SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("student: list ids by class: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	out := []string{}
	for cur.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("student: decode id: %w", err)
		}
		out = append(out, doc.ID)
	}
	return out, cur.Err()
}

func (r *Repository) Update(ctx context.Context, tenantID string, s *domain.Student) error {
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": s.ID}), bson.M{"$set": studentFields(s)})
	if err != nil {
		return fmt.Errorf("student: update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("student: delete: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- enrollments ---------------------------------------------------------

type enrollmentDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	StudentID      string    `bson:"student_id"`
	ClassID        string    `bson:"class_id"`
	AcademicYearID string    `bson:"academic_year_id"`
	EnrolledAt     time.Time `bson:"enrolled_at"`
}

func (d enrollmentDoc) toDomain() *domain.Enrollment {
	return &domain.Enrollment{
		ID: d.ID, TenantID: d.TenantID, StudentID: d.StudentID, ClassID: d.ClassID,
		AcademicYearID: d.AcademicYearID, EnrolledAt: d.EnrolledAt,
	}
}

func (r *Repository) createEnrollment(ctx context.Context, tenantID string, e *domain.Enrollment) error {
	scope, err := r.enrollments.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	if _, err := scope.InsertOne(ctx, bson.M{
		"_id": e.ID, "student_id": e.StudentID, "class_id": e.ClassID,
		"academic_year_id": e.AcademicYearID, "enrolled_at": e.EnrolledAt,
	}); err != nil {
		return fmt.Errorf("student: create enrollment: %w", err)
	}
	return nil
}

func (r *Repository) ListEnrollments(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Enrollment, string, error) {
	scope, err := r.enrollments.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	query := live(bson.M{"student_id": studentID})
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	cur, err := scope.Find(ctx, query,
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("student: list enrollments: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	out := make([]*domain.Enrollment, 0, limit)
	for cur.Next(ctx) {
		var doc enrollmentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("student: decode enrollment: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// ---- guardians -----------------------------------------------------------

type guardianDoc struct {
	ID           string    `bson:"_id"`
	TenantID     string    `bson:"tenant_id"`
	FirstName    string    `bson:"first_name"`
	LastName     string    `bson:"last_name"`
	Relationship string    `bson:"relationship"`
	Phone        *string   `bson:"phone,omitempty"`
	Email        *string   `bson:"email,omitempty"`
	UserID       *string   `bson:"user_id,omitempty"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func (d guardianDoc) toDomain() *domain.Guardian {
	return &domain.Guardian{
		ID: d.ID, TenantID: d.TenantID, FirstName: d.FirstName, LastName: d.LastName,
		Relationship: d.Relationship, Phone: d.Phone, Email: d.Email, UserID: d.UserID,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func guardianFields(g *domain.Guardian) bson.M {
	return bson.M{
		"first_name": g.FirstName, "last_name": g.LastName, "relationship": g.Relationship,
		"phone": g.Phone, "email": g.Email, "user_id": g.UserID,
		"created_at": g.CreatedAt, "updated_at": g.UpdatedAt,
	}
}

func (r *Repository) CreateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error {
	scope, err := r.guardians.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := guardianFields(g)
	doc["_id"] = g.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("student: create guardian: %w", err)
	}
	return nil
}

func (r *Repository) GetGuardianByID(ctx context.Context, tenantID, id string) (*domain.Guardian, error) {
	return r.guardian(ctx, tenantID, bson.M{"_id": id})
}

func (r *Repository) GetGuardianByUserID(ctx context.Context, tenantID, userID string) (*domain.Guardian, error) {
	return r.guardian(ctx, tenantID, bson.M{"user_id": userID})
}

func (r *Repository) guardian(ctx context.Context, tenantID string, query bson.M) (*domain.Guardian, error) {
	scope, err := r.guardians.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc guardianDoc
	if err := scope.FindOne(ctx, live(query)).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) UpdateGuardian(ctx context.Context, tenantID string, g *domain.Guardian) error {
	scope, err := r.guardians.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": g.ID}), bson.M{"$set": guardianFields(g)})
	if err != nil {
		return fmt.Errorf("student: update guardian: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) deleteGuardian(ctx context.Context, tenantID, id string) error {
	scope, err := r.guardians.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("student: delete guardian: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- student to guardian links -------------------------------------------

type linkDoc struct {
	ID           string    `bson:"_id"`
	TenantID     string    `bson:"tenant_id"`
	StudentID    string    `bson:"student_id"`
	GuardianID   string    `bson:"guardian_id"`
	Relationship *string   `bson:"relationship,omitempty"`
	IsPrimary    bool      `bson:"is_primary"`
	CreatedAt    time.Time `bson:"created_at"`
}

func (r *Repository) linkGuardianToStudent(ctx context.Context, tenantID string, link *domain.StudentGuardian) error {
	scope, err := r.links.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	if _, err := scope.InsertOne(ctx, bson.M{
		"_id": link.ID, "student_id": link.StudentID, "guardian_id": link.GuardianID,
		"relationship": link.Relationship, "is_primary": link.IsPrimary,
		"created_at": link.CreatedAt,
	}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("student: link guardian: %w", err)
	}
	return nil
}

func (r *Repository) UnlinkGuardianFromStudent(ctx context.Context, tenantID, studentID, guardianID string) error {
	scope, err := r.links.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, bson.M{"student_id": studentID, "guardian_id": guardianID})
	if err != nil {
		return fmt.Errorf("student: unlink guardian: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ListStudentsByGuardian resolves the link table then reads the students.
// PostgreSQL does this with a JOIN; MongoDB has no cross-collection join here,
// and doing it in two scoped reads keeps both sides inside the tenant filter.
func (r *Repository) ListStudentsByGuardian(ctx context.Context, tenantID, guardianID string) ([]*domain.Student, error) {
	ids, err := r.linkedIDs(ctx, tenantID, bson.M{"guardian_id": guardianID}, "student_id")
	if err != nil || len(ids) == 0 {
		return []*domain.Student{}, err
	}
	scope, err := r.students.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, live(bson.M{"_id": bson.M{"$in": ids}}),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("student: list by guardian: %w", err)
	}
	return decodeStudents(ctx, cur)
}

func (r *Repository) ListGuardiansByStudent(ctx context.Context, tenantID, studentID string, limit int, cursor string) ([]*domain.Guardian, string, error) {
	ids, err := r.linkedIDs(ctx, tenantID, bson.M{"student_id": studentID}, "guardian_id")
	if err != nil {
		return nil, "", err
	}
	if len(ids) == 0 {
		return []*domain.Guardian{}, "", nil
	}
	scope, err := r.guardians.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	query := bson.M{"_id": bson.M{"$in": ids}}
	if cursor != "" {
		query["_id"] = bson.M{"$in": ids, "$gt": cursor}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("student: list guardians: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	out := make([]*domain.Guardian, 0, limit)
	for cur.Next(ctx) {
		var doc guardianDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("student: decode guardian: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) linkedIDs(ctx context.Context, tenantID string, query bson.M, field string) ([]string, error) {
	scope, err := r.links.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, live(query), options.Find().SetProjection(bson.M{field: 1}))
	if err != nil {
		return nil, fmt.Errorf("student: list links: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var ids []string
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("student: decode link: %w", err)
		}
		if value, ok := doc[field].(string); ok && value != "" {
			ids = append(ids, value)
		}
	}
	return ids, cur.Err()
}

// ---- lifecycle -----------------------------------------------------------

// CommitStudentLifecycle applies the mutation and queues its event in one
// atomic single-document write on the aggregate the mutation touches.
//
// Deletes are the exception: removing the document would remove the queued event
// with it, so the record is tombstoned and cleared once the event is published.
//
// Multi-document mutations run in a replica-set transaction. Enrollment history,
// current class projection, guardian links and their embedded events commit together.
func (r *Repository) commitStudentLifecycle(ctx context.Context,
	tenantID string, mutation ports.LifecycleMutation, eventType string,
	payload map[string]any) error {
	event, err := lifecycleEvent(tenantID, eventType, payload)
	if err != nil {
		return err
	}

	switch mutation.Kind {
	case ports.MutationStudentCreate:
		return r.createStudentWithEvent(ctx, tenantID, mutation.Student, event)
	case ports.MutationStudentUpdate:
		return r.updateWithEvent(ctx, r.students, tenantID, mutation.Student.ID,
			bson.M{"$set": studentFields(mutation.Student)}, event)
	case ports.MutationStudentDelete:
		return r.tombstoneWithEvent(ctx, r.students, tenantID, mutation.Student.ID, event)
	case ports.MutationEnrollmentCreate:
		if err := r.CreateEnrollment(ctx, tenantID, mutation.Enrollment); err != nil {
			return err
		}
		return r.queueOn(ctx, r.enrollments, tenantID, mutation.Enrollment.ID, event)
	case ports.MutationGuardianCreate:
		return r.insertWithEvent(ctx, r.guardians, tenantID, mutation.Guardian.ID,
			guardianFields(mutation.Guardian), event)
	case ports.MutationGuardianUpdate:
		return r.updateWithEvent(ctx, r.guardians, tenantID, mutation.Guardian.ID,
			bson.M{"$set": guardianFields(mutation.Guardian)}, event)
	case ports.MutationGuardianDelete:
		return r.tombstoneWithEvent(ctx, r.guardians, tenantID, mutation.Guardian.ID, event)
	case ports.MutationGuardianLink:
		if err := r.LinkGuardianToStudent(ctx, tenantID, mutation.Link); err != nil {
			return err
		}
		return r.queueOn(ctx, r.links, tenantID, mutation.Link.ID, event)
	case ports.MutationGuardianUnlink:
		return r.unlinkWithEvent(ctx, tenantID, mutation.StudentID, mutation.GuardianID, event)
	default:
		return fmt.Errorf("student: unsupported lifecycle mutation %q", mutation.Kind)
	}
}

func (r *Repository) createStudentWithEvent(ctx context.Context, tenantID string, s *domain.Student, event tenancy.CloudEvent) error {
	if err := r.insertWithEvent(ctx, r.students, tenantID, s.ID, studentFields(s), event); err != nil {
		return err
	}
	if s.ClassID == nil || s.AcademicYearID == nil {
		return nil
	}
	enrollment, err := domain.NewEnrollment(tenantID, s.ID, *s.ClassID, *s.AcademicYearID, s.CreatedAt)
	if err != nil {
		return err
	}
	return r.CreateEnrollment(ctx, tenantID, enrollment)
}

func (r *Repository) insertWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, fields bson.M, event tenancy.CloudEvent) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := bson.M{}
	for k, v := range fields {
		doc[k] = v
	}
	doc["_id"] = id
	if _, err := scope.InsertWithEvents(ctx, doc, event); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("student: lifecycle insert: %w", err)
	}
	return nil
}

func (r *Repository) updateWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, update bson.M, event tenancy.CloudEvent) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": id}), update, event)
	if err != nil {
		return fmt.Errorf("student: lifecycle update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) tombstoneWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, event tenancy.CloudEvent) error {
	return r.updateWithEvent(ctx, coll, tenantID, id,
		bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
}

// queueOn attaches an event to a document that was just written, for the two
// mutations whose aggregate is created by an ordinary insert.
func (r *Repository) queueOn(ctx context.Context, coll *pmongo.Collection, tenantID, id string, event tenancy.CloudEvent) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": id}), bson.M{}, event)
	if err != nil {
		return fmt.Errorf("student: queue lifecycle event: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// unlinkWithEvent tombstones the link so its event survives to be published.
func (r *Repository) unlinkWithEvent(ctx context.Context, tenantID, studentID, guardianID string, event tenancy.CloudEvent) error {
	scope, err := r.links.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateWithEvents(ctx,
		live(bson.M{"student_id": studentID, "guardian_id": guardianID}),
		bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
	if err != nil {
		return fmt.Errorf("student: unlink guardian: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- outbox --------------------------------------------------------------

func claimedEvents(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID,
			EventType: c.Event.Type, Payload: json.RawMessage(c.Event.Data),
		})
	}
	return out
}

func (r *Repository) ClaimPendingStudentEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := []ports.OutboxEvent{}
	for _, outbox := range []*pmongo.ClaimableOutbox{r.studentOutbox, r.enrollmentOutbox, r.guardianOutbox, r.linkOutbox} {
		if len(out) >= limit {
			break
		}
		claimed, err := outbox.Claim(ctx, limit-len(out))
		if err != nil {
			return nil, fmt.Errorf("student: claim outbox: %w", err)
		}
		out = append(out, claimedEvents(claimed)...)
	}
	return out, nil
}

// MarkStudentEventPublished acknowledges an event from any of the collections
// that queue them. The port carries only the event id, and acknowledging one
// that is already gone is normal for an at-least-once outbox.
func (r *Repository) MarkStudentEventPublished(ctx context.Context, id string) error {
	for _, outbox := range []*pmongo.ClaimableOutbox{r.studentOutbox, r.enrollmentOutbox, r.guardianOutbox, r.linkOutbox} {
		if err := outbox.MarkPublishedByEventID(ctx, id); err != nil {
			return fmt.Errorf("student: mark published: %w", err)
		}
		// Tombstoned records are held until their delete event is delivered.
		if _, err := outbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) MarkStudentEventFailed(ctx context.Context, id, reason string) error {
	for _, outbox := range []*pmongo.ClaimableOutbox{r.studentOutbox, r.enrollmentOutbox, r.guardianOutbox, r.linkOutbox} {
		if err := outbox.MarkFailedByEventID(ctx, id, reason); err != nil {
			return fmt.Errorf("student: mark failed: %w", err)
		}
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX and UNIQUE statements in the
// PostgreSQL migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]mongo.IndexModel{
		StudentCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "class_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id",
				Value: 1}}, Options: options.Index().SetUnique(true).SetName("tenant_user_unique").
				SetPartialFilterExpression(bson.M{"user_id": bson.M{"$type": "string"}})},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_code", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		EnrollmentCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}, {Key: "academic_year_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}, {Key: "_id", Value: 1}}},
		},
		GuardianCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id",
				Value: 1}}, Options: options.Index().SetUnique(true).SetName("tenant_user_unique").
				SetPartialFilterExpression(bson.M{"user_id": bson.M{"$type": "string"}})},
		},
		StudentGuardianCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}, {Key: "guardian_id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "guardian_id", Value: 1}}},
		},
	}
	for name, models := range specs {
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("student: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, t string, s *domain.Student) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.create(ctx, t, s); err != nil {
			return err
		}
		if s.ClassID != nil && s.AcademicYearID != nil {
			e, err := domain.NewEnrollment(t, s.ID, *s.ClassID, *s.AcademicYearID, s.CreatedAt)
			if err != nil {
				return err
			}
			return r.createEnrollmentAndProject(ctx, t, e)
		}
		return nil
	})
}
func (r *Repository) CreateEnrollment(ctx context.Context, t string, e *domain.Enrollment) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error { return r.createEnrollmentAndProject(ctx, t, e) })
}
func (r *Repository) createEnrollmentAndProject(ctx context.Context, t string, e *domain.Enrollment) error {
	if err := r.fence(ctx, r.students, t, e.StudentID); err != nil {
		return err
	}
	if err := r.createEnrollment(ctx, t, e); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return err
	}
	scope, err := r.students.ScopeTo(t)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": e.StudentID}),
		bson.M{"$set": bson.M{"class_id": e.ClassID, "academic_year_id": e.AcademicYearID,
			"updated_at": e.EnrolledAt}})
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}
func (r *Repository) fence(ctx context.Context, c *pmongo.Collection, t, id string) error {
	scope, err := c.ScopeTo(t)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$inc": bson.M{"_reference_version": 1}})
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}
func (r *Repository) LinkGuardianToStudent(ctx context.Context, t string, l *domain.StudentGuardian) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.fence(ctx, r.students, t, l.StudentID); err != nil {
			return err
		}
		if err := r.fence(ctx, r.guardians, t, l.GuardianID); err != nil {
			return err
		}
		return r.linkGuardianToStudent(ctx, t, l)
	})
}
func (r *Repository) cascade(ctx context.Context, t, id string, student bool) error {
	coll := r.links
	key := "guardian_id"
	if student {
		key = "student_id"
		scope, err := r.enrollments.ScopeTo(t)
		if err != nil {
			return err
		}
		if _, err = scope.UpdateMany(ctx, bson.M{key: id}, bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}); err != nil {
			return err
		}
	}
	scope, err := coll.ScopeTo(t)
	if err != nil {
		return err
	}
	_, err = scope.UpdateMany(ctx, bson.M{key: id}, bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
	return err
}
func (r *Repository) Delete(ctx context.Context, t, id string) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.delete(ctx, t, id); err != nil {
			return err
		}
		return r.cascade(ctx, t, id, true)
	})
}
func (r *Repository) DeleteGuardian(ctx context.Context, t, id string) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.deleteGuardian(ctx, t, id); err != nil {
			return err
		}
		return r.cascade(ctx, t, id, false)
	})
}
func (r *Repository) CommitStudentLifecycle(ctx context.Context, t string, m ports.LifecycleMutation, k string, p map[string]any) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.commitStudentLifecycle(ctx, t, m, k, p); err != nil {
			return err
		}
		if m.Kind == ports.MutationStudentDelete {
			return r.cascade(ctx, t, m.Student.ID, true)
		}
		if m.Kind == ports.MutationGuardianDelete {
			return r.cascade(ctx, t, m.Guardian.ID, false)
		}
		return nil
	})
}
