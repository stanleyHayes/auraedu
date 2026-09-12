package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func startMongo(ctx context.Context, t *testing.T) string {
	t.Helper()
	ctr, err := tcmongo.Run(ctx, "mongo:8.0")
	if err != nil {
		t.Fatalf("start mongodb container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminate mongodb container: %v", err)
		}
	})
	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("mongodb connection string: %v", err)
	}
	return uri
}

func openTestStore(ctx context.Context, t *testing.T) *Store {
	t.Helper()
	store, err := Open(ctx, Config{URI: startMongo(ctx, t), Database: "auraedu_test"})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func platformCtx() context.Context {
	return auth.WithActor(context.Background(), auth.Actor{
		UserID: "u-super", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true,
	})
}

// MongoDB has no row-level security, so these are the tests that carry the
// guarantee RLS carries on PostgreSQL.

func TestScopeRefusesUnscopedAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("records")

	// No tenant and no platform authority must fail closed, not read everything.
	if _, err := coll.Scope(context.Background()); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("unscoped access was permitted: %v", err)
	}
	if _, err := coll.ScopeTo(""); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("empty explicit tenant was permitted: %v", err)
	}
}

func TestScopeConfinesReadsWritesAndDeletesToItsTenant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("records")

	upshs, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope upshs: %v", err)
	}
	aboom, err := coll.Scope(tenantCtx("aboom"))
	if err != nil {
		t.Fatalf("scope aboom: %v", err)
	}

	if _, err := upshs.InsertOne(ctx, bson.M{"_id": "r1", "name": "upshs record"}); err != nil {
		t.Fatalf("insert upshs: %v", err)
	}
	if _, err := aboom.InsertOne(ctx, bson.M{"_id": "r2", "name": "aboom record"}); err != nil {
		t.Fatalf("insert aboom: %v", err)
	}

	// A tenant sees only its own rows, even asking for everything.
	count, err := upshs.CountDocuments(ctx, bson.M{})
	if err != nil || count != 1 {
		t.Fatalf("upshs saw %d documents (err=%v); want exactly its own", count, err)
	}

	// Naming another tenant's document by id must not reach it.
	if err := upshs.FindOne(ctx, bson.M{"_id": "r2"}).Err(); err == nil {
		t.Fatal("a tenant read another tenant's document by id")
	}

	// Nor may it be widened by passing tenant_id in the caller's own filter.
	if err := upshs.FindOne(ctx, bson.M{"_id": "r2", TenantField: "aboom"}).Err(); err == nil {
		t.Fatal("a caller widened its scope by supplying tenant_id")
	}

	// Writes and deletes are bounded the same way.
	res, err := upshs.UpdateOne(ctx, bson.M{"_id": "r2"}, bson.M{"$set": bson.M{"name": "hijacked"}})
	if err != nil || res.MatchedCount != 0 {
		t.Fatalf("cross-tenant update matched %d documents (err=%v)", res.MatchedCount, err)
	}
	del, err := upshs.DeleteOne(ctx, bson.M{"_id": "r2"})
	if err != nil || del.DeletedCount != 0 {
		t.Fatalf("cross-tenant delete removed %d documents (err=%v)", del.DeletedCount, err)
	}

	// The victim's record is untouched.
	var got bson.M
	if err := aboom.FindOne(ctx, bson.M{"_id": "r2"}).Decode(&got); err != nil {
		t.Fatalf("aboom lost its own record: %v", err)
	}
	if got["name"] != "aboom record" {
		t.Fatalf("aboom record was modified across tenants: %v", got["name"])
	}
}

func TestInsertStampsTenantAndUpdatesMayNotRehomeADocument(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("records")

	upshs, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	// A document claiming another owner is stamped with the scope's tenant.
	if _, err := upshs.InsertOne(ctx, bson.M{"_id": "r1", TenantField: "aboom"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var got bson.M
	if err := upshs.FindOne(ctx, bson.M{"_id": "r1"}).Decode(&got); err != nil {
		t.Fatalf("insert did not land in the scope's tenant: %v", err)
	}

	// Re-homing a record is not an operation this system has.
	if _, err := upshs.UpdateOne(ctx, bson.M{"_id": "r1"}, bson.M{"$set": bson.M{TenantField: "aboom"}}); err == nil {
		t.Fatal("an update was allowed to move a document to another tenant")
	}
}

func TestPlatformAdminReadsAcrossTenantsButMustNameAnOwnerToWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("records")

	for _, tenant := range []string{"upshs", "aboom"} {
		scope, err := coll.Scope(tenantCtx(tenant))
		if err != nil {
			t.Fatalf("scope %s: %v", tenant, err)
		}
		if _, err := scope.InsertOne(ctx, bson.M{"_id": tenant}); err != nil {
			t.Fatalf("insert %s: %v", tenant, err)
		}
	}

	platform, err := coll.Scope(platformCtx())
	if err != nil {
		t.Fatalf("platform scope: %v", err)
	}
	count, err := platform.CountDocuments(ctx, bson.M{})
	if err != nil || count != 2 {
		t.Fatalf("platform admin saw %d documents (err=%v); want both tenants", count, err)
	}
	// A tenant-owned record with no owner is not a thing this system stores.
	if _, err := platform.InsertOne(ctx, bson.M{"_id": "orphan"}); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("platform admin wrote an ownerless record: %v", err)
	}
}

