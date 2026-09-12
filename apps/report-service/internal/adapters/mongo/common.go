// Package mongo implements tenant-scoped persistence using atomic aggregate documents.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Domain values are stored under data using BSON field names (lowercase Go
// names). BSON preserves private-to-HTTP fields and timestamps without a lossy
// JSON roundtrip. Delivery metadata lives beside data, never replaced by CRUD.
type record[T any] struct {
	ID       string `bson:"_id"`
	TenantID string `bson:"tenant_id"`
	Data     T      `bson:"data"`
}

func dataFields(v any, tenant string) (bson.M, error) {
	raw, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}
	var d bson.M
	if err = bson.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if _, ok := d["tenantid"]; ok {
		d["tenantid"] = tenant
	}
	return d, nil
}
func live(q bson.M) bson.M { q["deleted_at"] = bson.M{"$exists": false}; return q }
func mapped(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrConflict
	}
	return err
}
func changed(r *mongo.UpdateResult, err error) error {
	if err != nil {
		return mapped(err)
	}
	if r.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}
func event(tenant, kind string, payload map[string]any) (tenancy.CloudEvent, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, err
	}
	return tenancy.CloudEvent{SpecVersion: "1.0",
			ID:       uuid.NewString(),
			Type:     kind,
			Source:   "report-service",
			TenantID: tenant,
			Time:     time.Now().UTC().Format(time.RFC3339Nano),
			Data:     data},
		nil
}
func insert(ctx context.Context, c *pmongo.Collection, tenant, id string, v any, events ...tenancy.CloudEvent) error {
	s, err := c.ScopeTo(tenant)
	if err != nil {
		return err
	}
	d, err := dataFields(v, tenant)
	if err != nil {
		return err
	}
	_, err = s.InsertWithEvents(ctx, bson.M{"_id": id, "data": d}, events...)
	return mapped(err)
}
func update(ctx context.Context, c *pmongo.Collection, tenant, id string, v any, events ...tenancy.CloudEvent) error {
	s, err := c.ScopeTo(tenant)
	if err != nil {
		return err
	}
	d, err := dataFields(v, tenant)
	if err != nil {
		return err
	}
	return changed(s.UpdateWithEvents(ctx, live(bson.M{"_id": id}), bson.M{"$set": bson.M{"data": d}}, events...))
}
func remove(ctx context.Context, c *pmongo.Collection, tenant, id string, events ...tenancy.CloudEvent) error {
	s, err := c.ScopeTo(tenant)
	if err != nil {
		return err
	}
	// Keep tombstones so deleting an aggregate cannot erase undispatched events.
	return changed(s.UpdateWithEvents(ctx, live(bson.M{"_id": id}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, events...))
}
func get[T any](ctx context.Context, c *pmongo.Collection, tenant string, q bson.M) (*T, error) {
	s, err := c.ScopeTo(tenant)
	if err != nil {
		return nil, err
	}
	var d record[T]
	if err = s.FindOne(ctx, live(q)).Decode(&d); err != nil {
		return nil, mapped(err)
	}
	return &d.Data, nil
}
func list[T any](ctx context.Context, c *pmongo.Collection, tenant string, q bson.M, limit int, cursor string) ([]*T, string, error) {
	s, err := c.ScopeTo(tenant)
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if cursor != "" {
		var prev struct {
			Data struct {
				CreatedAt time.Time `bson:"createdat"`
			} `bson:"data"`
		}
		if err = s.FindOne(ctx, live(bson.M{"_id": cursor})).Decode(&prev); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return []*T{}, "", nil
			}
			return nil, "", err
		}
		q["$and"] = bson.A{bson.M{"$or": bson.A{bson.M{"data.createdat": bson.M{"$gt": prev.Data.CreatedAt}},
			bson.M{"data.createdat": prev.Data.CreatedAt,
				"_id": bson.M{"$gt": cursor}}}}}
	}
	cur, err := s.Find(ctx, live(q), options.Find().SetSort(bson.D{{Key: "data.createdat", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Warn("close Mongo cursor", "error", err)
		}
	}()
	out := []*T{}
	last := ""
	for cur.Next(ctx) {
		var d record[T]
		if err = cur.Decode(&d); err != nil {
			return nil, "", err
		}
		out = append(out, &d.Data)
		last = d.ID
	}
	if len(out) < limit {
		last = ""
	}
	return out, last, cur.Err()
}
func equal(q bson.M, key, value string) {
	if value != "" {
		q["data."+key] = value
	}
}
func indexes(ctx context.Context, s *pmongo.Store, specs map[string][]mongo.IndexModel) error {
	for name, models := range specs {
		if _, err := s.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("report: index %s: %w", name, err)
		}
	}
	return nil
}
