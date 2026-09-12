// Package mongo implements notification persistence with embedded durable events.
package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type base struct{ store *pmongo.Store }

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
func (r *base) write(ctx context.Context, tenant, collection, id string, body any, create, remove bool, events []tenancy.CloudEvent) error {
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
func get[T any](ctx context.Context, r *base, tenant, collection string, q bson.M) (*T, error) {
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
func list[T any](ctx context.Context, r *base, tenant, collection, cursor string, limit int, query bson.M) ([]*T, string, error) {
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

type MessageRepository struct{ *base }

var _ ports.MessageRepository = (*MessageRepository)(nil)

func NewMessageRepository(s *pmongo.Store) *MessageRepository { return &MessageRepository{&base{s}} }
func (r *MessageRepository) Create(ctx context.Context, tenant string, v *domain.Message) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "messages", v.ID, v, true, false, nil)
}
func (r *MessageRepository) GetByID(ctx context.Context, tenant, id string) (*domain.Message, error) {
	return get[domain.Message](ctx, r.base, tenant, "messages", live(id))
}
func (r *MessageRepository) Delete(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "messages", id, nil, false, true, nil)
}
func (r *MessageRepository) Update(ctx context.Context, tenant string, v *domain.Message) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "messages", v.ID, v, false, false, nil)
}
func (r *MessageRepository) List(ctx context.Context, tenant string, f ports.MessageFilter) ([]*domain.Message, string, error) {
	q := bson.M{}
	if f.Channel != "" {
		q["body.channel"] = f.Channel
	}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	if f.RecipientID != "" {
		q["body.recipientid"] = f.RecipientID
	}
	return list[domain.Message](ctx, r.base, tenant, "messages", f.Cursor, f.Limit, q)
}

type TemplateRepository struct{ *base }

var _ ports.TemplateRepository = (*TemplateRepository)(nil)

func NewTemplateRepository(s *pmongo.Store) *TemplateRepository { return &TemplateRepository{&base{s}} }
func (r *TemplateRepository) Create(ctx context.Context, tenant string, v *domain.Template) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "notification_templates", v.ID, v, true, false, nil)
}
func (r *TemplateRepository) GetByID(ctx context.Context, tenant, id string) (*domain.Template, error) {
	return get[domain.Template](ctx, r.base, tenant, "notification_templates", live(id))
}
func (r *TemplateRepository) Delete(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "notification_templates", id, nil, false, true, nil)
}
func (r *TemplateRepository) Update(ctx context.Context, tenant string, v *domain.Template) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "notification_templates", v.ID, v, false, false, nil)
}
func (r *TemplateRepository) List(ctx context.Context, tenant string, f ports.TemplateFilter) ([]*domain.Template, string, error) {
	q := bson.M{}
	if f.Channel != "" {
		q["body.channel"] = f.Channel
	}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	return list[domain.Template](ctx, r.base, tenant, "notification_templates", f.Cursor, f.Limit, q)
}

type SubscriptionRepository struct{ *base }

var _ ports.SubscriptionRepository = (*SubscriptionRepository)(nil)

func NewSubscriptionRepository(s *pmongo.Store) *SubscriptionRepository {
	return &SubscriptionRepository{&base{s}}
}
func (r *SubscriptionRepository) Create(ctx context.Context, tenant string, v *domain.Subscription) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "notification_subscriptions", v.ID, v, true, false, nil)
}
func (r *SubscriptionRepository) GetByID(ctx context.Context, tenant, id string) (*domain.Subscription, error) {
	return get[domain.Subscription](ctx, r.base, tenant, "notification_subscriptions", live(id))
}
func (r *SubscriptionRepository) Delete(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "notification_subscriptions", id, nil, false, true, nil)
}
func (r *SubscriptionRepository) Update(ctx context.Context, tenant string, v *domain.Subscription) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "notification_subscriptions", v.ID, v, false, false, nil)
}
func (r *SubscriptionRepository) List(ctx context.Context, tenant string, f ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	q := bson.M{}
	if f.Channel != "" {
		q["body.channel"] = f.Channel
	}
	if f.UserID != "" {
		q["body.userid"] = f.UserID
	}
	return list[domain.Subscription](ctx, r.base, tenant, "notification_subscriptions", f.Cursor, f.Limit, q)
}

type AnnouncementRepository struct{ *base }

var _ ports.AnnouncementRepository = (*AnnouncementRepository)(nil)

func NewAnnouncementRepository(s *pmongo.Store) *AnnouncementRepository {
	return &AnnouncementRepository{&base{s}}
}
func (r *AnnouncementRepository) Create(ctx context.Context, tenant string, v *domain.Announcement) error {
	v.TenantID = tenant
	return r.write(ctx, tenant, "announcements", v.ID, v, true, false, nil)
}
func (r *AnnouncementRepository) GetByID(ctx context.Context, tenant, id string) (*domain.Announcement, error) {
	return get[domain.Announcement](ctx, r.base, tenant, "announcements", live(id))
}
func (r *AnnouncementRepository) Delete(ctx context.Context, tenant, id string) error {
	return r.write(ctx, tenant, "announcements", id, nil, false, true, nil)
}
func (r *AnnouncementRepository) List(ctx context.Context, tenant string, f ports.AnnouncementFilter) ([]*domain.Announcement, string, error) {
	q := bson.M{}
	if f.Audience != "" {
		q["body.audience"] = f.Audience
	}
	if f.Audience == "" && len(f.Audiences) > 0 {
		q["body.audience"] = bson.M{"$in": f.Audiences}
	}
	return list[domain.Announcement](ctx, r.base, tenant, "announcements", f.Cursor, f.Limit, q)
}

func EnsureIndexes(ctx context.Context, s *pmongo.Store) error {
	for collection, sets := range map[string][][]string{"notification_subscriptions": {{"tenant_id",
		"body.userid", "body.channel"}}, "device_push_tokens": {{"tenant_id",
		"body.userid", "body.deviceid"}, {"body.token"}}, "communication_journey_enrollments": {{"tenant_id",
		"body.journeyid", "body.eventid"}}, "notification_email_suppressions": {{"tenant_id",
		"address_hash"}}} {
		for _, fields := range sets {
			keys := make(bson.D, 0, len(fields))
			for _, f := range fields {
				keys = append(keys, bson.E{Key: f, Value: 1})
			}
			if _, err := s.Database().Collection(collection).Indexes().CreateOne(ctx, driver.IndexModel{Keys: keys, Options: uniqueOptions(collection)}); err != nil {
				return err
			}
		}
	}
	_, err := s.Database().Collection("messages").Indexes().CreateOne(ctx,
		driver.IndexModel{Keys: bson.D{{Key: "body.provider",
			Value: 1}, {Key: "provider_message_id", Value: 1}}, Options: options.Index().SetUnique(true).
			SetPartialFilterExpression(bson.M{"body.provider": bson.M{"$type": "string"},
				"provider_message_id": bson.M{"$gt": ""}})})
	return err
}
func (r *base) tx(ctx context.Context, fn func(context.Context) error) error {
	return r.store.WithTransaction(ctx, fn)
}

func uniqueOptions(collection string) *options.IndexOptionsBuilder {
	opts := options.Index().SetUnique(true)
	if collection == "notification_subscriptions" {
		opts.
			SetPartialFilterExpression(bson.M{"deleted": false})
	}
	return opts
}
