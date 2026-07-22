package integration

import (
	"context"
	"errors"
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
	years     ports.AcademicYearRepository
	terms     ports.TermRepository
	classes   ports.ClassRepository
	subjects  ports.SubjectRepository
	timetable ports.TimetableRepository
	grading   ports.GradingScaleRepository
}

func newRepos(t *testing.T) repos {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return repos{
		years:     postgres.NewRepository(tdb.DB),
		terms:     postgres.NewTermRepository(tdb.DB),
		classes:   postgres.NewClassRepository(tdb.DB),
		subjects:  postgres.NewSubjectRepository(tdb.DB),
		timetable: postgres.NewTimetableRepository(tdb.DB),
		grading:   postgres.NewGradingScaleRepository(tdb.DB),
	}
}

func TestRepository_AcademicLifecycleAndOutboxAreAtomic(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	year, err := domain.NewAcademicYear(tenantA, "Durable Year", "DY", "2026-09-01", "2027-07-31", false)
	if err != nil {
		t.Fatalf("new durable year: %v", err)
	}
	mutation := ports.AcademicMutation{Kind: ports.AcademicMutationYearCreate, Year: year}
	if err := repo.CommitAcademicLifecycle(ctx, tenantA, mutation, "academic.year_created.v1", ports.YearEventData(year, nil)); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ClaimPendingAcademicEvents(context.Background(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	rollback, err := domain.NewAcademicYear(tenantA, "Rollback", "RB", "2027-09-01", "2028-07-31", false)
	if err != nil {
		t.Fatalf("new rollback year: %v", err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE academic_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitAcademicLifecycle(ctx, tenantA, ports.AcademicMutation{Kind: ports.AcademicMutationYearCreate, Year: rollback}, "academic.year_created.v1", ports.YearEventData(rollback, nil)); err == nil {
		t.Fatal("expected outbox failure")
	}
	if _, err := repo.GetByID(ctx, tenantA, rollback.ID); err == nil {
		t.Fatal("year mutation must roll back")
	}
}

func TestGradingScaleRepositoryTenantIsolationAndLifecycle(t *testing.T) {
	r := newRepos(t)
	ctxA := withTenant(context.Background(), tenantA)
	scale, err := domain.NewGradingScale(tenantA, "Standard", []domain.GradeRange{
		{Min: 0, Max: 59.99, Grade: "C"},
		{Min: 60, Max: 79.99, Grade: "B"},
		{Min: 80, Max: 100, Grade: "A"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.grading.Create(ctxA, tenantA, scale); err != nil {
		t.Fatalf("create grading scale: %v", err)
	}
	if _, err := r.grading.GetByID(withTenant(context.Background(), tenantB), tenantB, scale.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant get: %v", err)
	}
	items, _, err := r.grading.List(ctxA, tenantA, 25, "")
	if err != nil || len(items) != 1 {
		t.Fatalf("list: len=%d err=%v", len(items), err)
	}
	name := "Revised"
	if _, err := scale.ApplyUpdate(&name, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.grading.Update(ctxA, tenantA, scale); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := r.grading.Delete(ctxA, tenantA, scale.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.grading.GetByID(ctxA, tenantA, scale.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted get: %v", err)
	}
}

func TestTimetableRepositoryScopeAndOverlapProtection(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)
	year := mustCreateYear(ctx, t, r.years, "2026/27", "2026-09-01", "2027-07-31")
	term := mustCreateTerm(ctx, t, r.terms, year.ID, "Term 1", "2026-09-01", "2026-12-15")
	class := mustCreateClass(ctx, t, r.classes, year.ID, "Form 1A")
	subject := mustCreateSubject(ctx, t, r.subjects, "Mathematics", nil)
	entry, err := domain.NewTimetableEntry(tenantA, class.ID, term.ID, subject.ID, nil, 1, "08:00", "09:00", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.timetable.Create(ctx, tenantA, entry); err != nil {
		t.Fatalf("create timetable: %v", err)
	}
	overlap, err := domain.NewTimetableEntry(tenantA, class.ID, term.ID, subject.ID, nil, 1, "08:30", "09:30", nil)
	if err != nil {
		t.Fatalf("new overlapping timetable: %v", err)
	}
	if err := r.timetable.Create(ctx, tenantA, overlap); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected overlap conflict, got %v", err)
	}
	list, err := r.timetable.List(ctx, tenantA, ports.TimetableFilter{ClassIDs: []string{class.ID}, Status: "active", Limit: 10})
	if err != nil || len(list) != 1 {
		t.Fatalf("class-scoped timetable: len=%d err=%v", len(list), err)
	}
	empty, err := r.timetable.List(ctx, tenantA, ports.TimetableFilter{ClassIDs: []string{}, Limit: 10})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty scope must fail closed: len=%d err=%v", len(empty), err)
	}
	otherTenant, err := r.timetable.List(withTenant(context.Background(), tenantB), tenantB, ports.TimetableFilter{Limit: 10})
	if err != nil || len(otherTenant) != 0 {
		t.Fatalf("tenant isolation failed: len=%d err=%v", len(otherTenant), err)
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

func TestClassRepository_ListIDsByTeacher(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	r := newRepos(t)
	y := mustCreateYear(ctx, t, r.years, "2025/26", "2025-09-01", "2026-07-31")
	teacher := "33333333-3333-4333-8333-333333333333"
	other := "44444444-4444-4444-8444-444444444444"
	assigned, err := domain.NewClass(tenantA, y.ID, "Class 1A", &teacher, nil)
	if err != nil {
		t.Fatal(err)
	}
	unassigned, err := domain.NewClass(tenantA, y.ID, "Class 1B", &other, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.classes.Create(ctx, tenantA, assigned); err != nil {
		t.Fatal(err)
	}
	if err := r.classes.Create(ctx, tenantA, unassigned); err != nil {
		t.Fatal(err)
	}
	ids, err := r.classes.ListIDsByTeacher(ctx, tenantA, teacher)
	if err != nil || len(ids) != 1 || ids[0] != assigned.ID {
		t.Fatalf("ids=%v err=%v", ids, err)
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
