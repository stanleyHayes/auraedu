package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgresContainer(ctx context.Context, t *testing.T) string {
	t.Helper()

	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(ctr)
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	return fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())
}
