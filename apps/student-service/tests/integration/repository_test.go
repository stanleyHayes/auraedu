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

func newRepo(t *testing.T) ports.Repository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustCreate(ctx context.Context, t *testing.T, repo ports.Repository, first, last string) *domain.Student {
	t.Helper()
	s, err := domain.NewStudent(tenantA, first, last)
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s); err != nil {
		t.Fatalf("create student: %v", err)
	}
	return s
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Kwame", "Nkrumah")

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
	repo := newRepo(t)

	mustCreate(ctx, t, repo, "A", "One")
	s2 := mustCreate(ctx, t, repo, "B", "Two")

	page, next, err := repo.List(ctx, tenantA, nil, 1, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, nil, 1, next)
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != s2.ID {
		t.Fatalf("expected second student, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Yaa", "Asantewaa")
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
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Kofi", "Annan")
	if err := repo.Delete(ctx, tenantA, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, s.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	s := mustCreate(aCtx, t, repo, "Tenant", "A")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A student")
	}

	list, _, err := repo.List(bCtx, tenantB, nil, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 students, got %d", len(list))
	}
}

// mustCreateWithClass persists a student with an optional class assignment (AURA-10.11).
func mustCreateWithClass(ctx context.Context, t *testing.T, repo ports.Repository, tenantID, first, last string, classID, yearID *string) *domain.Student {
	t.Helper()
	s, err := domain.NewStudent(tenantID, first, last)
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	s.ClassID = classID
	s.AcademicYearID = yearID
	if err := repo.Create(ctx, tenantID, s); err != nil {
		t.Fatalf("create student: %v", err)
	}
	return s
}

func TestRepository_ClassFilter(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	classX := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	classY := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	yearY := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	s1 := mustCreateWithClass(ctx, t, repo, tenantA, "Ama", "One", &classX, &yearY)
	mustCreateWithClass(ctx, t, repo, tenantA, "Kojo", "Two", &classY, nil)
	s3 := mustCreateWithClass(ctx, t, repo, tenantA, "Esi", "Three", nil, nil)

	// Filter unset: every student of the tenant.
	all, _, err := repo.List(ctx, tenantA, nil, 10, "")
	if err != nil {
		t.Fatalf("list unfiltered: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 students without filter, got %d", len(all))
	}

	// Filter set: only the classX roster, with the assignment round-tripped.
	roster, _, err := repo.List(ctx, tenantA, &classX, 10, "")
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(roster) != 1 || roster[0].ID != s1.ID {
		t.Fatalf("expected only student %s, got %+v", s1.ID, roster)
	}
	if roster[0].ClassID == nil || *roster[0].ClassID != classX {
		t.Fatalf("class_id not round-tripped: %+v", roster[0])
	}
	if roster[0].AcademicYearID == nil || *roster[0].AcademicYearID != yearY {
		t.Fatalf("academic_year_id not round-tripped: %+v", roster[0])
	}

	// NULL columns scan back as nil.
	got3, err := repo.GetByID(ctx, tenantA, s3.ID)
	if err != nil {
		t.Fatalf("get s3: %v", err)
	}
	if got3.ClassID != nil || got3.AcademicYearID != nil {
		t.Fatalf("expected nil class fields for unassigned student, got %+v", got3)
	}

	// The filter composes with cursor pagination.
	s4 := mustCreateWithClass(ctx, t, repo, tenantA, "Abena", "Four", &classX, nil)
	page1, next, err := repo.List(ctx, tenantA, &classX, 1, "")
	if err != nil {
		t.Fatalf("list filtered page 1: %v", err)
	}
	if len(page1) != 1 || page1[0].ID != s1.ID || next == "" {
		t.Fatalf("unexpected first filtered page: %+v next=%q", page1, next)
	}
	page2, _, err := repo.List(ctx, tenantA, &classX, 1, next)
	if err != nil {
		t.Fatalf("list filtered page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != s4.ID {
		t.Fatalf("expected second classX student %s, got %+v", s4.ID, page2)
	}

	// Cross-tenant: the same class_id in another tenant is an independent roster (RLS).
	bCtx := withTenant(context.Background(), tenantB)
	sB := mustCreateWithClass(bCtx, t, repo, tenantB, "Tenant", "B", &classX, nil)
	bRoster, _, err := repo.List(bCtx, tenantB, &classX, 10, "")
	if err != nil {
		t.Fatalf("list tenant B filtered: %v", err)
	}
	if len(bRoster) != 1 || bRoster[0].ID != sB.ID {
		t.Fatalf("tenant B should see only its own roster, got %+v", bRoster)
	}
	bAll, _, err := repo.List(bCtx, tenantB, nil, 10, "")
	if err != nil {
		t.Fatalf("list tenant B unfiltered: %v", err)
	}
	if len(bAll) != 1 || bAll[0].ID != sB.ID {
		t.Fatalf("tenant B should not see tenant A students, got %+v", bAll)
	}
}
