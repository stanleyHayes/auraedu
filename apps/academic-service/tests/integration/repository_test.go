package integration

import (
	"context"
	"testing"

	"github.com/auraedu/academic-service/internal/adapters/postgres"
	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

func newRepo(t *testing.T) (ports.Repository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustCreateYear(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, name, start, end string) *domain.AcademicYear {
	t.Helper()
	y, err := domain.NewAcademicYear(tenantID, name, "", start, end, false)
	if err != nil {
		t.Fatalf("new academic year: %v", err)
	}
	if err := repo.Create(ctx, tenantID, y); err != nil {
		t.Fatalf("create academic year: %v", err)
	}
	return y
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	y := mustCreateYear(t, ctx, repo, tenantA, "2025/26", "2025-09-01", "2026-07-31")

	got, err := repo.GetByID(ctx, tenantA, y.ID)
	if err != nil {
		t.Fatalf("get academic year: %v", err)
	}
	if got.ID != y.ID || got.Name != "2025/26" {
		t.Fatalf("academic year mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateYear(t, ctx, repo, tenantA, "2024/25", "2024-09-01", "2025-07-31")
	y2 := mustCreateYear(t, ctx, repo, tenantA, "2025/26", "2025-09-01", "2026-07-31")

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
	if len(page2) != 1 || page2[0].ID != y2.ID {
		t.Fatalf("expected second academic year, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	y := mustCreateYear(t, ctx, repo, tenantA, "2025/26", "2025-09-01", "2026-07-31")
	newName := "2025/2026 Academic Year"
	if _, err := y.ApplyUpdate(&newName, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, y); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, y.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != newName {
		t.Fatalf("name not updated: %q", got.Name)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	y := mustCreateYear(t, ctx, repo, tenantA, "2025/26", "2025-09-01", "2026-07-31")
	if err := repo.Delete(ctx, tenantA, y.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, y.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	y := mustCreateYear(t, aCtx, repo, tenantA, "Tenant A Year", "2025-09-01", "2026-07-31")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, y.ID); err == nil {
		t.Fatal("tenant B should not see tenant A academic year")
	}

	list, _, err := repo.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 academic years, got %d", len(list))
	}
}
