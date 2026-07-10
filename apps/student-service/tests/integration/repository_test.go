package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/student-service/internal/adapters/postgres"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
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

func mustCreate(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, first, last string) *domain.Student {
	t.Helper()
	s, err := domain.NewStudent(tenantID, first, last)
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	if err := repo.Create(ctx, tenantID, s); err != nil {
		t.Fatalf("create student: %v", err)
	}
	return s
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	s := mustCreate(t, ctx, repo, tenantA, "Kwame", "Nkrumah")

	got, err := repo.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get student: %v", err)
	}
	if got.ID != s.ID || got.FirstName != "Kwame" {
		t.Fatalf("student mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreate(t, ctx, repo, tenantA, "A", "One")
	s2 := mustCreate(t, ctx, repo, tenantA, "B", "Two")

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
	if len(page2) != 1 || page2[0].ID != s2.ID {
		t.Fatalf("expected second student, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	s := mustCreate(t, ctx, repo, tenantA, "Yaa", "Asantewaa")
	newName := "Nana"
	if _, err := s.ApplyUpdate(&newName, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, s); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.FirstName != "Nana" {
		t.Fatalf("first name not updated: %q", got.FirstName)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	s := mustCreate(t, ctx, repo, tenantA, "Kofi", "Annan")
	if err := repo.Delete(ctx, tenantA, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, s.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	s := mustCreate(t, aCtx, repo, tenantA, "Tenant", "A")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A student")
	}

	list, _, err := repo.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 students, got %d", len(list))
	}
}
