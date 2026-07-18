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

// repos bundles the four Postgres repository implementations under test.
type repos struct {
	years    ports.AcademicYearRepository
	terms    ports.TermRepository
	classes  ports.ClassRepository
	subjects ports.SubjectRepository
}

func newRepos(t *testing.T) repos {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return repos{
		years:    postgres.NewRepository(tdb.DB),
		terms:    postgres.NewTermRepository(tdb.DB),
		classes:  postgres.NewClassRepository(tdb.DB),
		subjects: postgres.NewSubjectRepository(tdb.DB),
	}
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustCreateYear(ctx context.Context, t *testing.T, repo ports.AcademicYearRepository, name, start, end string) *domain.AcademicYear {
	t.Helper()
	y, err := domain.NewAcademicYear(tenantA, name, "", start, end, false)
	if err != nil {
		t.Fatalf("new academic year: %v", err)
	}
	if err := repo.Create(ctx, tenantA, y); err != nil {
		t.Fatalf("create academic year: %v", err)
	}
	return y
}

func mustCreateTerm(ctx context.Context, t *testing.T, repo ports.TermRepository, yearID, name, start, end string) *domain.Term {
	t.Helper()
	term, err := domain.NewTerm(tenantA, yearID, name, start, end)
	if err != nil {
		t.Fatalf("new term: %v", err)
	}
	if err := repo.Create(ctx, tenantA, term); err != nil {
		t.Fatalf("create term: %v", err)
	}
	return term
}

func mustCreateClass(ctx context.Context, t *testing.T, repo ports.ClassRepository, yearID, name string) *domain.Class {
	t.Helper()
	c, err := domain.NewClass(tenantA, yearID, name, nil, nil)
	if err != nil {
		t.Fatalf("new class: %v", err)
	}
	if err := repo.Create(ctx, tenantA, c); err != nil {
		t.Fatalf("create class: %v", err)
	}
	return c
}

func mustCreateSubject(ctx context.Context, t *testing.T, repo ports.SubjectRepository, name string, code *string) *domain.Subject {
	t.Helper()
	s, err := domain.NewSubject(tenantA, name, code, nil)
	if err != nil {
		t.Fatalf("new subject: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s); err != nil {
		t.Fatalf("create subject: %v", err)
	}
	return s
}

// ---- academic years ---------------------------------------------------------

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")

	got, err := r.years.GetByID(ctx, tenantA, y.ID)
	if err != nil {
		t.Fatalf("get academic year: %v", err)
	}
	if got.ID != y.ID || got.Name != "2025/26" {
		t.Fatalf("academic year mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	mustCreateYear(ctx, t, r.years, "2024/25", "2024-09-01", "2025-07-31")
	y2 := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")

	page, next, err := r.years.List(ctx, tenantA, 1, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := r.years.List(ctx, tenantA, 1, next)
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != y2.ID {
		t.Fatalf("expected second academic year, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	newName := "2025/2026 Academic Year"
	if _, err := y.ApplyUpdate(&newName, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := r.years.Update(ctx, tenantA, y); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := r.years.GetByID(ctx, tenantA, y.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != newName {
		t.Fatalf("name not updated: %q", got.Name)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	if err := r.years.Delete(ctx, tenantA, y.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.years.GetByID(ctx, tenantA, y.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	r := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	y := mustCreateYear(aCtx, t, r.years, "Tenant A Year", "2025-09-01", "2026-07-31")

	bCtx := withTenant(ctx, tenantB)
	if _, err := r.years.GetByID(bCtx, tenantB, y.ID); err == nil {
		t.Fatal("tenant B should not see tenant A academic year")
	}

	list, _, err := r.years.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 academic years, got %d", len(list))
	}
}

// ---- terms ------------------------------------------------------------------

func TestTermRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	term := mustCreateTerm(ctx, t, r.terms, y.ID, "Term 1", "2025-09-01", "2025-12-31")

	got, err := r.terms.GetByID(ctx, tenantA, term.ID)
	if err != nil {
		t.Fatalf("get term: %v", err)
	}
	if got.ID != term.ID || got.Name != "Term 1" || got.AcademicYearID != y.ID {
		t.Fatalf("term mismatch: %+v", got)
	}
	if got.StartDate.String() != "2025-09-01" || got.EndDate.String() != "2025-12-31" {
		t.Fatalf("term dates mismatch: %+v", got)
	}
}

func TestTermRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	mustCreateTerm(ctx, t, r.terms, y.ID, "Term 1", "2025-09-01", "2025-12-31")
	t2 := mustCreateTerm(ctx, t, r.terms, y.ID, "Term 2", "2026-01-05", "2026-04-30")

	page, next, err := r.terms.List(ctx, tenantA, 1, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := r.terms.List(ctx, tenantA, 1, next)
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != t2.ID {
		t.Fatalf("expected second term, got %+v", page2)
	}
}

func TestTermRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	term := mustCreateTerm(ctx, t, r.terms, y.ID, "Term 1", "2025-09-01", "2025-12-31")

	newName := "First Term"
	if _, err := term.ApplyUpdate(&newName, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := r.terms.Update(ctx, tenantA, term); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := r.terms.GetByID(ctx, tenantA, term.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != newName {
		t.Fatalf("name not updated: %q", got.Name)
	}
}

func TestTermRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	term := mustCreateTerm(ctx, t, r.terms, y.ID, "Term 1", "2025-09-01", "2025-12-31")

	if err := r.terms.Delete(ctx, tenantA, term.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.terms.GetByID(ctx, tenantA, term.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTermRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	r := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	y := mustCreateYear(aCtx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	term := mustCreateTerm(aCtx, t, r.terms, y.ID, "Tenant A Term", "2025-09-01", "2025-12-31")

	bCtx := withTenant(ctx, tenantB)
	if _, err := r.terms.GetByID(bCtx, tenantB, term.ID); err == nil {
		t.Fatal("tenant B should not see tenant A term")
	}

	list, _, err := r.terms.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 terms, got %d", len(list))
	}
}

// ---- classes ----------------------------------------------------------------

func TestClassRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	capacity := 45
	teacher := "33333333-3333-3333-3333-333333333333"
	c, err := domain.NewClass(tenantA, y.ID, "Class 1", &teacher, &capacity)
	if err != nil {
		t.Fatalf("new class: %v", err)
	}
	if err := r.classes.Create(ctx, tenantA, c); err != nil {
		t.Fatalf("create class: %v", err)
	}

	got, err := r.classes.GetByID(ctx, tenantA, c.ID)
	if err != nil {
		t.Fatalf("get class: %v", err)
	}
	if got.ID != c.ID || got.Name != "Class 1" || got.AcademicYearID != y.ID {
		t.Fatalf("class mismatch: %+v", got)
	}
	if got.ClassTeacherID == nil || *got.ClassTeacherID != teacher {
		t.Fatalf("class_teacher_id mismatch: %+v", got)
	}
	if got.Capacity == nil || *got.Capacity != capacity {
		t.Fatalf("capacity mismatch: %+v", got)
	}
}

func TestClassRepository_CreateAndGet_NullableFieldsEmpty(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	c := mustCreateClass(ctx, t, r.classes, y.ID, "Class 1")

	got, err := r.classes.GetByID(ctx, tenantA, c.ID)
	if err != nil {
		t.Fatalf("get class: %v", err)
	}
	if got.ClassTeacherID != nil || got.Capacity != nil {
		t.Fatalf("expected null teacher/capacity round-trip, got %+v", got)
	}
}

func TestClassRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	c := mustCreateClass(ctx, t, r.classes, y.ID, "Class 1")

	newName := "Class 1A"
	teacher := "33333333-3333-3333-3333-333333333333"
	capacity := 40
	if _, err := c.ApplyUpdate(&newName, &teacher, &capacity); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := r.classes.Update(ctx, tenantA, c); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := r.classes.GetByID(ctx, tenantA, c.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != newName || got.ClassTeacherID == nil || *got.ClassTeacherID != teacher || got.Capacity == nil || *got.Capacity != capacity {
		t.Fatalf("class not updated: %+v", got)
	}
}

func TestClassRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	c := mustCreateClass(ctx, t, r.classes, y.ID, "Class 1")

	if err := r.classes.Delete(ctx, tenantA, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.classes.GetByID(ctx, tenantA, c.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestClassRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	r := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	y := mustCreateYear(aCtx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	c := mustCreateClass(aCtx, t, r.classes, y.ID, "Tenant A Class")

	bCtx := withTenant(ctx, tenantB)
	if _, err := r.classes.GetByID(bCtx, tenantB, c.ID); err == nil {
		t.Fatal("tenant B should not see tenant A class")
	}

	list, _, err := r.classes.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 classes, got %d", len(list))
	}
}

// ---- subjects ---------------------------------------------------------------

func TestSubjectRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	code := "MATH"
	s := mustCreateSubject(ctx, t, r.subjects, "Mathematics", &code)

	got, err := r.subjects.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get subject: %v", err)
	}
	if got.ID != s.ID || got.Name != "Mathematics" {
		t.Fatalf("subject mismatch: %+v", got)
	}
	if got.Code == nil || *got.Code != code {
		t.Fatalf("code mismatch: %+v", got)
	}
	if got.Description != nil {
		t.Fatalf("expected null description round-trip, got %+v", got)
	}
}

func TestSubjectRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	s := mustCreateSubject(ctx, t, r.subjects, "Mathematics", nil)

	newName := "Core Mathematics"
	code := "CMATH"
	desc := "Compulsory mathematics"
	if _, err := s.ApplyUpdate(&newName, &code, &desc); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := r.subjects.Update(ctx, tenantA, s); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := r.subjects.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != newName || got.Code == nil || *got.Code != code || got.Description == nil || *got.Description != desc {
		t.Fatalf("subject not updated: %+v", got)
	}
}

func TestSubjectRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)

	s := mustCreateSubject(ctx, t, r.subjects, "Mathematics", nil)
	if err := r.subjects.Delete(ctx, tenantA, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.subjects.GetByID(ctx, tenantA, s.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSubjectRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	r := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	s := mustCreateSubject(aCtx, t, r.subjects, "Tenant A Subject", nil)

	bCtx := withTenant(ctx, tenantB)
	if _, err := r.subjects.GetByID(bCtx, tenantB, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A subject")
	}

	list, _, err := r.subjects.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 subjects, got %d", len(list))
	}
}