func event(t *testing.T, tenantID, id string) tenancy.CloudEvent {
	t.Helper()
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: "student.created.v1", Source: "student-service",
		ID: id, Time: time.Now().UTC().Format(time.RFC3339), TenantID: tenantID,
		Data: json.RawMessage(`{"student_id":"s1"}`),
	}
}

type recordingPublisher struct {
	published []tenancy.CloudEvent
	failFirst bool
}

func (p *recordingPublisher) Publish(_ context.Context, e tenancy.CloudEvent) error {
	if p.failFirst {
		p.failFirst = false
		return errors.New("bus unavailable")
	}
	p.published = append(p.published, e)
	return nil
}

func TestEventsCommitWithTheirDocumentAndDrainOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("students")

	scope, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if _, err := scope.InsertWithEvents(ctx, bson.M{"_id": "s1", "name": "Ama"}, event(t, "upshs", "e1")); err != nil {
		t.Fatalf("insert with events: %v", err)
	}

	publisher := &recordingPublisher{}
	relay := NewRelay(coll, publisher, 10)
	published, err := relay.Drain(ctx)
	if err != nil || published != 1 {
		t.Fatalf("drain published %d (err=%v); want 1", published, err)
	}
	if publisher.published[0].Type != "student.created.v1" {
		t.Fatalf("unexpected event published: %+v", publisher.published[0])
	}

	// A second drain must find nothing: the event was cleared after publishing.
	published, err = relay.Drain(ctx)
	if err != nil || published != 0 {
		t.Fatalf("drain republished %d events (err=%v); want 0", published, err)
	}
}

func TestAFailedPublishIsRetriedRatherThanDropped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("students")

	scope, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if _, err := scope.InsertWithEvents(ctx, bson.M{"_id": "s1"}, event(t, "upshs", "e1")); err != nil {
		t.Fatalf("insert with events: %v", err)
	}

	failing := &recordingPublisher{failFirst: true}
	relay := NewRelay(coll, failing, 10)
	if published, err := relay.Drain(ctx); published != 0 || err == nil {
		t.Fatalf("a failed publish was reported as success: published=%d err=%v", published, err)
	}

	// The event is still queued, so the next drain delivers it.
	published, err := relay.Drain(ctx)
	if err != nil || published != 1 {
		t.Fatalf("a failed event was dropped instead of retried: published=%d err=%v", published, err)
	}
}

func TestAnEventMayNotBeQueuedUnderAnotherTenant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("students")

	scope, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	_, err = scope.InsertWithEvents(ctx, bson.M{"_id": "s1"}, event(t, "aboom", "e1"))
	if err == nil {
		t.Fatal("a scope queued an event announcing another tenant's change")
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatal("expected a descriptive error")
	}
}

// A few collections name their owner something other than tenant_id —
// tenant-service keys its own records on code and tenant_code, mirroring its RLS
// columns. Those collections must obey exactly the same scoping rules.
func TestACollectionMayScopeOnADifferentOwnerField(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("tenants").WithTenantField("code")

	upshs, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if _, err := upshs.InsertOne(ctx, bson.M{"_id": "upshs", "name": "UPSHS"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	aboom, err := coll.Scope(tenantCtx("aboom"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if _, err := aboom.InsertOne(ctx, bson.M{"_id": "aboom", "name": "Aboom"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// The owner field is stamped, not tenant_id.
	var got bson.M
	if err := upshs.FindOne(ctx, bson.M{"_id": "upshs"}).Decode(&got); err != nil {
		t.Fatalf("read own record: %v", err)
	}
	if got["code"] != "upshs" {
		t.Fatalf("owner field was not stamped: %v", got)
	}
	if _, unexpected := got[TenantField]; unexpected {
		t.Fatalf("tenant_id was stamped on a collection scoped by code: %v", got)
	}

	// Isolation still holds, and cannot be widened by naming the field.
	if err := upshs.FindOne(ctx, bson.M{"_id": "aboom"}).Err(); err == nil {
		t.Fatal("a tenant read another tenant's record")
	}
	if err := upshs.FindOne(ctx, bson.M{"_id": "aboom", "code": "aboom"}).Err(); err == nil {
		t.Fatal("a caller widened its scope by supplying the owner field")
	}
	if _, err := upshs.UpdateOne(ctx, bson.M{"_id": "upshs"}, bson.M{"$set": bson.M{"code": "aboom"}}); err == nil {
		t.Fatal("an update was allowed to re-home a record scoped by code")
	}
}
