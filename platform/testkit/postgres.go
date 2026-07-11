package testkit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresTestDB struct {
	Container testcontainers.Container
	DB        *db.DB
	DSN       string
}

// NewPostgres starts a Postgres testcontainer, runs migrations and returns a
// wrapped *db.DB.
func NewPostgres(ctx context.Context, tb testing.TB, migrationsDir string) *PostgresTestDB {
	tb.Helper()

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
		tb.Fatalf("start postgres container: %v", err)
	}

	tb.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			tb.Logf("terminate container: %v", err)
		}
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		tb.Fatalf("container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		tb.Fatalf("container port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())
	database, err := db.Open(ctx, db.Config{
		DSN:        dsn,
		Migrations: migrationsDir,
	})
	if err != nil {
		tb.Fatalf("open db: %v", err)
	}
	tb.Cleanup(database.Close)

	return &PostgresTestDB{Container: ctr, DB: database, DSN: dsn}
}

// SeedTenants creates the canonical tenant rows required by integration tests.
func (p *PostgresTestDB) SeedTenants(ctx context.Context, tb testing.TB) {
	tb.Helper()

	const schema = `
CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
);
`
	if _, err := p.DB.Pool().Exec(ctx, schema); err != nil {
		tb.Fatalf("create tenants table: %v", err)
	}

	for _, tenant := range CanonicalTenants() {
		_, err := p.DB.Pool().Exec(ctx, `
            INSERT INTO tenants (id, code, name, status)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status
        `, tenant.ID, tenant.Code, tenant.Name, tenant.Status)
		if err != nil {
			tb.Fatalf("seed tenant %s: %v", tenant.Code, err)
		}
	}
}

type Tenant struct {
	ID     string
	Code   string
	Name   string
	Status string
}

// CanonicalTenants returns the two standard test tenants used across the
// platform testkit.
func CanonicalTenants() []Tenant {
	return []Tenant{
		{ID: "tenant-upshs", Code: "upshs", Name: "University Practice Senior High School", Status: "active"},
		{ID: "tenant-aboom", Code: "aboom-ame-zion-c", Name: "Aboom AME Zion C Basic", Status: "active"},
	}
}

func (p *PostgresTestDB) ExecAs(ctx context.Context, tenantID, sql string, args ...any) error {
	tx, err := p.DB.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *PostgresTestDB) QueryAs(ctx context.Context, tenantID, sql string, args []any, fn func(pgx.Row) error) error {
	tx, err := p.DB.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

type JWTSigner struct {
	Key []byte
}

func NewJWTSigner() *JWTSigner {
	return &JWTSigner{Key: []byte("test-jwt-signing-key-for-auraedu-platform-tests-only")}
}

func (s *JWTSigner) Mint(tenantID, userID, role string, perms []string) (string, error) {
	return auth.Sign(auth.Claims{
		Subject:     userID,
		TenantID:    tenantID,
		Role:        role,
		Permissions: perms,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}, s.Key)
}

func (s *JWTSigner) Verify(token string) (auth.Claims, error) {
	return auth.Verify(token, s.Key, time.Now())
}
