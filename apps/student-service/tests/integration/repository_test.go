package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/student-service/internal/adapters/postgres"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRepository_EnrollmentHistoryIsAtomicUniqueAndTenantScoped(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	classA := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	classB := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	yearA := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	yearB := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	student := mustCreateWithClass(ctx, t, repo, tenantA, "Ama", "History", &classA, &yearA)

	initial, _, err := repo.ListEnrollments(ctx, tenantA, student.ID, 10, "")
	if err != nil || len(initial) != 1 || initial[0].ClassID != classA {
		t.Fatalf("initial enrollment: %+v err=%v", initial, err)
	}
	next, err := domain.NewEnrollment(tenantA, student.ID, classB, yearB, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateEnrollment(ctx, tenantA, next); err != nil {
		t.Fatalf("create next enrollment: %v", err)
	}
	updated, err := repo.GetByID(ctx, tenantA, student.ID)
	if err != nil || updated.ClassID == nil || *updated.ClassID != classB || updated.AcademicYearID == nil || *updated.AcademicYearID != yearB {
		t.Fatalf("current roster projection: %+v err=%v", updated, err)
	}
	duplicate, err := domain.NewEnrollment(tenantA, student.ID, classA, yearB, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateEnrollment(ctx, tenantA, duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected one enrollment per academic year, got %v", err)
	}
	history, _, err := repo.ListEnrollments(ctx, tenantA, student.ID, 10, "")
	if err != nil || len(history) != 2 {
		t.Fatalf("history: %+v err=%v", history, err)
	}
	foreign, _, err := repo.ListEnrollments(withTenant(context.Background(), tenantB), tenantB, student.ID, 10, "")
	if err != nil || len(foreign) != 0 {
		t.Fatalf("cross-tenant history leaked: %+v err=%v", foreign, err)
	}
}

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

func TestRepository_LifecycleMutationAndOutboxAreAtomic(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	student, err := domain.NewStudent(tenantA, "Durable", "Learner")
	if err != nil {
		t.Fatal(err)
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: student}
	if err := repo.CommitStudentLifecycle(ctx, tenantA, mutation, "student.created.v1", ports.StudentEventData(student, nil)); err != nil {
		t.Fatalf("commit lifecycle: %v", err)
	}
	items, err := repo.ClaimPendingStudentEvents(context.Background(), 10)
	if err != nil || len(items) != 1 || items[0].EventType != "student.created.v1" {
		t.Fatalf("outbox=%+v err=%v", items, err)
	}

	rollbackStudent, err := domain.NewStudent(tenantA, "Rollback", "Learner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE student_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitStudentLifecycle(ctx, tenantA, ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: rollbackStudent}, "student.created.v1", ports.StudentEventData(rollbackStudent, nil)); err == nil {
		t.Fatal("expected outbox failure")
	}
	if _, err := repo.GetByID(ctx, tenantA, rollbackStudent.ID); err == nil {
		t.Fatal("student mutation must roll back when outbox insert fails")
	}
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
	userID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err := s.ApplyUpdate(&newName, nil, nil, &userID); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, s); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.FirstName != "Nana" || got.UserID == nil || *got.UserID != userID {
		t.Fatalf("student lifecycle fields not updated: %+v", got)
	}

	clearUser := ""
	if _, err := got.ApplyUpdate(nil, nil, nil, &clearUser); err != nil {
		t.Fatalf("clear portal account: %v", err)
	}
	if err := repo.Update(ctx, tenantA, got); err != nil {
		t.Fatalf("persist cleared portal account: %v", err)
	}
	cleared, err := repo.GetByID(ctx, tenantA, s.ID)
	if err != nil || cleared.UserID != nil {
		t.Fatalf("portal account not cleared: %+v err=%v", cleared, err)
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

func TestRepository_ListStudentIDsByClassIDs(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	classX := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	classY := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	s1 := mustCreateWithClass(ctx, t, repo, tenantA, "Ama", "One", &classX, nil)
	s2 := mustCreateWithClass(ctx, t, repo, tenantA, "Kojo", "Two", &classY, nil)
	withdrawn := mustCreateWithClass(ctx, t, repo, tenantA, "Esi", "Three", &classX, nil)
	withdrawn.Status = string(domain.StatusWithdrawn)
	if err := repo.Update(ctx, tenantA, withdrawn); err != nil {
		t.Fatal(err)
	}
	ids, err := repo.ListStudentIDsByClassIDs(ctx, tenantA, []string{classX, classY})
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	found := map[string]bool{ids[0]: true, ids[1]: true}
	if !found[s1.ID] || !found[s2.ID] || found[withdrawn.ID] {
		t.Fatalf("unexpected active roster ids=%v", ids)
	}
}
