package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/staff-service/internal/adapters/postgres"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
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

func mustCreate(ctx context.Context, t *testing.T, repo ports.Repository, first, last, staffType string) *domain.Staff {
	t.Helper()
	s, err := domain.NewStaff(tenantA, first, last, staffType)
	if err != nil {
		t.Fatalf("new staff: %v", err)
	}
	if err := repo.Create(ctx, tenantA, s); err != nil {
		t.Fatalf("create staff: %v", err)
	}
	return s
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Kwame", "Nkrumah", "teacher")

	got, err := repo.GetByID(ctx, tenantA, s.ID)
	if err != nil {
		t.Fatalf("get staff: %v", err)
	}
	if got.ID != s.ID || got.FirstName != "Kwame" {
		t.Fatalf("staff mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	mustCreate(ctx, t, repo, "A", "One", "teacher")
	s2 := mustCreate(ctx, t, repo, "B", "Two", "non_teaching")

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
		t.Fatalf("expected second staff, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Yaa", "Asantewaa", "teacher")
	newName := "Nana"
	if _, err := s.ApplyUpdate(&newName, nil, nil, nil, nil, nil); err != nil {
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

func TestRepository_GetByUserID(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Ama", "Mensah", "teacher")
	userID := "33333333-3333-4333-8333-333333333333"
	s.UserID = &userID
	if err := repo.Update(ctx, tenantA, s); err != nil {
		t.Fatalf("link identity user: %v", err)
	}

	got, err := repo.GetByUserID(ctx, tenantA, userID)
	if err != nil {
		t.Fatalf("get by user id: %v", err)
	}
	if got.ID != s.ID || got.UserID == nil || *got.UserID != userID {
		t.Fatalf("linked staff mismatch: %+v", got)
	}
	if _, err := repo.GetByUserID(withTenant(context.Background(), tenantB), tenantB, userID); err == nil {
		t.Fatal("tenant B must not resolve tenant A's staff identity")
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	s := mustCreate(ctx, t, repo, "Kofi", "Annan", "teacher")
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
	s := mustCreate(aCtx, t, repo, "Tenant", "A", "teacher")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, s.ID); err == nil {
		t.Fatal("tenant B should not see tenant A staff")
	}

	list, _, err := repo.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 staff, got %d", len(list))
	}
}

func TestRepository_LifecycleMutationAndOutboxAreAtomic(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	staff, err := domain.NewStaff(tenantA, "Durable", "Teacher", "teacher")
	if err != nil {
		t.Fatal(err)
	}
	payload := ports.StaffEventData(staff, nil)
	if err := repo.CommitStaffLifecycle(ctx, tenantA, staff, ports.StaffMutationCreate, "staff.created.v1", payload); err != nil {
		t.Fatalf("commit lifecycle: %v", err)
	}
	items, err := repo.ClaimPendingStaffEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(items) != 1 || items[0].EventType != "staff.created.v1" || items[0].TenantID != tenantA {
		t.Fatalf("unexpected outbox items: %+v", items)
	}

	rollbackStaff, err := domain.NewStaff(tenantA, "Rollback", "Teacher", "teacher")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE staff_outbox`); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	if err := repo.CommitStaffLifecycle(ctx, tenantA, rollbackStaff, ports.StaffMutationCreate, "staff.created.v1", ports.StaffEventData(rollbackStaff, nil)); err == nil {
		t.Fatal("expected outbox failure")
	}
	if _, err := repo.GetByID(ctx, tenantA, rollbackStaff.ID); err == nil {
		t.Fatal("staff mutation must roll back when outbox insert fails")
	}
}

func TestRepository_AssignmentScopeAndEventAreTenantIsolated(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	staff := mustCreate(ctx, t, repo, "Scoped", "Teacher", "teacher")
	assignment, err := domain.NewAssignment(tenantA, staff.ID, "44444444-4444-4444-8444-444444444444", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAssignment(ctx, tenantA, assignment, ports.AssignmentEventData(assignment)); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	rows, _, err := repo.ListAssignments(ctx, tenantA, staff.ID, 25, "")
	if err != nil || len(rows) != 1 || rows[0].ClassID != assignment.ClassID {
		t.Fatalf("assignments=%+v err=%v", rows, err)
	}
	classIDs, err := repo.ListAssignmentClassIDs(ctx, tenantA, staff.ID)
	if err != nil || len(classIDs) != 1 || classIDs[0] != assignment.ClassID {
		t.Fatalf("class_ids=%v err=%v", classIDs, err)
	}
	otherRows, _, err := repo.ListAssignments(withTenant(context.Background(), tenantB), tenantB, staff.ID, 25, "")
	if err != nil || len(otherRows) != 0 {
		t.Fatalf("tenant B assignments=%+v err=%v", otherRows, err)
	}
	items, err := repo.ClaimPendingStaffEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.EventType == "staff.assigned.v1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("staff.assigned.v1 event not queued: %+v", items)
	}
	if err := repo.DeleteAssignment(ctx, tenantA, staff.ID, assignment.ID); err != nil {
		t.Fatal(err)
	}
}
