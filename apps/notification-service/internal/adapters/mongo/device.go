package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
)

type DeviceTokenRepository struct{ *base }

func NewDeviceTokenRepository(s *pmongo.Store) *DeviceTokenRepository {
	return &DeviceTokenRepository{&base{s}}
}

var _ ports.DeviceTokenRepository = (*DeviceTokenRepository)(nil)

func (r *DeviceTokenRepository) Upsert(ctx context.Context, tenant string, input *domain.DeviceToken) (*domain.DeviceToken, error) {
	var result domain.DeviceToken
	err := r.tx(ctx, func(ctx context.Context) error {
		scope, err := r.store.Collection("device_push_tokens").ScopeTo(tenant)
		if err != nil {
			return err
		}
		if _, err = scope.DeleteMany(ctx, bson.M{"body.token": input.Token,
			"$or": bson.A{bson.M{"body.userid": bson.M{"$ne": input.UserID}},
				bson.M{"body.deviceid": bson.M{"$ne": input.DeviceID}}}}); err != nil {
			return err
		}
		q := bson.M{"body.userid": input.UserID, "body.deviceid": input.DeviceID}
		old, err := get[domain.DeviceToken](ctx, r.base, tenant, "device_push_tokens", q)
		result = *input
		result.TenantID = tenant
		result.Status = "active"
		if err == nil {
			result.ID = old.ID
			result.CreatedAt = old.CreatedAt
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		res, err := scope.UpsertOne(ctx, q, bson.M{"$set": bson.M{"body": result, "deleted": false}, "$setOnInsert": bson.M{"_id": result.ID}})
		_ = res
		return err
	})
	return &result, mapped(err)
}
func (r *DeviceTokenRepository) DeleteByDevice(ctx context.Context, tenant, user, device string) error {
	scope, err := r.store.Collection("device_push_tokens").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.DeleteOne(ctx, bson.M{"body.userid": user, "body.deviceid": device})
	return err
}
func (r *DeviceTokenRepository) ListActive(ctx context.Context, tenant, user string) ([]*domain.DeviceToken, error) {
	out, _, err := list[domain.DeviceToken](ctx, r.base, tenant, "device_push_tokens", "", 1000, bson.M{"body.userid": user, "body.status": "active"})
	return out, err
}
func (r *DeviceTokenRepository) MarkInvalid(ctx context.Context, tenant, token string) error {
	scope, err := r.store.Collection("device_push_tokens").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.UpdateMany(ctx, bson.M{"body.token": token}, bson.M{"$set": bson.M{"body.status": "invalid", "body.updatedat": time.Now().UTC()}})
	return err
}

type ProcessedEventRepository struct{ *base }

func NewProcessedEventRepository(s *pmongo.Store) *ProcessedEventRepository {
	return &ProcessedEventRepository{&base{s}}
}

var _ ports.ProcessedEventRepository = (*ProcessedEventRepository)(nil)

func (r *ProcessedEventRepository) Claim(ctx context.Context, tenant, id, kind string) (bool, error) {
	scope, err := r.store.Collection("notification_processed_events").ScopeTo(tenant)
	if err != nil {
		return false, err
	}
	_, err = scope.InsertOne(ctx, bson.M{"_id": tenant + ":" + id, "event_id": id, "event_type": kind})
	if driver.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}
func (r *ProcessedEventRepository) Release(ctx context.Context, tenant, id string) error {
	scope, err := r.store.Collection("notification_processed_events").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.DeleteOne(ctx, bson.M{"_id": tenant + ":" + id})
	return err
}
