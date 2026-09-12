// Package mongo implements tenant-scoped CBT persistence with atomic embedded events.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct{ store *pmongo.Store }

func NewRepository(s *pmongo.Store) *Repository { return &Repository{s} }

var _ ports.Repository = (*Repository)(nil)
var _ ports.LifecycleRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.store.Database().Collection("cbt_submissions").Indexes().CreateOne(ctx,
		driver.IndexModel{Keys: bson.D{{Key: "tenant_id", Value: 1},
			{Key: "body.examsessionid", Value: 1}, {Key: "body.studentid",
				Value: 1}}, Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"deleted": false})})
	return err
}
func live(id string) bson.M {
	q := bson.M{"deleted": false}
	if id != "" {
		q["_id"] = id
	}
	return q
}
func mapped(err error) error {
	if errors.Is(err, driver.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if driver.IsDuplicateKeyError(err) {
		return domain.ErrConflict
	}
	return err
}
func (r *Repository) write(ctx context.Context, tenant, collection, id string, body any, create, remove bool, events []tenancy.CloudEvent) error {
	switch v := body.(type) {
	case *domain.QuestionBank:
		cloned := *v
		cloned.TenantID = tenant
		body = cloned
	case *domain.ExamSession:
		cloned := *v
		cloned.TenantID = tenant
		body = cloned
	case *domain.Submission:
		cloned := *v
		cloned.TenantID = tenant
		body = cloned
	}
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return err
	}
	if create {
		_, err = scope.InsertWithEvents(ctx, bson.M{"_id": id, "body": body, "deleted": false}, events...)
		return mapped(err)
	}
	fields := bson.M{"body": body}
	if remove {
		fields = bson.M{"deleted": true}
	}
	result, err := scope.UpdateWithEvents(ctx, live(id), bson.M{"$set": fields}, events...)
	if err != nil {
		return mapped(err)
	}
	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func get[T any](ctx context.Context, r *Repository, tenant, collection string, q bson.M) (*T, error) {
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Body T `bson:"body"`
	}
	err = scope.FindOne(ctx, q).Decode(&doc)
	if err != nil {
		return nil, mapped(err)
	}
	return &doc.Body, nil
}
func list[T any](ctx context.Context, r *Repository, tenant, collection, cursor string, limit int, query bson.M) ([]*T, string, error) {
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return nil, "", err
	}
	query["deleted"] = false
	if cursor != "" {
		var anchor struct {
			Body struct {
				CreatedAt time.Time `bson:"createdat"`
			} `bson:"body"`
		}
		if err = scope.FindOne(ctx, bson.M{"_id": cursor}).Decode(&anchor); err != nil {
			if errors.Is(err, driver.ErrNoDocuments) {
				return []*T{}, "", nil
			}
			return nil, "", err
		}
		query["$or"] = bson.A{bson.M{"body.createdat": bson.M{"$gt": anchor.Body.CreatedAt}},
			bson.M{"body.createdat": anchor.Body.CreatedAt, "_id": bson.M{"$gt": cursor}}}
	}
	if limit <= 0 {
		limit = 50
	}
	cur, err := scope.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "body.createdat", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", err
	}
	defer closeCursor(ctx, cur)
	out := make([]*T, 0)
	last := ""
	for cur.Next(ctx) {
		var doc struct {
			ID   string `bson:"_id"`
			Body T      `bson:"body"`
		}
		if err = cur.Decode(&doc); err != nil {
			return nil, "", err
		}
		out = append(out, &doc.Body)
		last = doc.ID
	}
	if len(out) < limit {
		last = ""
	}
	return out, last, cur.Err()
}
func (r *Repository) CreateQuestion(ctx context.Context, tenant string, v *domain.QuestionBank) error {
	return r.write(ctx, tenant, "cbt_questions", v.ID, v, true, false, nil)
}
func (r *Repository) UpdateQuestion(ctx context.Context, tenant string, v *domain.QuestionBank) error {
	return r.write(ctx, tenant, "cbt_questions", v.ID, v, false, false, nil)
}
func (r *Repository) DeleteQuestion(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "cbt_questions", id, nil, false, true, nil)
}
func (r *Repository) GetQuestionByID(ctx context.Context, tenant, id string) (*domain.QuestionBank, error) {
	return get[domain.QuestionBank](ctx, r, tenant, "cbt_questions", live(id))
}
func (r *Repository) ListQuestions(ctx context.Context, tenant string, f ports.QuestionListFilter) ([]*domain.QuestionBank, string, error) {
	q := bson.M{}
	if f.AcademicYearID != "" {
		q["body.academicyearid"] = f.AcademicYearID
	}
	if f.SubjectID != "" {
		q["body.subjectid"] = f.SubjectID
	}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	return list[domain.QuestionBank](ctx, r, tenant, "cbt_questions", f.Cursor, f.Limit, q)
}
func (r *Repository) CreateExamSession(ctx context.Context, tenant string, v *domain.ExamSession) error {
	return r.write(ctx, tenant, "cbt_exam_sessions", v.ID, v, true, false, nil)
}
func (r *Repository) UpdateExamSession(ctx context.Context, tenant string, v *domain.ExamSession) error {
	return r.write(ctx, tenant, "cbt_exam_sessions", v.ID, v, false, false, nil)
}
func (r *Repository) DeleteExamSession(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "cbt_exam_sessions", id, nil, false, true, nil)
}
func (r *Repository) GetExamSessionByID(ctx context.Context, tenant, id string) (*domain.ExamSession, error) {
	return get[domain.ExamSession](ctx, r, tenant, "cbt_exam_sessions", live(id))
}
func (r *Repository) ListExamSessions(ctx context.Context, tenant string, f ports.ExamSessionListFilter) ([]*domain.ExamSession, string, error) {
	q := bson.M{}
	if f.AcademicYearID != "" {
		q["body.academicyearid"] = f.AcademicYearID
	}
	if f.SubjectID != "" {
		q["body.subjectid"] = f.SubjectID
	}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	return list[domain.ExamSession](ctx, r, tenant, "cbt_exam_sessions", f.Cursor, f.Limit, q)
}
func (r *Repository) CreateSubmission(ctx context.Context, tenant string, v *domain.Submission) error {
	return r.write(ctx, tenant, "cbt_submissions", v.ID, v, true, false, nil)
}
func (r *Repository) UpdateSubmission(ctx context.Context, tenant string, v *domain.Submission) error {
	return r.write(ctx, tenant, "cbt_submissions", v.ID, v, false, false, nil)
}
func (r *Repository) DeleteSubmission(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "cbt_submissions", id, nil, false, true, nil)
}
func (r *Repository) GetSubmissionByID(ctx context.Context, tenant, id string) (*domain.Submission, error) {
	return get[domain.Submission](ctx, r, tenant, "cbt_submissions", live(id))
}
func (r *Repository) ListSubmissions(ctx context.Context, tenant string, f ports.SubmissionListFilter) ([]*domain.Submission, string, error) {
	q := bson.M{}
	if f.ExamSessionID != "" {
		q["body.examsessionid"] = f.ExamSessionID
	}
	if f.StudentID != "" {
		q["body.studentid"] = f.StudentID
	}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	return list[domain.Submission](ctx, r, tenant, "cbt_submissions", f.Cursor, f.Limit, q)
}
func (r *Repository) GetSubmissionByExamAndStudent(ctx context.Context, tenant, exam, student string) (*domain.Submission, error) {
	q := live("")
	q["body.examsessionid"] = exam
	q["body.studentid"] = student
	return get[domain.Submission](ctx, r, tenant, "cbt_submissions", q)
}
func (r *Repository) CommitCBTLifecycle(ctx context.Context, tenant string, m ports.LifecycleMutation, es []ports.LifecycleEvent) error {
	if len(es) == 0 {
		return errors.New("cbt: lifecycle events are required")
	}
	events := make([]tenancy.CloudEvent, 0, len(es))
	for _, e := range es {
		data, err := json.Marshal(e.Payload)
		if err != nil {
			return err
		}
		events = append(events, tenancy.CloudEvent{SpecVersion: "1.0",
			ID: uuid.NewString(), Source: "cbt-service", Type: e.EventType,
			TenantID: tenant, Time: time.Now().UTC().Format(time.RFC3339),
			Data: data})
	}
	switch m.Kind {
	case ports.CBTMutationQuestionCreate, ports.CBTMutationQuestionUpdate, ports.CBTMutationQuestionDelete:
		if m.Question == nil {
			return errors.New("cbt: question required")
		}
		return r.write(ctx, tenant, "cbt_questions", m.Question.ID,
			m.Question, m.Kind == ports.CBTMutationQuestionCreate,
			m.Kind == ports.CBTMutationQuestionDelete, events)
	case ports.CBTMutationExamCreate, ports.CBTMutationExamUpdate, ports.CBTMutationExamDelete:
		if m.Exam == nil {
			return errors.New("cbt: exam required")
		}
		return r.write(ctx, tenant, "cbt_exam_sessions", m.Exam.ID, m.Exam, m.Kind == ports.CBTMutationExamCreate, m.Kind == ports.CBTMutationExamDelete, events)
	case ports.CBTMutationSubmissionUpdate:
		if m.Submission == nil {
			return errors.New("cbt: submission required")
		}
		return r.write(ctx, tenant, "cbt_submissions", m.Submission.ID, m.Submission, false, false, events)
	default:
		return fmt.Errorf("cbt: unsupported mutation %q", m.Kind)
	}
}

func collections() []string { return []string{"cbt_questions", "cbt_exam_sessions", "cbt_submissions"} }

func (r *Repository) ClaimPendingCBTEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := make([]ports.OutboxEvent, 0)
	for _, name := range collections() {
		if len(out) >= limit {
			break
		}
		items, err := pmongo.NewClaimableOutbox(r.store.Collection(name), 5*time.Minute).Claim(ctx, limit-len(out))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			out = append(out, ports.OutboxEvent{ID: item.Event.ID, TenantID: item.Event.TenantID, EventType: item.Event.Type, Payload: item.Event.Data})
		}
	}
	return out, nil
}
func (r *Repository) MarkCBTEventPublished(ctx context.Context, id string) error {
	for _, name := range collections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkPublishedByEventID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *Repository) MarkCBTEventFailed(ctx context.Context, id, message string) error {
	for _, name := range collections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkFailedByEventID(ctx, id, message); err != nil {
			return err
		}
	}
	return nil
}
