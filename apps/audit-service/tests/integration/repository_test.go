package integration

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/postgres"
	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "school-a"
const tenantB = "school-b"

func newRepo(t *testing.T) ports.Repository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustInsert(ctx context.Context, t *testing.T, repo ports.Repository, eventType, subject string) *domain.AuditLog {
	t.Helper()
	log, err := domain.NewAuditLogBuilder().
		TenantID(tenantA).
		EventID(uuid.NewString()).
		EventType(eventType).
		SourceService("test-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action(eventType).
		ResourceType("student").
		ResourceID(subject).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := repo.Insert(ctx, log); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return log
}

func TestRepository_InsertAndList(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	log := mustInsert(ctx, t, repo, "student.created.v1", "stu-1")

	list, _, err := repo.List(ctx, tenantA, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 log, got %d", len(list))
	}
	if list[0].EventID != log.EventID {
		t.Fatalf("event id mismatch: got %s, want %s", list[0].EventID, log.EventID)
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	mustInsert(aCtx, t, repo, "student.created.v1", "stu-a")

	bCtx := withTenant(ctx, tenantB)
	list, _, err := repo.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 logs for tenant B, got %d", len(list))
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	log1 := mustInsert(ctx, t, repo, "student.created.v1", "stu-1")
	time.Sleep(10 * time.Millisecond)
	mustInsert(ctx, t, repo, "student.updated.v1", "stu-2")

	page, next, err := repo.List(ctx, tenantA, 1, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, 1, next)
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 item on second page, got %d", len(page2))
	}
	if page2[0].ID != log1.ID {
		t.Fatalf("expected oldest log on second page, got %s, want %s", page2[0].ID, log1.ID)
	}
}

func TestRepository_ListAllCrossTenant(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	mustInsert(withTenant(ctx, tenantA), t, repo, "student.created.v1", "stu-a")

	logB, err := domain.NewAuditLogBuilder().
		TenantID(tenantB).
		EventID(uuid.NewString()).
		EventType("invoice.created.v1").
		SourceService("test-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		Action("invoice.created.v1").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := repo.Insert(withTenant(ctx, tenantB), logB); err != nil {
		t.Fatalf("insert tenant B: %v", err)
	}

	// A platform super admin reads across tenants (app.is_platform_admin RLS bypass).
	adminCtx := auth.WithActor(ctx, auth.Actor{
		UserID:        "admin-1",
		Role:          auth.RolePlatformSuperAdmin,
		PlatformAdmin: true,
	})
	list, _, err := repo.ListAll(adminCtx, 10, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 logs across tenants, got %d", len(list))
	}
}
