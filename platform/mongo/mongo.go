// Package mongo is the MongoDB persistence driver for AuraEDU services.
//
// It exists alongside platform/db rather than replacing it: a service selects a
// driver at runtime (see platform/store), so the PostgreSQL adapters stay intact
// and switching back is a configuration change.
//
// # Tenant isolation
//
// PostgreSQL enforces tenant isolation in the database, with row-level security
// making an unscoped query impossible no matter what the application asks for.
// MongoDB has no equivalent, so that guarantee has to move into the type system:
// a Collection cannot be queried at all. Every read and write goes through a
// Scope, which is obtained from a request context, carries the tenant filter on
// every operation, and never hands out the underlying driver collection. Code
// that forgets to scope a query does not compile rather than leaking a tenant.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TenantField is the document field every tenant-owned collection scopes on. It
// mirrors the tenant_id column the PostgreSQL adapters filter and RLS enforces.
const TenantField = "tenant_id"

// ErrTenantRequired is returned when a caller reaches persistence without a
// resolved tenant and without platform-admin authority. It is deliberately a
// hard error: an unscoped query is a cross-tenant read, so this fails closed.
var ErrTenantRequired = errors.New("mongo: tenant context is required")

type Config struct {
	URI      string
	Database string
	// ConnectTimeout bounds the initial connection and ping.
	ConnectTimeout time.Duration
	// MaxPoolSize bounds connections per service. The free Atlas tier shares a
	// small connection allowance across the whole fleet, so this matters for the
	// same reason DATABASE_MAX_CONNS does on PostgreSQL.
	MaxPoolSize uint64
}

type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

// Open connects, verifies the connection and returns a Store.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.URI == "" {
		return nil, errors.New("mongo: URI is required")
	}
	if cfg.Database == "" {
		return nil, errors.New("mongo: database name is required")
	}
	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts := options.Client().ApplyURI(cfg.URI).SetConnectTimeout(timeout)
	if cfg.MaxPoolSize > 0 {
		opts = opts.SetMaxPoolSize(cfg.MaxPoolSize)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping: %w", err)
	}
	return &Store{client: client, db: client.Database(cfg.Database)}, nil
}

func (s *Store) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}

// Database exposes the raw handle for schema work only — index creation and
// migrations. It is deliberately not a query path: everything that reads or
// writes documents goes through Collection/Scope so it cannot skip the tenant
// filter.
func (s *Store) Database() *mongo.Database { return s.db }

// Collection returns a handle that cannot be queried until it is scoped.
func (s *Store) Collection(name string) *Collection {
	return &Collection{coll: s.db.Collection(name)}
}

// Collection is an unscoped handle. It exposes no query methods by design; call
// Scope (or ScopeTo) to get one that carries tenant isolation.
type Collection struct {
	coll        *mongo.Collection
	tenantField string
}

// WithTenantField scopes this collection on a field other than tenant_id.
//
// Most collections are owned by a tenant and carry tenant_id, but a few name the
// owner differently — tenant-service keys its own records on code and
// tenant_code, mirroring the columns its RLS policies use. Naming the field here
// keeps those collections inside the same scoping rules rather than outside them.
func (c *Collection) WithTenantField(field string) *Collection {
	return &Collection{coll: c.coll, tenantField: field}
}

func (c *Collection) field() string {
	if c.tenantField != "" {
		return c.tenantField
	}
	return TenantField
}

// Name reports the underlying collection name, for logging and index setup.
func (c *Collection) Name() string { return c.coll.Name() }

// Scope binds the collection to the tenant in ctx. A caller with no tenant is
// refused unless it is a platform super admin, which mirrors exactly what the
// app.tenant_id / app.is_platform_admin session variables do under RLS.
func (c *Collection) Scope(ctx context.Context) (*Scope, error) {
	tenantID := tenancy.TenantID(ctx)
	actor, hasActor := auth.ActorFromContext(ctx)
	platformAdmin := hasActor && actor.PlatformAdmin

	if tenantID == "" && !platformAdmin {
		return nil, ErrTenantRequired
	}
	return &Scope{coll: c.coll, field: c.field(), tenantID: tenantID, platformAdmin: platformAdmin}, nil
}

// ScopeTo binds the collection to an explicit tenant. It is for background work
// that carries its tenant in an event rather than a request context; a caller
// that has a request context should use Scope so authority comes from the actor.
func (c *Collection) ScopeTo(tenantID string) (*Scope, error) {
	if tenantID == "" {
		return nil, ErrTenantRequired
	}
	return &Scope{coll: c.coll, field: c.field(), tenantID: tenantID}, nil
}

