// Package mongo implements the admissions ports. Documents and lifecycle events
// commit in one write; catalogue validation uses a replica-set transaction.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/admissions-service/internal/ports"
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
var _ ports.TransactionalRepository = (*Repository)(nil)
var _ ports.CatalogueRepository = (*Repository)(nil)
var _ ports.CatalogueTransactionalRepository = (*Repository)(nil)
var _ ports.CatalogueEventRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

func mapped(e error) error {
	if errors.Is(e, driver.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if driver.IsDuplicateKeyError(e) {
		return domain.ErrConflict
	}
	return e
}
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	for collection, keys := range map[string][][]string{"applications": {{"body.applicantuserid",
		"body.programmeid", "body.intakeid"}}, "admissions_programmes": {{"body.code"},
		{"body.slug"}}, "admissions_intakes": {{"body.programmeid",
		"body.name", "body.startsat"}}} {
		for _, fields := range keys {
			k := make(bson.D, 0, 1+len(fields))
			k = append(k, bson.E{Key: "tenant_id", Value: 1})
			for _, f := range fields {
				k = append(k, bson.E{Key: f, Value: 1})
			}
			if _, err := r.store.Database().Collection(collection).Indexes().CreateOne(ctx,
				driver.IndexModel{Keys: k, Options: options.Index().SetUnique(true)}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Repository) tx(ctx context.Context, fn func(context.Context) error) error {
	return mapped(r.store.WithTransaction(ctx, fn))
}

func event(tenant, kind string, payload map[string]any) ([]tenancy.CloudEvent, error) {
	if kind == "" {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return []tenancy.CloudEvent{{SpecVersion: "1.0", ID: uuid.NewString(),
			Type: kind, Source: "admissions-service", TenantID: tenant,
			Time: time.Now().UTC().Format(time.RFC3339), Data: data}},
		nil
}
func (r *Repository) write(ctx context.Context, tenant, collection,
	id string, body any, create bool, expected bson.M, kind string,
	payload map[string]any) error {
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return err
	}
	events, err := event(tenant, kind, payload)
	if err != nil {
		return err
	}
	if create {
		_, err = scope.InsertWithEvents(ctx, bson.M{"_id": id, "body": body}, events...)
		return mapped(err)
	}
	expected["_id"] = id
	res, err := scope.UpdateWithEvents(ctx, expected, bson.M{"$set": bson.M{"body": body}}, events...)
	if err != nil {
		return mapped(err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrConflict
	}
	return nil
}
func get[T any](ctx context.Context, r *Repository, tenant, collection, id string) (T, error) {
	var doc struct {
		Body T `bson:"body"`
	}
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return doc.Body, err
	}
	err = scope.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	return doc.Body, mapped(err)
}
func list[T any](ctx context.Context, r *Repository, tenant, collection string, q bson.M, sort bson.D, limit int) ([]T, error) {
	scope, err := r.store.Collection(collection).ScopeTo(tenant)
	if err != nil {
		return nil, err
	}
	opts := options.Find().SetSort(sort)
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cur, err := scope.Find(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cur)
	out := make([]T, 0)
	for cur.Next(ctx) {
		var d struct {
			Body T `bson:"body"`
		}
		if err = cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, d.Body)
	}
	return out, cur.Err()
}
func (r *Repository) Create(ctx context.Context, a domain.Application) error {
	return r.CreateWithEvent(ctx, a, "", nil)
}
func (r *Repository) CreateWithEvent(ctx context.Context, a domain.Application, k string, p map[string]any) error {
	return r.write(ctx, a.TenantID, "applications", a.ID, a, true, nil, k, p)
}
func (r *Repository) Get(ctx context.Context, t, id string) (domain.Application, error) {
	return get[domain.Application](ctx, r, t, "applications", id)
}
func (r *Repository) List(ctx context.Context, t, applicant string, status domain.Status, limit int) ([]domain.Application, error) {
	q := bson.M{}
	if applicant != "" {
		q["body.applicantuserid"] = applicant
	}
	if status != "" {
		q["body.status"] = status
	}
	return list[domain.Application](ctx, r, t, "applications", q, bson.D{{Key: "body.createdat", Value: -1}, {Key: "_id", Value: -1}}, limit)
}
func (r *Repository) Update(ctx context.Context, a domain.Application, e domain.Status) error {
	return r.UpdateWithEvent(ctx, a, e, "", nil)
}
func (r *Repository) UpdateWithEvent(ctx context.Context, a domain.Application, e domain.Status, k string, p map[string]any) error {
	q := bson.M{"body.status": e}
	if k == "offer.issued.v1" {
		q["body.offerstatus"] = "none"
	}
	if k == "offer.accepted.v1" {
		q["body.offerstatus"] = "issued"
	}
	return r.write(ctx, a.TenantID, "applications", a.ID, a, false, q, k, p)
}
func (r *Repository) CreateProgramme(ctx context.Context, p domain.Programme) error {
	return r.CreateProgrammeWithEvent(ctx, p, "", nil)
}
func (r *Repository) CreateProgrammeWithEvent(ctx context.Context, p domain.Programme, k string, data map[string]any) error {
	p.Intakes = nil
	return r.write(ctx, p.TenantID, "admissions_programmes", p.ID, p, true, nil, k, data)
}
func (r *Repository) GetProgramme(ctx context.Context, t, id string) (domain.Programme, error) {
	p, err := get[domain.Programme](ctx, r, t, "admissions_programmes", id)
	if err == nil {
		p.Intakes, err = r.intakes(ctx, t, id, false, time.Time{})
	}
	return p, err
}
func (r *Repository) intakes(ctx context.Context, t, id string, public bool, now time.Time) ([]domain.Intake, error) {
	q := bson.M{"body.programmeid": id}
	if public {
		q["body.status"] = domain.IntakeOpen
		q["body.applicationopensat"] = bson.M{"$lte": now}
		q["body.applicationclosesat"] = bson.M{"$gt": now}
	}
	return list[domain.Intake](ctx, r, t, "admissions_intakes", q, bson.D{{Key: "body.startsat", Value: 1}, {Key: "_id", Value: 1}}, 0)
}
func (r *Repository) ListProgrammes(ctx context.Context, t string, public bool, now time.Time, limit int) ([]domain.Programme, error) {
	q := bson.M{}
	if public {
		q["body.status"] = domain.ProgrammePublished
	}
	ps, err := list[domain.Programme](ctx, r, t, "admissions_programmes", q, bson.D{{Key: "body.name", Value: 1}, {Key: "_id", Value: 1}}, limit)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Programme, 0, len(ps))
	for _, p := range ps {
		p.Intakes, err = r.intakes(ctx, t, p.ID, public, now)
		if err != nil {
			return nil, err
		}
		if !public || len(p.Intakes) > 0 {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *Repository) UpdateProgramme(ctx context.Context, p domain.Programme, v int) error {
	return r.UpdateProgrammeWithEvent(ctx, p, v, "", nil)
}
func (r *Repository) UpdateProgrammeWithEvent(ctx context.Context, p domain.Programme, v int, k string, data map[string]any) error {
	p.Intakes = nil
	return r.write(ctx, p.TenantID, "admissions_programmes", p.ID, p, false, bson.M{"body.version": v}, k, data)
}
func (r *Repository) CreateIntake(ctx context.Context, i domain.Intake) error {
	return r.CreateIntakeWithEvent(ctx, i, "", nil)
}
func (r *Repository) CreateIntakeWithEvent(ctx context.Context, i domain.Intake, k string, data map[string]any) error {
	if _, err := get[domain.Programme](ctx, r, i.TenantID, "admissions_programmes", i.ProgrammeID); err != nil {
		return err
	}
	return r.write(ctx, i.TenantID, "admissions_intakes", i.ID, i, true, nil, k, data)
}
func (r *Repository) GetIntake(ctx context.Context, t, id string) (domain.Intake, error) {
	return get[domain.Intake](ctx, r, t, "admissions_intakes", id)
}
func (r *Repository) UpdateIntake(ctx context.Context, i domain.Intake, v int) error {
	return r.UpdateIntakeWithEvent(ctx, i, v, "", nil)
}
func (r *Repository) UpdateIntakeWithEvent(ctx context.Context, i domain.Intake, v int, k string, data map[string]any) error {
	return r.write(ctx, i.TenantID, "admissions_intakes", i.ID, i, false, bson.M{"body.version": v}, k, data)
}
func (r *Repository) ResolveAvailableIntake(ctx context.Context, t, pid, iid string, now time.Time) (domain.Programme, domain.Intake, error) {
	p, err := get[domain.Programme](ctx, r, t, "admissions_programmes", pid)
	if err != nil {
		return p, domain.Intake{}, err
	}
	i, err := r.GetIntake(ctx, t, iid)
	if err != nil {
		return p, i, err
	}
	if p.Status != domain.ProgrammePublished || i.ProgrammeID != pid || i.Status != domain.IntakeOpen ||
		i.ApplicationOpensAt.After(now) || !i.ApplicationClosesAt.After(now) {
		return p, i, domain.ErrNotFound
	}
	p.Intakes = []domain.Intake{i}
	return p, i, nil
}
func (r *Repository) CreateForAvailableIntake(ctx context.Context, a domain.Application, now time.Time, k string, data map[string]any) error {
	return r.tx(ctx, func(ctx context.Context) error {
		p, i, err := r.ResolveAvailableIntake(ctx, a.TenantID, a.ProgrammeID, a.IntakeID, now)
		if err != nil {
			return err
		}
		// Write fences serialize admission against concurrent catalogue closure.
		for collection, id := range map[string]string{"admissions_programmes": p.ID, "admissions_intakes": i.ID} {
			scope, err := r.store.Collection(collection).ScopeTo(a.TenantID)
			if err != nil {
				return err
			}
			if _, err = scope.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"admission_fence": 1}}); err != nil {
				return err
			}
		}
		a.ProgrammeName = p.Name
		a.IntakeName = i.Name
		return r.CreateWithEvent(ctx, a, k, data)
	})
}

func collections() []string {
	return []string{"applications", "admissions_programmes", "admissions_intakes"}
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
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
			at, err := time.Parse(time.RFC3339, item.Event.Time)
			if err != nil {
				return nil, err
			}
			out = append(out, ports.OutboxEvent{ID: item.Event.ID, TenantID: item.Event.TenantID, EventType: item.Event.Type, Payload: item.Event.Data, CreatedAt: at})
		}
	}
	return out, nil
}
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	for _, name := range collections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkPublishedByEventID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	for _, name := range collections() {
		if err := pmongo.NewClaimableOutbox(r.store.Collection(name), 0).MarkFailedByEventID(ctx, id, message); err != nil {
			return err
		}
	}
	return nil
}
