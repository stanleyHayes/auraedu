// Package mongo provides the MongoDB implementation of the assessment-service repository.
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

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	AssessmentsCollection = "assessments"
	ScoresCollection      = "scores"
	outboxLease           = 5 * time.Minute
	eventSource           = "assessment-service"
)

type Repository struct {
	store       *pmongo.Store
	assessments *pmongo.Collection
	scores      *pmongo.Collection
	aOutbox     *pmongo.ClaimableOutbox
	sOutbox     *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	assessments := store.Collection(AssessmentsCollection)
	scores := store.Collection(ScoresCollection)
	return &Repository{store: store,
		assessments: assessments,
		scores:      scores,
		aOutbox:     pmongo.NewClaimableOutbox(assessments, outboxLease),
		sOutbox:     pmongo.NewClaimableOutbox(scores, outboxLease),
	}
}

// assessmentDoc mirrors the assessments table so both adapters describe the same record.
type assessmentDoc struct {
	ID             string     `bson:"_id"`
	TenantID       string     `bson:"tenant_id"`
	AcademicYearID string     `bson:"academic_year_id"`
	SubjectID      string     `bson:"subject_id"`
	Type           string     `bson:"type"`
	Title          string     `bson:"title"`
	Description    *string    `bson:"description,omitempty"`
	MaxScore       int        `bson:"max_score"`
	DueDate        *time.Time `bson:"due_date,omitempty"`
	Status         string     `bson:"status"`
	ClassIDs       []string   `bson:"class_ids,omitempty"`
	PublishedAt    *time.Time `bson:"published_at,omitempty"`
	CreatedAt      time.Time  `bson:"created_at"`
	UpdatedAt      time.Time  `bson:"updated_at"`
	DeletedAt      *time.Time `bson:"deleted_at,omitempty"`
}

func (d assessmentDoc) toDomain() *domain.Assessment {
	return &domain.Assessment{
		ID:             d.ID,
		TenantID:       d.TenantID,
		AcademicYearID: d.AcademicYearID,
		SubjectID:      d.SubjectID,
		Type:           d.Type,
		Title:          d.Title,
		Description:    d.Description,
		MaxScore:       d.MaxScore,
		DueDate:        d.DueDate,
		Status:         d.Status,
		ClassIDs:       d.ClassIDs,
		PublishedAt:    d.PublishedAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		DeletedAt:      d.DeletedAt,
	}
}

// scoreDoc mirrors the scores table so both adapters describe the same record.
type scoreDoc struct {
	ID           string     `bson:"_id"`
	TenantID     string     `bson:"tenant_id"`
	AssessmentID string     `bson:"assessment_id"`
	StudentID    string     `bson:"student_id"`
	Score        int        `bson:"score"`
	RecordedBy   string     `bson:"recorded_by"`
	Notes        *string    `bson:"notes,omitempty"`
	CreatedAt    time.Time  `bson:"created_at"`
	UpdatedAt    time.Time  `bson:"updated_at"`
	DeletedAt    *time.Time `bson:"deleted_at,omitempty"`
}

func (d scoreDoc) toDomain() *domain.Score {
	return &domain.Score{
		ID:           d.ID,
		TenantID:     d.TenantID,
		AssessmentID: d.AssessmentID,
		StudentID:    d.StudentID,
		Score:        d.Score,
		RecordedBy:   d.RecordedBy,
		Notes:        d.Notes,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
		DeletedAt:    d.DeletedAt,
	}
}

// live excludes tombstoned records.
func live(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	merged["deleted_at"] = bson.M{"$exists": false}
	return merged
}

func assessmentFields(a *domain.Assessment) bson.M {
	return bson.M{
		"academic_year_id": a.AcademicYearID,
		"subject_id":       a.SubjectID,
		"type":             a.Type,
		"title":            a.Title,
		"description":      a.Description,
		"max_score":        a.MaxScore,
		"due_date":         a.DueDate,
		"status":           a.Status,
		"class_ids":        classIDsOrEmpty(a.ClassIDs),
		"published_at":     a.PublishedAt,
		"created_at":       a.CreatedAt,
		"updated_at":       a.UpdatedAt,
	}
}

