package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// A few collections hold no tenant-owned data at all. The onboarding queue is
// the example: a school applying to join has no tenant yet, and its PostgreSQL
// table is marked rls-exempt for exactly that reason.
//
// Those collections still must not fall out of the scoping rules by accident.
// PlatformOwned makes the exemption explicit and greppable, so "this record has
// no tenant" is a decision written in the code rather than a Scope someone
// forgot. A collection is either tenant-scoped or declared platform-owned;
// there is no third path that simply skips the check.

// PlatformScope is an unscoped view of a collection that, by declaration, holds
// no tenant-owned records.
type PlatformScope struct {
	coll *mongo.Collection
}

// PlatformOwned declares that this collection holds no tenant-owned data.
//
// Do not use it to reach tenant data from a privileged caller — Scope already
// widens correctly for a platform super admin while still refusing an ordinary
// caller. This is only for records that have no owner in the first place.
func (c *Collection) PlatformOwned() *PlatformScope {
	return &PlatformScope{coll: c.coll}
}

func (p *PlatformScope) FindOne(ctx context.Context, query bson.M, opts ...options.Lister[options.FindOneOptions]) *mongo.SingleResult {
	return p.coll.FindOne(ctx, query, opts...)
}

func (p *PlatformScope) Find(ctx context.Context, query bson.M, opts ...options.Lister[options.FindOptions]) (*mongo.Cursor, error) {
	return p.coll.Find(ctx, query, opts...)
}

func (p *PlatformScope) CountDocuments(ctx context.Context, query bson.M, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return p.coll.CountDocuments(ctx, query, opts...)
}

func (p *PlatformScope) InsertOne(ctx context.Context, doc bson.M, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	return p.coll.InsertOne(ctx, doc, opts...)
}

func (p *PlatformScope) UpdateOne(ctx context.Context, query bson.M, update bson.M, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return p.coll.UpdateOne(ctx, query, update, opts...)
}

func (p *PlatformScope) DeleteOne(ctx context.Context, query bson.M, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return p.coll.DeleteOne(ctx, query, opts...)
}

func (p *PlatformScope) FindOneAndUpdate(ctx context.Context, query bson.M, update bson.M, opts ...options.Lister[options.FindOneAndUpdateOptions]) *mongo.SingleResult {
	return p.coll.FindOneAndUpdate(ctx, query, update, opts...)
}
