// Package mongo persists academic years, terms, classes, subjects, grading
// scales and timetables in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config. It
// mirrors that adapter's shape, one repository type per aggregate.
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

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	YearCollection      = "academic_years"
	TermCollection      = "terms"
	ClassCollection     = "classes"
	SubjectCollection   = "subjects"
	GradingCollection   = "grading_scales"
	TimetableCollection = "timetable_entries"
	outboxLease         = 5 * time.Minute
	eventSource         = "academic-service"
)

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
		return tenancy.CloudEvent{}, fmt.Errorf("academic: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// page runs the keyset query every aggregate here shares: tenant-scoped,
// ordered by id, with the cursor being the last id seen.
func page(ctx context.Context, coll *pmongo.Collection, tenantID string, query bson.M, limit int, cursor string) (*mongo.Cursor, error) {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	if cursor != "" {
		query["_id"] = bson.M{"$gt": cursor}
	}
	return scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
}

func dateOf(v string) domain.Date {
	d, err := domain.NewDate(v)
	if err != nil {
		return domain.Date{}
	}
	return d
}

// ---- academic years (also the lifecycle and outbox owner) ----------------

type Repository struct {
	years      *pmongo.Collection
	terms      *pmongo.Collection
	classes    *pmongo.Collection
	subjects   *pmongo.Collection
	outboxes   []*pmongo.ClaimableOutbox
	yearOut    *pmongo.ClaimableOutbox
	termOut    *pmongo.ClaimableOutbox
	classOut   *pmongo.ClaimableOutbox
	subjectOut *pmongo.ClaimableOutbox
}

var (
	_ ports.AcademicYearRepository = (*Repository)(nil)
	_ ports.LifecycleRepository    = (*Repository)(nil)
	_ ports.OutboxRepository       = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	years := store.Collection(YearCollection)
	terms := store.Collection(TermCollection)
	classes := store.Collection(ClassCollection)
	subjects := store.Collection(SubjectCollection)
	r := &Repository{
		years: years, terms: terms, classes: classes, subjects: subjects,
		yearOut:    pmongo.NewClaimableOutbox(years, outboxLease),
		termOut:    pmongo.NewClaimableOutbox(terms, outboxLease),
		classOut:   pmongo.NewClaimableOutbox(classes, outboxLease),
		subjectOut: pmongo.NewClaimableOutbox(subjects, outboxLease),
	}
	r.outboxes = []*pmongo.ClaimableOutbox{r.yearOut, r.termOut, r.classOut, r.subjectOut}
	return r
}

type yearDoc struct {
	ID        string    `bson:"_id"`
	TenantID  string    `bson:"tenant_id"`
	Name      string    `bson:"name"`
	Code      string    `bson:"code"`
	StartDate string    `bson:"start_date"`
	EndDate   string    `bson:"end_date"`
	Status    string    `bson:"status"`
	IsCurrent bool      `bson:"is_current"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (d yearDoc) toDomain() *domain.AcademicYear {
	return &domain.AcademicYear{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Code: d.Code,
		StartDate: dateOf(d.StartDate), EndDate: dateOf(d.EndDate),
		Status: d.Status, IsCurrent: d.IsCurrent,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func yearFields(y *domain.AcademicYear) bson.M {
	return bson.M{
		"name": y.Name, "code": y.Code, "start_date": y.StartDate.String(),
		"end_date": y.EndDate.String(), "status": y.Status, "is_current": y.IsCurrent,
		"created_at": y.CreatedAt, "updated_at": y.UpdatedAt,
	}
}

func (r *Repository) Create(ctx context.Context, tenantID string, y *domain.AcademicYear) error {
	return insert(ctx, r.years, tenantID, y.ID, yearFields(y), "academic: create year")
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.AcademicYear, error) {
	scope, err := r.years.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc yearDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AcademicYear, string, error) {
	cur, err := page(ctx, r.years, tenantID, bson.M{}, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("academic: list years: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.AcademicYear
	for cur.Next(ctx) {
		var doc yearDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("academic: decode year: %w", err)
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

func (r *Repository) Update(ctx context.Context, tenantID string, y *domain.AcademicYear) error {
	return update(ctx, r.years, tenantID, y.ID, yearFields(y), "academic: update year")
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.years, tenantID, id, "academic: delete year")
}

// ---- shared CRUD helpers -------------------------------------------------

func insert(ctx context.Context, coll *pmongo.Collection, tenantID, id string, fields bson.M, what string) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := bson.M{}
	for k, v := range fields {
		doc[k] = v
	}
	doc["_id"] = id
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

func update(ctx context.Context, coll *pmongo.Collection, tenantID, id string, fields bson.M, what string) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$set": fields})
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func remove(ctx context.Context, coll *pmongo.Collection, tenantID, id, what string) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- terms ---------------------------------------------------------------

type TermRepository struct{ terms *pmongo.Collection }

var _ ports.TermRepository = (*TermRepository)(nil)

func NewTermRepository(store *pmongo.Store) *TermRepository {
	return &TermRepository{terms: store.Collection(TermCollection)}
}

type termDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	AcademicYearID string    `bson:"academic_year_id"`
	Name           string    `bson:"name"`
	StartDate      string    `bson:"start_date"`
	EndDate        string    `bson:"end_date"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d termDoc) toDomain() *domain.Term {
	return &domain.Term{
		ID: d.ID, TenantID: d.TenantID, AcademicYearID: d.AcademicYearID, Name: d.Name,
		StartDate: dateOf(d.StartDate), EndDate: dateOf(d.EndDate),
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func termFields(t *domain.Term) bson.M {
	return bson.M{
		"academic_year_id": t.AcademicYearID, "name": t.Name,
		"start_date": t.StartDate.String(), "end_date": t.EndDate.String(),
		"created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
	}
}

func (r *TermRepository) Create(ctx context.Context, tenantID string, t *domain.Term) error {
	return insert(ctx, r.terms, tenantID, t.ID, termFields(t), "academic: create term")
}

func (r *TermRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Term, error) {
	scope, err := r.terms.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc termDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *TermRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Term, string, error) {
	cur, err := page(ctx, r.terms, tenantID, bson.M{}, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("academic: list terms: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.Term
	for cur.Next(ctx) {
		var doc termDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("academic: decode term: %w", err)
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

func (r *TermRepository) Update(ctx context.Context, tenantID string, t *domain.Term) error {
	return update(ctx, r.terms, tenantID, t.ID, termFields(t), "academic: update term")
}

func (r *TermRepository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.terms, tenantID, id, "academic: delete term")
}

// ---- classes -------------------------------------------------------------

type ClassRepository struct{ classes *pmongo.Collection }

var _ ports.ClassRepository = (*ClassRepository)(nil)

func NewClassRepository(store *pmongo.Store) *ClassRepository {
	return &ClassRepository{classes: store.Collection(ClassCollection)}
}

type classDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	Name           string    `bson:"name"`
	AcademicYearID string    `bson:"academic_year_id"`
	ClassTeacherID *string   `bson:"class_teacher_id,omitempty"`
	Capacity       *int      `bson:"capacity,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d classDoc) toDomain() *domain.Class {
	return &domain.Class{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, AcademicYearID: d.AcademicYearID,
		ClassTeacherID: d.ClassTeacherID, Capacity: d.Capacity,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func classFields(c *domain.Class) bson.M {
	return bson.M{
		"name": c.Name, "academic_year_id": c.AcademicYearID,
		"class_teacher_id": c.ClassTeacherID, "capacity": c.Capacity,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
}

func (r *ClassRepository) Create(ctx context.Context, tenantID string, c *domain.Class) error {
	return insert(ctx, r.classes, tenantID, c.ID, classFields(c), "academic: create class")
}

func (r *ClassRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Class, error) {
	scope, err := r.classes.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc classDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *ClassRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Class, string, error) {
	cur, err := page(ctx, r.classes, tenantID, bson.M{}, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("academic: list classes: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.Class
	for cur.Next(ctx) {
		var doc classDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("academic: decode class: %w", err)
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

func (r *ClassRepository) ListIDsByTeacher(ctx context.Context, tenantID, staffID string) ([]string, error) {
	scope, err := r.classes.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, live(bson.M{"class_teacher_id": staffID}),
		options.Find().SetProjection(bson.M{"_id": 1}).SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("academic: list class ids: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	out := []string{}
	for cur.Next(ctx) {
		var doc struct {
			ID string `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("academic: decode class id: %w", err)
		}
		out = append(out, doc.ID)
	}
	return out, cur.Err()
}

func (r *ClassRepository) Update(ctx context.Context, tenantID string, c *domain.Class) error {
	return update(ctx, r.classes, tenantID, c.ID, classFields(c), "academic: update class")
}

func (r *ClassRepository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.classes, tenantID, id, "academic: delete class")
}

// ---- subjects ------------------------------------------------------------

type SubjectRepository struct{ subjects *pmongo.Collection }

var _ ports.SubjectRepository = (*SubjectRepository)(nil)

func NewSubjectRepository(store *pmongo.Store) *SubjectRepository {
	return &SubjectRepository{subjects: store.Collection(SubjectCollection)}
}

type subjectDoc struct {
	ID          string    `bson:"_id"`
	TenantID    string    `bson:"tenant_id"`
	Name        string    `bson:"name"`
	Code        *string   `bson:"code,omitempty"`
	Description *string   `bson:"description,omitempty"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (d subjectDoc) toDomain() *domain.Subject {
	return &domain.Subject{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Code: d.Code,
		Description: d.Description, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func subjectFields(s *domain.Subject) bson.M {
	return bson.M{
		"name": s.Name, "code": s.Code, "description": s.Description,
		"created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}

func (r *SubjectRepository) Create(ctx context.Context, tenantID string, s *domain.Subject) error {
	return insert(ctx, r.subjects, tenantID, s.ID, subjectFields(s), "academic: create subject")
}

func (r *SubjectRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Subject, error) {
	scope, err := r.subjects.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc subjectDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *SubjectRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Subject, string, error) {
	cur, err := page(ctx, r.subjects, tenantID, bson.M{}, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("academic: list subjects: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.Subject
	for cur.Next(ctx) {
		var doc subjectDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("academic: decode subject: %w", err)
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

func (r *SubjectRepository) Update(ctx context.Context, tenantID string, s *domain.Subject) error {
	return update(ctx, r.subjects, tenantID, s.ID, subjectFields(s), "academic: update subject")
}

func (r *SubjectRepository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.subjects, tenantID, id, "academic: delete subject")
}

// ---- grading scales ------------------------------------------------------

type GradingScaleRepository struct{ scales *pmongo.Collection }

var _ ports.GradingScaleRepository = (*GradingScaleRepository)(nil)

func NewGradingScaleRepository(store *pmongo.Store) *GradingScaleRepository {
	return &GradingScaleRepository{scales: store.Collection(GradingCollection)}
}

type gradingScaleDoc struct {
	ID        string              `bson:"_id"`
	TenantID  string              `bson:"tenant_id"`
	Name      string              `bson:"name"`
	Ranges    []domain.GradeRange `bson:"ranges"`
	CreatedAt time.Time           `bson:"created_at"`
	UpdatedAt time.Time           `bson:"updated_at"`
}

func (d gradingScaleDoc) toDomain() *domain.GradingScale {
	return &domain.GradingScale{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, Ranges: d.Ranges,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func gradingFields(s *domain.GradingScale) bson.M {
	return bson.M{
		"name": s.Name, "ranges": s.Ranges,
		"created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}

func (r *GradingScaleRepository) Create(ctx context.Context, tenantID string, s *domain.GradingScale) error {
	return insert(ctx, r.scales, tenantID, s.ID, gradingFields(s), "academic: create grading scale")
}

func (r *GradingScaleRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.GradingScale, error) {
	scope, err := r.scales.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc gradingScaleDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *GradingScaleRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.GradingScale, string, error) {
	cur, err := page(ctx, r.scales, tenantID, bson.M{}, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("academic: list grading scales: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.GradingScale
	for cur.Next(ctx) {
		var doc gradingScaleDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("academic: decode grading scale: %w", err)
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

func (r *GradingScaleRepository) Update(ctx context.Context, tenantID string, s *domain.GradingScale) error {
	return update(ctx, r.scales, tenantID, s.ID, gradingFields(s), "academic: update grading scale")
}

func (r *GradingScaleRepository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.scales, tenantID, id, "academic: delete grading scale")
}

// ---- timetable -----------------------------------------------------------

type TimetableRepository struct{ entries *pmongo.Collection }

var _ ports.TimetableRepository = (*TimetableRepository)(nil)

func NewTimetableRepository(store *pmongo.Store) *TimetableRepository {
	return &TimetableRepository{entries: store.Collection(TimetableCollection)}
}

type timetableDoc struct {
	ID          string    `bson:"_id"`
	TenantID    string    `bson:"tenant_id"`
	ClassID     string    `bson:"class_id"`
	TermID      string    `bson:"term_id"`
	SubjectID   string    `bson:"subject_id"`
	TeacherID   *string   `bson:"teacher_id,omitempty"`
	Weekday     int       `bson:"weekday"`
	StartMinute int       `bson:"start_minute"`
	EndMinute   int       `bson:"end_minute"`
	Room        *string   `bson:"room,omitempty"`
	Status      string    `bson:"status"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (d timetableDoc) toDomain() *domain.TimetableEntry {
	return &domain.TimetableEntry{
		ID: d.ID, TenantID: d.TenantID, ClassID: d.ClassID, TermID: d.TermID,
		SubjectID: d.SubjectID, TeacherID: d.TeacherID, Weekday: d.Weekday,
		StartTime: formatMinute(d.StartMinute), EndTime: formatMinute(d.EndMinute),
		Room: d.Room, Status: d.Status, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func minute(value string) int {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0
	}
	return parsed.Hour()*60 + parsed.Minute()
}

func formatMinute(value int) string { return fmt.Sprintf("%02d:%02d", value/60, value%60) }

func timetableFields(e *domain.TimetableEntry) bson.M {
	return bson.M{
		"class_id": e.ClassID, "term_id": e.TermID, "subject_id": e.SubjectID,
		"teacher_id": e.TeacherID, "weekday": e.Weekday,
		"start_minute": minute(e.StartTime), "end_minute": minute(e.EndTime),
		"room": e.Room, "status": e.Status,
		"created_at": e.CreatedAt, "updated_at": e.UpdatedAt,
	}
}

// assertNoOverlap rejects an active period that collides with another active
// period for the same class, or for the same teacher, within a term and weekday.
//
// PostgreSQL enforces this with two gist exclusion constraints, which hold under
// concurrency. MongoDB has no equivalent and no transaction on the free tier, so
// this is a check before the write: it catches the conflicts a user can actually
// create, but two simultaneous writes could still both pass it. The constraint
// is therefore advisory here in a way it is not on PostgreSQL.
func (r *TimetableRepository) assertNoOverlap(ctx context.Context, tenantID string, e *domain.TimetableEntry) error {
	if e.Status != "active" {
		return nil
	}
	scope, err := r.entries.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	start, end := minute(e.StartTime), minute(e.EndTime)
	overlapping := bson.M{
		"_id":          bson.M{"$ne": e.ID},
		"status":       "active",
		"term_id":      e.TermID,
		"weekday":      e.Weekday,
		"start_minute": bson.M{"$lt": end},
		"end_minute":   bson.M{"$gt": start},
	}

	byClass := bson.M{}
	for k, v := range overlapping {
		byClass[k] = v
	}
	byClass["class_id"] = e.ClassID
	count, err := scope.CountDocuments(ctx, live(byClass))
	if err != nil {
		return fmt.Errorf("academic: check class timetable overlap: %w", err)
	}
	if count > 0 {
		return domain.ErrConflict
	}

	if e.TeacherID == nil {
		return nil
	}
	byTeacher := bson.M{}
	for k, v := range overlapping {
		byTeacher[k] = v
	}
	byTeacher["teacher_id"] = *e.TeacherID
	count, err = scope.CountDocuments(ctx, live(byTeacher))
	if err != nil {
		return fmt.Errorf("academic: check teacher timetable overlap: %w", err)
	}
	if count > 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *TimetableRepository) Create(ctx context.Context, tenantID string, e *domain.TimetableEntry) error {
	if err := r.assertNoOverlap(ctx, tenantID, e); err != nil {
		return err
	}
	return insert(ctx, r.entries, tenantID, e.ID, timetableFields(e), "academic: create timetable entry")
}

func (r *TimetableRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.TimetableEntry, error) {
	scope, err := r.entries.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc timetableDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *TimetableRepository) List(ctx context.Context, tenantID string, filter ports.TimetableFilter) ([]*domain.TimetableEntry, error) {
	scope, err := r.entries.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	query := bson.M{}
	if len(filter.ClassIDs) > 0 {
		query["class_id"] = bson.M{"$in": filter.ClassIDs}
	}
	if filter.TermID != "" {
		query["term_id"] = filter.TermID
	}
	if filter.Weekday > 0 {
		query["weekday"] = filter.Weekday
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	cur, err := scope.Find(ctx, live(query), options.Find().
		SetSort(bson.D{{Key: "weekday", Value: 1}, {Key: "start_minute", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("academic: list timetable: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	out := []*domain.TimetableEntry{}
	for cur.Next(ctx) {
		var doc timetableDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("academic: decode timetable entry: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, cur.Err()
}

func (r *TimetableRepository) Update(ctx context.Context, tenantID string, e *domain.TimetableEntry) error {
	if err := r.assertNoOverlap(ctx, tenantID, e); err != nil {
		return err
	}
	return update(ctx, r.entries, tenantID, e.ID, timetableFields(e), "academic: update timetable entry")
}

func (r *TimetableRepository) Delete(ctx context.Context, tenantID, id string) error {
	return remove(ctx, r.entries, tenantID, id, "academic: delete timetable entry")
}

// ---- lifecycle and outbox ------------------------------------------------

// CommitAcademicLifecycle applies the mutation and queues its event in one
// atomic single-document write on the aggregate it touches. Deletes tombstone
// the record so the queued event survives to be published.
func (r *Repository) CommitAcademicLifecycle(ctx context.Context, tenantID string, mutation ports.AcademicMutation, eventType string, payload map[string]any) error {
	event, err := lifecycleEvent(tenantID, eventType, payload)
	if err != nil {
		return err
	}

	switch mutation.Kind {
	case ports.AcademicMutationYearCreate:
		return insertWithEvent(ctx, r.years, tenantID, mutation.Year.ID, yearFields(mutation.Year), event)
	case ports.AcademicMutationYearUpdate:
		return updateWithEvent(ctx, r.years, tenantID, mutation.Year.ID, yearFields(mutation.Year), event)
	case ports.AcademicMutationYearDelete:
		return tombstoneWithEvent(ctx, r.years, tenantID, mutation.Year.ID, event)
	case ports.AcademicMutationTermUpdate:
		return updateWithEvent(ctx, r.terms, tenantID, mutation.Term.ID, termFields(mutation.Term), event)
	case ports.AcademicMutationTermDelete:
		return tombstoneWithEvent(ctx, r.terms, tenantID, mutation.Term.ID, event)
	case ports.AcademicMutationClassCreate:
		return insertWithEvent(ctx, r.classes, tenantID, mutation.Class.ID, classFields(mutation.Class), event)
	case ports.AcademicMutationClassUpdate:
		return updateWithEvent(ctx, r.classes, tenantID, mutation.Class.ID, classFields(mutation.Class), event)
	case ports.AcademicMutationClassDelete:
		return tombstoneWithEvent(ctx, r.classes, tenantID, mutation.Class.ID, event)
	case ports.AcademicMutationSubjectCreate:
		return insertWithEvent(ctx, r.subjects, tenantID, mutation.Subject.ID, subjectFields(mutation.Subject), event)
	case ports.AcademicMutationSubjectUpdate:
		return updateWithEvent(ctx, r.subjects, tenantID, mutation.Subject.ID, subjectFields(mutation.Subject), event)
	case ports.AcademicMutationSubjectDelete:
		return tombstoneWithEvent(ctx, r.subjects, tenantID, mutation.Subject.ID, event)
	default:
		return fmt.Errorf("academic: unsupported lifecycle mutation %q", mutation.Kind)
	}
}

func insertWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, fields bson.M, event tenancy.CloudEvent) error {
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
		return fmt.Errorf("academic: lifecycle insert: %w", err)
	}
	return nil
}

func updateWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, fields bson.M, event tenancy.CloudEvent) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": id}), bson.M{"$set": fields}, event)
	if err != nil {
		return fmt.Errorf("academic: lifecycle update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func tombstoneWithEvent(ctx context.Context, coll *pmongo.Collection, tenantID, id string, event tenancy.CloudEvent) error {
	scope, err := coll.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": id}),
		bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, event)
	if err != nil {
		return fmt.Errorf("academic: lifecycle delete: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

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

func (r *Repository) ClaimPendingAcademicEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := []ports.OutboxEvent{}
	for _, outbox := range r.outboxes {
		if len(out) >= limit {
			break
		}
		claimed, err := outbox.Claim(ctx, limit-len(out))
		if err != nil {
			return nil, fmt.Errorf("academic: claim outbox: %w", err)
		}
		out = append(out, claimedEvents(claimed)...)
	}
	return out, nil
}

func (r *Repository) MarkAcademicEventPublished(ctx context.Context, id string) error {
	for _, outbox := range r.outboxes {
		if err := outbox.MarkPublishedByEventID(ctx, id); err != nil {
			return fmt.Errorf("academic: mark published: %w", err)
		}
		if _, err := outbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) MarkAcademicEventFailed(ctx context.Context, id, reason string) error {
	for _, outbox := range r.outboxes {
		if err := outbox.MarkFailedByEventID(ctx, id, reason); err != nil {
			return fmt.Errorf("academic: mark failed: %w", err)
		}
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX and UNIQUE statements in the
// PostgreSQL migrations. The gist exclusion constraints protecting the timetable
// have no MongoDB equivalent; assertNoOverlap covers them in the application.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]mongo.IndexModel{
		YearCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}}},
		TermCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "academic_year_id", Value: 1}}}},
		ClassCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "class_teacher_id", Value: 1}}},
		},
		SubjectCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}}},
		GradingCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}}},
		TimetableCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "term_id", Value: 1}, {Key: "weekday", Value: 1}, {Key: "class_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "term_id", Value: 1}, {Key: "weekday", Value: 1}, {Key: "teacher_id", Value: 1}}},
		},
	}
	for name, models := range specs {
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("academic: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}