func scoreFields(s *domain.Score) bson.M {
	return bson.M{
		"score":       s.Score,
		"recorded_by": s.RecordedBy,
		"notes":       s.Notes,
		"updated_at":  s.UpdatedAt,
	}
}

// classIDsOrEmpty maps a nil slice to an empty one.
func classIDsOrEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// --- Assessments. ---

func (r *Repository) CreateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: create: %w", err)
	}
	doc := assessmentFields(a)
	doc["_id"] = a.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("assessment: create: %w", err)
	}
	return nil
}

func (r *Repository) GetAssessmentByID(ctx context.Context, tenantID, id string) (*domain.Assessment, error) {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("assessment: get: %w", err)
	}
	var doc assessmentDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("assessment: get: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) ListAssessments(ctx context.Context, tenantID string, filter ports.AssessmentListFilter) ([]*domain.Assessment, string, error) {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list: %w", err)
	}

	query := bson.M{}
	if filter.AcademicYearID != "" {
		query["academic_year_id"] = filter.AcademicYearID
	}
	if filter.SubjectID != "" {
		query["subject_id"] = filter.SubjectID
	}
	if filter.Type != "" {
		query["type"] = filter.Type
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.ClassIDs != nil {
		if len(filter.ClassIDs) == 0 {
			return []*domain.Assessment{}, "", nil
		}
		query["class_ids"] = bson.M{"$in": filter.ClassIDs}
	}
	if filter.Cursor != "" {
		var cursorDoc assessmentDoc
		if err := scope.FindOne(ctx, live(bson.M{"_id": filter.Cursor})).Decode(&cursorDoc); err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", fmt.Errorf("assessment: resolve cursor: %w", err)
			}
		} else {
			query["$or"] = bson.A{
				bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
				bson.M{"created_at": cursorDoc.CreatedAt, "_id": bson.M{"$gt": filter.Cursor}},
			}
		}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Assessment
	for cur.Next(ctx) {
		var doc assessmentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("assessment: list decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("assessment: list rows: %w", err)
	}

	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) UpdateAssessment(ctx context.Context, tenantID string, a *domain.Assessment) error {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: update: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": a.ID}), bson.M{"$set": assessmentFields(a)})
	if err != nil {
		return fmt.Errorf("assessment: update: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteAssessment(ctx context.Context, tenantID, id string) error {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: delete: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
	if err != nil {
		return fmt.Errorf("assessment: delete: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Scores. ---

func (r *Repository) createScore(ctx context.Context, tenantID string, s *domain.Score) error {
	scope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: create score: %w", err)
	}
	doc := bson.M{
		"_id":           s.ID,
		"assessment_id": s.AssessmentID,
		"student_id":    s.StudentID,
		"score":         s.Score,
		"recorded_by":   s.RecordedBy,
		"notes":         s.Notes,
		"created_at":    s.CreatedAt,
		"updated_at":    s.UpdatedAt,
	}
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("assessment: create score: %w", err)
	}
	return nil
}

func (r *Repository) GetScoreByID(ctx context.Context, tenantID, assessmentID, scoreID string) (*domain.Score, error) {
	scope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("assessment: get score: %w", err)
	}
	var doc scoreDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": scoreID, "assessment_id": assessmentID})).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("assessment: get score: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) ListScores(ctx context.Context, tenantID, assessmentID string, filter ports.ScoreListFilter) ([]*domain.Score, string, error) {
	scope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list scores: %w", err)
	}

	query := bson.M{"assessment_id": assessmentID}
	if filter.StudentID != "" {
		query["student_id"] = filter.StudentID
	} else if filter.StudentIDs != nil {
		query["student_id"] = bson.M{"$in": filter.StudentIDs}
	}

	if filter.Cursor != "" {
		var cursorDoc scoreDoc
		if err := scope.FindOne(ctx, live(bson.M{"_id": filter.Cursor})).Decode(&cursorDoc); err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", fmt.Errorf("assessment: resolve cursor: %w", err)
			}
		} else {
			query["$or"] = bson.A{
				bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
				bson.M{"created_at": cursorDoc.CreatedAt, "_id": bson.M{"$gt": filter.Cursor}},
			}
		}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list scores: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Score
	for cur.Next(ctx) {
		var doc scoreDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("assessment: list scores decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("assessment: list scores rows: %w", err)
	}

	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) updateScore(ctx context.Context, tenantID string, s *domain.Score) error {
	scope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: update score: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": s.ID}), bson.M{"$set": scoreFields(s)})
	if err != nil {
		return fmt.Errorf("assessment: update score: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteScore(ctx context.Context, tenantID, assessmentID, scoreID string) error {
	scope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: delete score: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": scoreID, "assessment_id": assessmentID}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
	if err != nil {
		return fmt.Errorf("assessment: delete score: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Lifecycle. ---

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("assessment: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

func (r *Repository) commitAssessmentLifecycle(ctx context.Context, tenantID string, m ports.LifecycleMutation, events []ports.LifecycleEvent) error {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: lifecycle: %w", err)
	}
	scoreScope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("assessment: lifecycle: %w", err)
	}

	cloudEvents, err := assessmentEvents(tenantID, events)
	if err != nil {
		return err
	}

	switch m.Kind {
	case ports.AssessmentMutationCreate:
		a := m.Assessment
		doc := assessmentFields(a)
		doc["_id"] = a.ID
		if _, err := scope.InsertWithEvents(ctx, doc, cloudEvents...); err != nil {
			return fmt.Errorf("assessment: create: %w", err)
		}
	case ports.AssessmentMutationUpdate:
		a := m.Assessment
		res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": a.ID}),
			bson.M{"$set": assessmentFields(a)}, cloudEvents...)
		if err != nil {
			return fmt.Errorf("assessment: update: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	case ports.AssessmentMutationDelete:
		a := m.Assessment
		res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": a.ID, "deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, cloudEvents...)
		if err != nil {
			return fmt.Errorf("assessment: delete: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	case ports.AssessmentMutationScoreCreate:
		s := m.Score
		doc := bson.M{
			"_id":           s.ID,
			"assessment_id": s.AssessmentID,
			"student_id":    s.StudentID,
			"score":         s.Score,
			"recorded_by":   s.RecordedBy,
			"notes":         s.Notes,
			"created_at":    s.CreatedAt,
			"updated_at":    s.UpdatedAt,
		}
		if _, err := scoreScope.InsertWithEvents(ctx, doc, cloudEvents...); err != nil {
			return fmt.Errorf("assessment: create score: %w", err)
		}
	case ports.AssessmentMutationScoreUpdate:
		s := m.Score
		res, err := scoreScope.UpdateWithEvents(ctx, live(bson.M{"_id": s.ID}),
			bson.M{"$set": scoreFields(s)}, cloudEvents...)
		if err != nil {
			return fmt.Errorf("assessment: update score: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	case ports.AssessmentMutationScoreDelete:
		s := m.Score
		res, err := scoreScope.UpdateWithEvents(ctx, bson.M{"_id": s.ID, "deleted_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, cloudEvents...)
		if err != nil {
			return fmt.Errorf("assessment: delete score: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
	default:
		return fmt.Errorf("assessment: unsupported lifecycle mutation %q", m.Kind)
	}
	return nil
}

// --- Outbox. ---

func claimed(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID:        c.Event.ID,
			TenantID:  c.Event.TenantID,
			EventType: c.Event.Type,
			Payload:   json.RawMessage(c.Event.Data),
		})
	}
	return out
}

func (r *Repository) ClaimPendingAssessmentEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	fromAssessments, err := r.aOutbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("assessment: claim outbox: %w", err)
	}
	out := claimed(fromAssessments)
	if len(out) >= limit {
		return out, nil
	}
	fromScores, err := r.sOutbox.Claim(ctx, limit-len(out))
	if err != nil {
		return nil, fmt.Errorf("assessment: claim score outbox: %w", err)
	}
	return append(out, claimed(fromScores)...), nil
}

func (r *Repository) MarkAssessmentEventPublished(ctx context.Context, id string) error {
	if err := r.aOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("assessment: mark published: %w", err)
	}
	if err := r.sOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("assessment: mark score published: %w", err)
	}
	return r.clearPublishedTombstones(ctx)
}

func (r *Repository) MarkAssessmentEventFailed(ctx context.Context, id, reason string) error {
	if err := r.aOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("assessment: mark failed: %w", err)
	}
	if err := r.sOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("assessment: mark score failed: %w", err)
	}
	return nil
}

func (r *Repository) clearPublishedTombstones(ctx context.Context) error {
	_, err := r.aOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("assessment: clear assessment tombstones: %w", err)
	}
	_, err = r.sOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("assessment: clear score tombstones: %w", err)
	}
	return nil
}

// --- Assignments. ---

func (r *Repository) ListAssignments(ctx context.Context, tenantID string, filter ports.AssignmentListFilter) ([]*domain.Assessment, string, error) {
	scope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list assignments: %w", err)
	}

	query := bson.M{"type": string(domain.TypeAssignment)}
	if filter.SubjectID != "" {
		query["subject_id"] = filter.SubjectID
	}
	if filter.ClassID != "" {
		query["class_ids"] = filter.ClassID
	}
	if filter.ClassIDs != nil {
		if len(filter.ClassIDs) == 0 {
			return []*domain.Assessment{}, "", nil
		}
		query["class_ids"] = bson.M{"$in": filter.ClassIDs}
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}

	if filter.StudentID != "" {
		scoreScope, err := r.scores.ScopeTo(tenantID)
		if err != nil {
			return nil, "", fmt.Errorf("assessment: list assignments: %w", err)
		}

		scoreCur, err := scoreScope.Find(ctx, live(bson.M{"student_id": filter.StudentID}),
			options.Find().SetProjection(bson.M{"assessment_id": 1}))
		if err != nil {
			return nil, "", fmt.Errorf("assessment: list assignments find scores: %w", err)
		}
		defer func() { _ = scoreCur.Close(ctx) }()

		var assessmentIDs []string
		for scoreCur.Next(ctx) {
			var doc struct {
				AssessmentID string `bson:"assessment_id"`
			}
			if err := scoreCur.Decode(&doc); err != nil {
				return nil, "", fmt.Errorf("assessment: list assignments decode score: %w", err)
			}
			assessmentIDs = append(assessmentIDs, doc.AssessmentID)
		}
		if err := scoreCur.Err(); err != nil {
			return nil, "", fmt.Errorf("assessment: list assignments score rows: %w", err)
		}

		if len(assessmentIDs) == 0 {
			return []*domain.Assessment{}, "", nil
		}
		query["_id"] = bson.M{"$in": assessmentIDs}
	}

	if filter.Cursor != "" {
		var cursorDoc assessmentDoc
		if err := scope.FindOne(ctx, live(bson.M{"_id": filter.Cursor})).Decode(&cursorDoc); err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", fmt.Errorf("assessment: resolve cursor: %w", err)
			}
		} else {
			query["$or"] = bson.A{
				bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
				bson.M{"created_at": cursorDoc.CreatedAt, "_id": bson.M{"$gt": filter.Cursor}},
			}
		}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("assessment: list assignments: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Assessment
	for cur.Next(ctx) {
		var doc assessmentDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("assessment: list assignments decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("assessment: list assignments rows: %w", err)
	}

	var next string
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// --- Gradebook. ---

func (r *Repository) GradebookScores(ctx context.Context, tenantID string, filter ports.GradebookFilter) ([]domain.GradeRow, error) {
	var out []domain.GradeRow
	aScope, err := r.assessments.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("assessment: gradebook: %w", err)
	}
	sScope, err := r.scores.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("assessment: gradebook: %w", err)
	}

	scoreQuery := bson.M{}
	if filter.StudentID != "" {
		scoreQuery["student_id"] = filter.StudentID
	}

	scoreCur, err := sScope.Find(ctx, live(scoreQuery),
		options.Find().SetProjection(bson.M{"assessment_id": 1, "score": 1}))
	if err != nil {
		return nil, fmt.Errorf("assessment: gradebook find scores: %w", err)
	}
	defer func() { _ = scoreCur.Close(ctx) }()

	type scoreWithAssessment struct {
		AssessmentID string
		Score        int
	}
	var scores []scoreWithAssessment
	for scoreCur.Next(ctx) {
		var doc struct {
			AssessmentID string `bson:"assessment_id"`
			Score        int    `bson:"score"`
		}
		if err := scoreCur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("assessment: gradebook decode score: %w", err)
		}
		scores = append(scores, scoreWithAssessment{AssessmentID: doc.AssessmentID, Score: doc.Score})
	}
	if err := scoreCur.Err(); err != nil {
		return nil, fmt.Errorf("assessment: gradebook score rows: %w", err)
	}

	assessmentQuery := bson.M{}
	if filter.AcademicYearID != "" {
		assessmentQuery["academic_year_id"] = filter.AcademicYearID
	}
	if filter.SubjectID != "" {
		assessmentQuery["subject_id"] = filter.SubjectID
	}
	if filter.ClassID != "" {
		assessmentQuery["class_ids"] = filter.ClassID
	}

	aCur, err := aScope.Find(ctx, live(assessmentQuery),
		options.Find().SetProjection(bson.M{"_id": 1, "subject_id": 1, "max_score": 1}).
			SetSort(bson.D{{Key: "subject_id", Value: 1}, {Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("assessment: gradebook find assessments: %w", err)
	}
	defer func() { _ = aCur.Close(ctx) }()

	type assessmentInfo struct {
		ID        string
		SubjectID string
		MaxScore  int
	}
	var assessments []assessmentInfo
	for aCur.Next(ctx) {
		var doc struct {
			ID        string `bson:"_id"`
			SubjectID string `bson:"subject_id"`
			MaxScore  int    `bson:"max_score"`
		}
		if err := aCur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("assessment: gradebook decode assessment: %w", err)
		}
		assessments = append(assessments, assessmentInfo{ID: doc.ID, SubjectID: doc.SubjectID, MaxScore: doc.MaxScore})
	}
	if err := aCur.Err(); err != nil {
		return nil, fmt.Errorf("assessment: gradebook assessment rows: %w", err)
	}

	assessmentMap := make(map[string]assessmentInfo)
	for _, a := range assessments {
		assessmentMap[a.ID] = a
	}

	for _, s := range scores {
		if a, ok := assessmentMap[s.AssessmentID]; ok {
			out = append(out, domain.GradeRow{
				SubjectID: a.SubjectID,
				Score:     s.Score,
				MaxScore:  a.MaxScore,
			})
		}
	}

	return out, nil
}

// EnsureIndexes creates the indexes the queries above rely on.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]bson.D{
		AssessmentsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "academic_year_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "subject_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "type", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
		},
		ScoresCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "assessment_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
		},
	}
	for name, keys := range specs {
		models := make([]mongo.IndexModel, 0, len(keys))
		for _, k := range keys {
			models = append(models, mongo.IndexModel{Keys: k})
		}
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("assessment: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}

func (r *Repository) fenceAssessment(ctx context.Context, t, id string) error {
	scope, err := r.assessments.ScopeTo(t)
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
func (r *Repository) CreateScore(ctx context.Context, t string, s *domain.Score) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.fenceAssessment(ctx, t, s.AssessmentID); err != nil {
			return err
		}
		return r.createScore(ctx, t, s)
	})
}
func (r *Repository) UpdateScore(ctx context.Context, t string, s *domain.Score) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.fenceAssessment(ctx, t, s.AssessmentID); err != nil {
			return err
		}
		return r.updateScore(ctx, t, s)
	})
}
func (r *Repository) CommitAssessmentLifecycle(ctx context.Context, t string, m ports.LifecycleMutation, events []ports.LifecycleEvent) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if m.Kind == ports.AssessmentMutationScoreCreate || m.Kind == ports.AssessmentMutationScoreUpdate {
			if m.Score == nil {
				return domain.ErrValidation
			}
			if err := r.fenceAssessment(ctx, t, m.Score.AssessmentID); err != nil {
				return err
			}
		}
		return r.commitAssessmentLifecycle(ctx, t, m, events)
	})
}

func assessmentEvents(tenantID string, events []ports.LifecycleEvent) ([]tenancy.CloudEvent, error) {
	result := make([]tenancy.CloudEvent, 0, len(events))
	for _, event := range events {
		cloudEvent, err := lifecycleEvent(tenantID, event.EventType, event.Payload)
		if err != nil {
			return nil, err
		}
		result = append(result, cloudEvent)
	}
	return result, nil
}
