package testkit

import (
	"context"
	"strings"
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
// This standalone fixture exercises single-document atomicity. Use
// NewMongoReplicaSet for multi-document transactions (including Atlas Free).
func NewMongo(ctx context.Context, tb testing.TB) *MongoTestDB {
	tb.Helper()
	return newMongo(ctx, tb, false)
}

func NewMongoReplicaSet(ctx context.Context, tb testing.TB) *MongoTestDB {
	tb.Helper()
	return newMongo(ctx, tb, true)
}

func newMongo(ctx context.Context, tb testing.TB, replica bool) *MongoTestDB {
	tb.Helper()

	var opts []testcontainers.ContainerCustomizer
	if replica {
		opts = append(opts, tcmongo.WithReplicaSet("rs0"))
	}
	ctr, err := tcmongo.Run(ctx, "mongo:8.0", opts...)
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

	if replica {
		separator := "?"
		if strings.Contains(uri, "?") {
			separator = "&"
		}
		uri += separator + "directConnection=true"
	}
	store, err := pmongo.Open(ctx, pmongo.Config{URI: uri, Database: "auraedu_test"})
	if err != nil {
		tb.Fatalf("open mongo store: %v", err)
	}
	tb.Cleanup(func() { _ = store.Close(context.Background()) })

	return &MongoTestDB{Container: ctr, Store: store, URI: uri}
}
