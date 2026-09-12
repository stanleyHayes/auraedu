package testkit

import (
	"context"
	"testing"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

type MongoTestDB struct {
	Container testcontainers.Container
	Store     *pmongo.Store
	URI       string
}

// NewMongo starts a MongoDB testcontainer and returns a connected Store. It is
// the counterpart to NewPostgres so a service's adapter tests can run the same
// assertions against either driver.
//
// The image is a single node without a replica set, which is what the free
// Atlas tier gives you: no multi-document transactions. Adapters that work here
// work there, and one that quietly depends on a transaction fails here first.
func NewMongo(ctx context.Context, tb testing.TB) *MongoTestDB {
	tb.Helper()

	ctr, err := tcmongo.Run(ctx, "mongo:8.0")
	if err != nil {
		tb.Fatalf("start mongodb container: %v", err)
	}
	tb.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			tb.Logf("terminate mongodb container: %v", err)
		}
	})

	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		tb.Fatalf("mongodb connection string: %v", err)
	}

	store, err := pmongo.Open(ctx, pmongo.Config{URI: uri, Database: "auraedu_test"})
	if err != nil {
		tb.Fatalf("open mongo store: %v", err)
	}
	tb.Cleanup(func() { _ = store.Close(context.Background()) })

	return &MongoTestDB{Container: ctr, Store: store, URI: uri}
}