// Scope is a tenant-bound view of a collection. Every operation merges the
// tenant filter, and inserts stamp the tenant, so no call site can omit it.
type Scope struct {
	coll          *mongo.Collection
	field         string
	tenantID      string
	platformAdmin bool
}

func (s *Scope) tenantKey() string {
	if s.field != "" {
		return s.field
	}
	return TenantField
}

// TenantID reports the bound tenant; empty means a cross-tenant platform view.
func (s *Scope) TenantID() string { return s.tenantID }

// filter merges the caller's filter with the tenant predicate. A platform admin
// with no tenant bound reads across tenants, matching the RLS policy's
// app.is_platform_admin escape hatch; any other caller is always constrained.
func (s *Scope) filter(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	if s.tenantID == "" && s.platformAdmin {
		return merged
	}
	// Set last so a caller cannot widen its own scope by naming the field itself.
	merged[s.tenantKey()] = s.tenantID
	return merged
}

func (s *Scope) FindOne(ctx context.Context, query bson.M, opts ...options.Lister[options.FindOneOptions]) *mongo.SingleResult {
	return s.coll.FindOne(ctx, s.filter(query), opts...)
}

func (s *Scope) Find(ctx context.Context, query bson.M, opts ...options.Lister[options.FindOptions]) (*mongo.Cursor, error) {
	return s.coll.Find(ctx, s.filter(query), opts...)
}

func (s *Scope) CountDocuments(ctx context.Context, query bson.M, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return s.coll.CountDocuments(ctx, s.filter(query), opts...)
}

// InsertOne stamps the document with the bound tenant. A platform admin writing
// without a bound tenant must name the tenant in the document, because a
// tenant-owned record with no owner is not a thing this system stores.
func (s *Scope) InsertOne(ctx context.Context, doc bson.M, opts ...options.Lister[options.InsertOneOptions]) (*mongo.InsertOneResult, error) {
	stamped, err := s.stamp(doc)
	if err != nil {
		return nil, err
	}
	return s.coll.InsertOne(ctx, stamped, opts...)
}

func (s *Scope) UpdateOne(ctx context.Context, query bson.M, update bson.M, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	if err := guardUpdate(update, s.tenantKey()); err != nil {
		return nil, err
	}
	return s.coll.UpdateOne(ctx, s.filter(query), update, opts...)
}

func (s *Scope) UpdateMany(ctx context.Context, query bson.M, update bson.M, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	if err := guardUpdate(update, s.tenantKey()); err != nil {
		return nil, err
	}
	return s.coll.UpdateMany(ctx, s.filter(query), update, opts...)
}

func (s *Scope) DeleteOne(ctx context.Context, query bson.M, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	return s.coll.DeleteOne(ctx, s.filter(query), opts...)
}

func (s *Scope) DeleteMany(ctx context.Context, query bson.M, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	return s.coll.DeleteMany(ctx, s.filter(query), opts...)
}

// Aggregate prepends a tenant $match so a pipeline cannot start unscoped.
func (s *Scope) Aggregate(ctx context.Context, pipeline []bson.M, opts ...options.Lister[options.AggregateOptions]) (*mongo.Cursor, error) {
	scoped := make([]bson.M, 0, len(pipeline)+1)
	scoped = append(scoped, bson.M{"$match": s.filter(bson.M{})})
	scoped = append(scoped, pipeline...)
	return s.coll.Aggregate(ctx, scoped, opts...)
}

func (s *Scope) stamp(doc bson.M) (bson.M, error) {
	stamped := bson.M{}
	for k, v := range doc {
		stamped[k] = v
	}
	if s.tenantID != "" {
		stamped[s.tenantKey()] = s.tenantID
		return stamped, nil
	}
	// Platform admin with no bound tenant: the document must name its owner.
	owner, ok := stamped[s.tenantKey()].(string)
	if !ok || owner == "" {
		return nil, ErrTenantRequired
	}
	return stamped, nil
}

// guardUpdate refuses an update that would move a document to another tenant.
// Re-homing a record is not an operation this system has, and allowing it here
// would turn a single careless $set into a cross-tenant write.
func guardUpdate(update bson.M, tenantKey string) error {
	for _, op := range []string{"$set", "$setOnInsert", "$unset", "$rename"} {
		fields, ok := update[op].(bson.M)
		if !ok {
			continue
		}
		if _, touching := fields[tenantKey]; touching {
			return fmt.Errorf("mongo: update may not modify %s", tenantKey)
		}
	}
	if _, replacing := update[tenantKey]; replacing {
		return fmt.Errorf("mongo: update may not modify %s", tenantKey)
	}
	return nil
}
