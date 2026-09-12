package mongo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const TemplateCollection = "report_templates"
const CardCollection = "report_cards"

type Repository struct {
	templates, cards *pmongo.Collection
	store            *pmongo.Store
	outboxes         []*pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(s *pmongo.Store) *Repository {
	return &Repository{s.Collection(TemplateCollection),
		s.Collection(CardCollection),
		s,
		[]*pmongo.ClaimableOutbox{pmongo.NewClaimableOutbox(s.Collection(TemplateCollection),
			time.Minute),
			pmongo.NewClaimableOutbox(s.Collection(CardCollection),
				time.Minute)}}
}
func (r *Repository) CreateReportTemplate(ctx context.Context, t string, v *domain.ReportTemplate) error {
	return insert(ctx, r.templates, t, v.ID, v)
}
func (r *Repository) GetReportTemplateByID(ctx context.Context, t, id string) (*domain.ReportTemplate, error) {
	return get[domain.ReportTemplate](ctx, r.templates, t, bson.M{"_id": id})
}
func (r *Repository) ListReportTemplates(ctx context.Context, t string, f ports.ReportTemplateListFilter) ([]*domain.ReportTemplate, string, error) {
	q := bson.M{}
	equal(q, "academicyearid", f.AcademicYearID)
	equal(q, "status", f.Status)
	return list[domain.ReportTemplate](ctx, r.templates, t, q, f.Limit, f.Cursor)
}
func (r *Repository) UpdateReportTemplate(ctx context.Context, t string, v *domain.ReportTemplate) error {
	return update(ctx, r.templates, t, v.ID, v)
}
func (r *Repository) DeleteReportTemplate(ctx context.Context, t, id string) error {
	return r.deleteTemplate(ctx, t, id)
}
func (r *Repository) checkTemplate(ctx context.Context, t, id string) error {
	if id == "" {
		return nil
	}
	scope, err := r.templates.ScopeTo(t)
	if err != nil {
		return err
	}
	return changed(scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$inc": bson.M{"_reference_version": 1}}))
}
func (r *Repository) CreateReportCard(ctx context.Context, t string, v *domain.ReportCard) error {
	fn := func(ctx context.Context) error {
		if err := r.checkTemplate(ctx, t, v.TemplateID); err != nil {
			return err
		}
		return insert(ctx, r.cards, t, v.ID, v)
	}
	if v.TemplateID == "" {
		return fn(ctx)
	}
	return inTransaction(ctx, r.store, fn)
}
func (r *Repository) GetReportCardByID(ctx context.Context, t, id string) (*domain.ReportCard, error) {
	return get[domain.ReportCard](ctx, r.cards, t, bson.M{"_id": id})
}
func (r *Repository) ListReportCards(ctx context.Context, t string, f ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	q := bson.M{}
	equal(q, "academicyearid", f.AcademicYearID)
	equal(q, "status", f.Status)
	equal(q, "studentid", f.StudentID)
	equal(q, "templateid", f.TemplateID)
	if len(f.StudentIDs) > 0 && f.StudentID != "" && !slices.Contains(f.StudentIDs, f.StudentID) {
		return []*domain.ReportCard{}, "", nil
	}
	if len(f.StudentIDs) > 0 && f.StudentID == "" {
		q["data.studentid"] = bson.M{"$in": f.StudentIDs}
	}
	return list[domain.ReportCard](ctx, r.cards, t, q, f.Limit, f.Cursor)
}
func (r *Repository) UpdateReportCard(ctx context.Context, t string, v *domain.ReportCard) error {
	fn := func(ctx context.Context) error {
		if err := r.checkTemplate(ctx, t, v.TemplateID); err != nil {
			return err
		}
		return update(ctx, r.cards, t, v.ID, v)
	}
	if v.TemplateID == "" {
		return fn(ctx)
	}
	return inTransaction(ctx, r.store, fn)
}
func (r *Repository) DeleteReportCard(ctx context.Context, t, id string) error {
	return remove(ctx, r.cards, t, id)
}
func (r *Repository) CommitReportTemplateLifecycle(ctx context.Context,
	t string,
	v *domain.ReportTemplate,
	mutation,
	kind string,
	payload map[string]any) error {
	e, err := event(t, kind, payload)
	if err != nil {
		return err
	}
	switch mutation {
	case ports.ReportMutationCreate:
		return insert(ctx, r.templates, t, v.ID, v, e)
	case ports.ReportMutationUpdate:
		return update(ctx, r.templates, t, v.ID, v, e)
	case ports.ReportMutationDelete:
		return r.deleteTemplate(ctx, t, v.ID, e)
	}
	return domain.ErrValidation
}
func (r *Repository) CommitReportCardLifecycle(ctx context.Context, t string, v *domain.ReportCard, mutation, kind string, payload map[string]any) error {
	fn := func(ctx context.Context) error {
		if mutation != ports.ReportMutationDelete {
			if err := r.checkTemplate(ctx, t, v.TemplateID); err != nil {
				return err
			}
		}
		e, err := event(t, kind, payload)
		if err != nil {
			return err
		}
		switch mutation {
		case ports.ReportMutationCreate:
			return insert(ctx, r.cards, t, v.ID, v, e)
		case ports.ReportMutationUpdate:
			return update(ctx, r.cards, t, v.ID, v, e)
		case ports.ReportMutationDelete:
			return remove(ctx, r.cards, t, v.ID, e)
		}
		return domain.ErrValidation
	}
	if v.TemplateID == "" {
		return fn(ctx)
	}
	return inTransaction(ctx, r.store, fn)
}
func (r *Repository) ListTranscriptReportCards(ctx context.Context, t, student string) ([]*domain.ReportCard, error) {
	s, err := r.cards.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	cur,
		err := s.Find(ctx,
		live(bson.M{"data.studentid": student,
			"data.status": bson.M{"$in": bson.A{"published",
				"archived"}}}),
		options.Find().SetSort(bson.D{{Key: "data.createdat",
			Value: 1},
			{Key: "_id",
				Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Warn("close Mongo cursor", "error", err)
		}
	}()
	out := []*domain.ReportCard{}
	for cur.Next(ctx) {
		var d record[domain.ReportCard]
		if err = cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, &d.Data)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].GeneratedAt, out[j].GeneratedAt
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.Before(*b)
	})
	return out, cur.Err()
}
func (r *Repository) FindDraftReportCard(ctx context.Context, t, student, term string) (*domain.ReportCard, error) {
	s, err := r.cards.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	q := live(bson.M{"data.studentid": student, "data.status": "draft"})
	if term != "" {
		q["data.termid"] = term
		var exact record[domain.ReportCard]
		err = s.FindOne(ctx, q, options.FindOne().SetSort(bson.D{{Key: "data.createdat", Value: -1}, {Key: "_id", Value: -1}})).Decode(&exact)
		if err == nil {
			return &exact.Data, nil
		}
		if !errors.Is(mapped(err), domain.ErrNotFound) {
			return nil, err
		}
		q["data.termid"] = ""
	}
	var d record[domain.ReportCard]
	if err = s.FindOne(ctx, q, options.FindOne().SetSort(bson.D{{Key: "data.createdat", Value: -1}, {Key: "_id", Value: -1}})).Decode(&d); err != nil {
		return nil, mapped(err)
	}
	return &d.Data, nil
}

type generation struct {
	State    string     `bson:"state"`
	Attempts int        `bson:"attempts"`
	Lease    *time.Time `bson:"lease"`
	Next     time.Time  `bson:"next"`
}

func (r *Repository) EnqueueReportGeneration(ctx context.Context, t, id string) (*domain.ReportCard, error) {
	s, err := r.cards.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res,
		err := s.UpdateOne(ctx,
		live(bson.M{"_id": id,
			"data.status": bson.M{"$in": bson.A{"draft",
				"published"}}}),
		bson.M{"$set": bson.M{"data.status": "generating",
			"data.updatedat": now,
			"generation": generation{State: "queued",
				Next: now}}})
	if err != nil {
		return nil, mapped(err)
	}
	card, err := r.GetReportCardByID(ctx, t, id)
	if err != nil {
		return nil, err
	}
	if res.MatchedCount != 1 {
		return nil, domain.ErrConflict
	}
	return card, nil
}

// ClaimReportGeneration is an optimistic compare-and-swap on the card's embedded queue record.
// Attempts and the exact lease fence late workers even after crash/reclaim.
func (r *Repository) ClaimReportGeneration(ctx context.Context, lease time.Duration) (*domain.GenerationJob, error) {
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	admin := auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	s, err := r.cards.Scope(admin)
	if err != nil {
		return nil, err
	}
	for range 20 {
		now := time.Now().UTC()
		q := live(bson.M{"data.status": "generating",
			"$or": bson.A{bson.M{"generation.state": "queued",
				"generation.next": bson.M{"$lte": now}},
				bson.M{"generation.state": "running",
					"generation.lease": bson.M{"$lt": now}}}})
		var d struct {
			ID       string     `bson:"_id"`
			TenantID string     `bson:"tenant_id"`
			Job      generation `bson:"generation"`
		}
		if err = s.FindOne(ctx, q, options.FindOne().SetSort(bson.D{{Key: "generation.next", Value: 1}, {Key: "_id", Value: 1}})).Decode(&d); err != nil {
			return nil, mapped(err)
		}
		expires := now.Add(lease).Truncate(time.Millisecond)
		res,
			err := s.UpdateOne(ctx,
			live(bson.M{"_id": d.ID,
				"tenant_id":           d.TenantID,
				"data.status":         "generating",
				"generation.state":    d.Job.State,
				"generation.attempts": d.Job.Attempts,
				"generation.lease":    d.Job.Lease,
				"generation.next":     d.Job.Next}),
			bson.M{"$set": bson.M{"generation.state": "running",
				"generation.lease": expires},
				"$inc": bson.M{"generation.attempts": 1}})
		if err != nil {
			return nil, err
		}
		if res.MatchedCount == 1 {
			return &domain.GenerationJob{ReportCardID: d.ID, TenantID: d.TenantID, Attempts: d.Job.Attempts + 1, LeaseExpires: &expires, NextAttemptAt: d.Job.Next}, nil
		}
	}
	return nil, domain.ErrConflict
}
func jobFilter(j *domain.GenerationJob) bson.M {
	return live(bson.M{"_id": j.ReportCardID,
		"data.status":         "generating",
		"generation.state":    "running",
		"generation.attempts": j.Attempts,
		"generation.lease":    j.LeaseExpires,
		"$and":                bson.A{bson.M{"generation.lease": bson.M{"$gt": time.Now().UTC()}}}})
}
func (r *Repository) CompleteReportGeneration(ctx context.Context, j *domain.GenerationJob, path string) (*domain.ReportCard, error) {
	if j == nil || j.LeaseExpires == nil {
		return nil, domain.ErrConflict
	}
	s, err := r.cards.ScopeTo(j.TenantID)
	if err != nil {
		return nil, err
	}
	card, err := r.GetReportCardByID(ctx, j.TenantID, j.ReportCardID)
	if err != nil {
		return nil, err
	}
	card.SetPublished(path)
	e,
		err := event(j.TenantID,
		"report.published.v1",
		map[string]any{"report_card_id": card.ID,
			"student_id":       card.StudentID,
			"academic_year_id": card.AcademicYearID,
			"term_id":          card.TermID,
			"template_id":      card.TemplateID,
			"status":           card.Status,
			"file_url":         "/api/v1/report-cards/" + card.ID + "/download",
			"generated_at":     card.GeneratedAt.Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	res,
		err := s.UpdateWithEvents(ctx,
		jobFilter(j),
		bson.M{"$set": bson.M{"data.status": card.Status,
			"data.pdfpath":     path,
			"data.generatedat": card.GeneratedAt,
			"data.updatedat":   card.UpdatedAt,
			"generation.state": "completed",
			"generation.lease": nil}},
		e)
	if err != nil {
		return nil, mapped(err)
	}
	if res.MatchedCount != 1 {
		return nil, domain.ErrConflict
	}
	return card, nil
}
func (r *Repository) RetryReportGeneration(ctx context.Context, j *domain.GenerationJob, message string, maxAttempts int) (bool, error) {
	if j == nil || j.LeaseExpires == nil {
		return false, domain.ErrConflict
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	terminal := j.Attempts >= maxAttempts
	s, err := r.cards.ScopeTo(j.TenantID)
	if err != nil {
		return false, err
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	set := bson.M{"generation.state": "queued",
		"generation.lease": nil,
		"generation.error": message,
		"generation.next": time.Now().UTC().Add(time.Duration(math.Min(300,
			math.Pow(2,
				float64(j.Attempts)))) * time.Second)}
	if terminal {
		set["generation.state"] = "failed"
		set["data.status"] = "draft"
		set["data.updatedat"] = time.Now().UTC()
	}
	res, err := s.UpdateOne(ctx, jobFilter(j), bson.M{"$set": set})
	if err != nil {
		return false, mapped(err)
	}
	if res.MatchedCount != 1 {
		return false, domain.ErrConflict
	}
	return terminal, nil
}

func entryKey(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

// Natural keys become hashed map keys, allowing idempotent entry upserts in a
// single card write, with no cross-document parent/entry race or field injection.
func (r *Repository) upsertEntry(ctx context.Context, t, card, key string, v any) error {
	scope, err := r.cards.ScopeTo(t)
	if err != nil {
		return err
	}
	data, err := dataFields(v, t)
	if err != nil {
		return err
	}
	for range 10 {
		var current bson.Raw
		if err = scope.FindOne(ctx, live(bson.M{"_id": card}), options.FindOne().SetProjection(bson.M{key: 1})).Decode(&current); err != nil {
			return mapped(err)
		}
		parts := strings.SplitN(key, ".", 2)
		query := live(bson.M{"_id": card, key: bson.M{"$exists": false}})
		if raw, ok := current.Lookup(parts...).DocumentOK(); ok {
			var entry bson.M
			if err = bson.Unmarshal(raw, &entry); err != nil {
				return err
			}
			data["id"] = entry["id"]
			data["createdat"] = entry["createdat"]
			data["studentid"] = entry["studentid"]
			delete(query, key)
			query[key+".id"] = entry["id"]
		}
		result, err := scope.UpdateOne(ctx, query, bson.M{"$set": bson.M{key: data}})
		if err != nil {
			return mapped(err)
		}
		if result.MatchedCount == 1 {
			return nil
		}
	}
	return domain.ErrConflict
}
func (r *Repository) UpsertScoreEntry(ctx context.Context, t string, e *domain.ScoreEntry) error {
	return r.upsertEntry(ctx, t, e.ReportCardID, "scores."+entryKey(e.SourceKey), e)
}
func (r *Repository) UpsertAttendanceEntry(ctx context.Context, t string, e *domain.AttendanceEntry) error {
	cloned := *e
	day := e.Date.UTC().Format(time.DateOnly)
	cloned.Date = e.Date.UTC().Truncate(24 * time.Hour)
	return r.upsertEntry(ctx, t, e.ReportCardID, "attendance."+entryKey(day), &cloned)
}
func (r *Repository) ListScoreEntries(ctx context.Context, t, id string) ([]*domain.ScoreEntry, error) {
	s, err := r.cards.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	var d struct {
		Entries map[string]*domain.ScoreEntry `bson:"scores"`
	}
	if err = s.FindOne(ctx, live(bson.M{"_id": id})).Decode(&d); err != nil {
		return nil, mapped(err)
	}
	out := make([]*domain.ScoreEntry, 0, len(d.Entries))
	for _, v := range d.Entries {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}
func (r *Repository) ListAttendanceEntries(ctx context.Context, t, id string) ([]*domain.AttendanceEntry, error) {
	s, err := r.cards.ScopeTo(t)
	if err != nil {
		return nil, err
	}
	var d struct {
		Entries map[string]*domain.AttendanceEntry `bson:"attendance"`
	}
	if err = s.FindOne(ctx, live(bson.M{"_id": id})).Decode(&d); err != nil {
		return nil, mapped(err)
	}
	out := make([]*domain.AttendanceEntry, 0, len(d.Entries))
	for _, v := range d.Entries {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}
func (r *Repository) ClaimPendingReportEvents(ctx context.Context, n int) ([]ports.OutboxEvent, error) {
	if n <= 0 || n > 100 {
		n = 25
	}
	out := []ports.OutboxEvent{}
	for _, o := range r.outboxes {
		if len(out) == n {
			break
		}
		items, err := o.Claim(ctx, n-len(out))
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			created, err := time.Parse(time.RFC3339Nano, i.Event.Time)
			if err != nil {
				return nil, fmt.Errorf("invalid outbox event time: %w", err)
			}
			out = append(out,
				ports.OutboxEvent{ID: i.Event.ID,
					TenantID:  i.Event.TenantID,
					EventType: i.Event.Type,
					Payload:   json.RawMessage(i.Event.Data),
					CreatedAt: created})
		}
	}
	return out, nil
}
func (r *Repository) MarkReportEventPublished(ctx context.Context, id string) error {
	for _, o := range r.outboxes {
		if err := o.MarkPublishedByEventID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *Repository) MarkReportEventFailed(ctx context.Context, id, reason string) error {
	for _, o := range r.outboxes {
		if err := o.MarkFailedByEventID(ctx, id, reason); err != nil {
			return err
		}
	}
	return nil
}
func EnsureIndexes(ctx context.Context, s *pmongo.Store) error {
	return indexes(ctx, s, map[string][]mongo.IndexModel{
		TemplateCollection: {{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "data.createdat", Value: 1}, {Key: "_id", Value: 1}}}},
		CardCollection: {{Keys: bson.D{{Key: "tenant_id",
			Value: 1},
			{Key: "data.studentid",
				Value: 1},
			{Key: "data.termid",
				Value: 1}},
			Options: options.Index().SetName("auto_draft_unique").SetUnique(true).SetPartialFilterExpression(bson.M{"data.status": "draft",
				"data.templateid": "",
				"deleted_at":      nil})},
			{Keys: bson.D{{Key: "generation.state",
				Value: 1},
				{Key: "generation.next",
					Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id",
				Value: 1},
				{Key: "data.createdat",
					Value: 1},
				{Key: "_id",
					Value: 1}}}},
	})
}

func (r *Repository) deleteTemplate(ctx context.Context, t, id string, events ...tenancy.CloudEvent) error {
	return inTransaction(ctx, r.store, func(ctx context.Context) error {
		if err := remove(ctx, r.templates, t, id, events...); err != nil {
			return err
		}
		scope, err := r.cards.ScopeTo(t)
		if err != nil {
			return err
		}
		n, err := scope.CountDocuments(ctx, live(bson.M{"data.templateid": id}))
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrConflict
		}
		return nil
	})
}
