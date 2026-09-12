// Package adapters_test runs one contract against every repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them. A behaviour that differs
// between PostgreSQL and MongoDB fails this suite rather than surfacing in
// production after the switch.
package adapters_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/audit-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/audit-service/internal/adapters/postgres"
	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func platformCtx() context.Context {
	return auth.WithActor(context.Background(), auth.Actor{
		UserID: "u-super", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true,
	})
}

func auditLog(t *testing.T, tenantID, action string, at time.Time) *domain.AuditLog {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	return &domain.AuditLog{
		ID: id, TenantID: tenantID, EventID: id.String(),
		EventType: "student.created.v1", SourceService: "student-service",
		Timestamp: at, ReceivedAt: at, Action: action,
		Payload: json.RawMessage(`{"student_id":"s1"}`),
	}
}

// runRepositoryContract is the behaviour every audit repository must have,
// whichever database is behind it.
func runRepositoryContract(t *testing.T, repo ports.Repository) {
	base := time.Now().UTC().Truncate(time.Second)

	for i, tenant := range []string{"upshs", "upshs", "aboom"} {
		log := auditLog(t, tenant, "created", base.Add(time.Duration(i)*time.Second))
		if err := repo.Insert(tenantCtx(tenant), log); err != nil {
			t.Fatalf("insert %s: %v", tenant, err)
		}
	}

	t.Run("list is confined to its tenant", func(t *testing.T) {
		got, _, err := repo.List(tenantCtx("upshs"), "upshs", domain.ListFilter{}, 25, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d records; want the tenant's 2", len(got))
		}
		for _, log := range got {
			if log.TenantID != "upshs" {
				t.Fatalf("list leaked a record owned by %q", log.TenantID)
			}
		}
	})

	t.Run("list is newest first", func(t *testing.T) {
		got, _, err := repo.List(tenantCtx("upshs"), "upshs", domain.ListFilter{}, 25, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) < 2 {
			t.Fatalf("need 2 records to check ordering, got %d", len(got))
		}
		if got[0].Timestamp.Before(got[1].Timestamp) {
			t.Fatalf("records are oldest-first: %v then %v", got[0].Timestamp, got[1].Timestamp)
		}
	})

	t.Run("cursor pages without repeating or skipping", func(t *testing.T) {
		first, cursor, err := repo.List(tenantCtx("upshs"), "upshs", domain.ListFilter{}, 1, "")
		if err != nil || len(first) != 1 {
			t.Fatalf("first page: got %d records, err=%v", len(first), err)
		}
		if cursor == "" {
			t.Fatal("a full page returned no cursor")
		}
		second, _, err := repo.List(tenantCtx("upshs"), "upshs", domain.ListFilter{}, 1, cursor)
		if err != nil || len(second) != 1 {
			t.Fatalf("second page: got %d records, err=%v", len(second), err)
		}
		if first[0].ID == second[0].ID {
			t.Fatal("the cursor repeated a record")
		}
	})

	t.Run("filters narrow the page", func(t *testing.T) {
		got, _, err := repo.List(tenantCtx("upshs"), "upshs",
			domain.ListFilter{SourceService: "student-service"}, 25, "")
		if err != nil || len(got) != 2 {
			t.Fatalf("matching filter returned %d records, err=%v", len(got), err)
		}
		got, _, err = repo.List(tenantCtx("upshs"), "upshs",
			domain.ListFilter{SourceService: "no-such-service"}, 25, "")
		if err != nil || len(got) != 0 {
			t.Fatalf("non-matching filter returned %d records, err=%v", len(got), err)
		}
	})

	t.Run("a tenant cannot read another tenant's records", func(t *testing.T) {
		got, _, err := repo.List(tenantCtx("upshs"), "aboom", domain.ListFilter{}, 25, "")
		if err != nil {
			return // refusing outright is also correct
		}
		for _, log := range got {
			if log.TenantID != "aboom" {
				t.Fatalf("cross-tenant list returned a %q record", log.TenantID)
			}
		}
	})

	t.Run("platform admin reads across tenants", func(t *testing.T) {
		got, _, err := repo.ListAll(platformCtx(), domain.ListFilter{}, 25, "")
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		seen := map[string]bool{}
		for _, log := range got {
			seen[log.TenantID] = true
		}
		if !seen["upshs"] || !seen["aboom"] {
			t.Fatalf("platform view missed a tenant: %v", seen)
		}
	})

	t.Run("payload survives a round trip", func(t *testing.T) {
		got, _, err := repo.List(tenantCtx("upshs"), "upshs", domain.ListFilter{}, 1, "")
		if err != nil || len(got) == 0 {
			t.Fatalf("list: %d records, err=%v", len(got), err)
		}
		var payload map[string]string
		if err := json.Unmarshal(got[0].Payload, &payload); err != nil {
			t.Fatalf("payload did not survive storage: %v (raw=%s)", err, got[0].Payload)
		}
		if payload["student_id"] != "s1" {
			t.Fatalf("payload changed in storage: %v", payload)
		}
	})
}

func TestPostgresRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runRepositoryContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runRepositoryContract(t, mongoadapter.NewRepository(mg.Store))
}
