package persistence

import (
	"context"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func TestOpenRefusesStandaloneMongo(t *testing.T) {
	ctx := context.Background()
	c, err := tcmongo.Run(ctx, "mongo:8.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_DRIVER", "mongodb")
	t.Setenv("MONGODB_URI", uri)
	t.Setenv("MONGODB_DATABASE", "topology_test")
	connection, err := Open(ctx)
	if connection != nil {
		connection.Close()
		t.Fatal("standalone accepted")
	}
	if err == nil || !strings.Contains(err.Error(), "replica set") {
		t.Fatalf("expected clear topology refusal, got %v", err)
	}
}
