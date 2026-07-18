package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/student-service/internal/adapters/postgres"
	"github.com/auraedu/student-service/internal/application"
)

func newService(t *testing.T) *application.Service {
	t.Helper()
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	gates.Set(tenantB, application.FeatureStudentManagement, true)

	return application.NewService(repo, application.WithFeatureGate(gates))
}

func actorWith(perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: perms}
}

func TestService_CreateAndGetRoundtrip(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	actor := actorWith(application.PermCreate, application.PermRead)
	created, err := svc.Create(ctx, actor, application.CreateStudentRequest{
		FirstName: "Kwame",
		LastName:  "Nkrumah",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.StudentCode == "" {
		t.Fatal("expected student_code to be generated")
	}

	got, err := svc.Get(ctx, actor, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.FullName() != "Kwame Nkrumah" {
		t.Fatalf("student mismatch: %+v", got)
	}
}

func TestService_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	actor := actorWith(application.PermCreate, application.PermRead)
	if _, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "A", LastName: "One"}); err != nil {
		t.Fatalf("create first: %v", err)
	}
	s2, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "B", LastName: "Two"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	page, next, err := svc.List(ctx, actor, nil, 1, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := svc.List(ctx, actor, nil, 1, next)
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != s2.ID {
		t.Fatalf("expected second student, got %+v", page2)
	}
}

func TestService_UpdateAndDelete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	actor := actorWith(application.PermCreate, application.PermRead, application.PermUpdate, application.PermDelete)
	created, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Yaa", LastName: "Asantewaa"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "Nana"
	updated, err := svc.Update(ctx, actor, created.ID, application.UpdateStudentRequest{FirstName: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.FirstName != "Nana" {
		t.Fatalf("first name not updated: %q", updated.FirstName)
	}

	if err := svc.Delete(ctx, actor, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(ctx, actor, created.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestService_FeatureFlagDisabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)

	// Feature flag disabled for tenantA.
	svc := application.NewService(repo, application.WithFeatureGate(flags.NewStaticSnapshot()))
	actor := actorWith(application.PermCreate)

	_, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "X", LastName: "Y"})
	if err == nil {
		t.Fatal("expected error when feature flag is disabled")
	}
}

func TestService_TenantIsolation(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	actorA := actorWith(application.PermCreate, application.PermRead)
	actorB := auth.Actor{UserID: "user-2", TenantID: tenantB, Permissions: []string{application.PermRead}}

	created, err := svc.Create(ctx, actorA, application.CreateStudentRequest{FirstName: "Tenant", LastName: "A"})
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}

	bCtx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantB})
	if _, err := svc.Get(bCtx, actorB, created.ID); err == nil {
		t.Fatal("tenant B should not see tenant A student")
	}
}

func TestService_MissingPermission(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	actor := actorWith() // no permissions
	_, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "X", LastName: "Y"})
	if err == nil {
		t.Fatal("expected error when actor lacks permission")
	}
}

func TestService_ListFilterByClass(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc := newService(t)

	classX := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	classY := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	yearY := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	actor := actorWith(application.PermCreate, application.PermRead)
	s1, err := svc.Create(ctx, actor, application.CreateStudentRequest{
		FirstName: "Ama", LastName: "One", ClassID: &classX, AcademicYearID: &yearY,
	})
	if err != nil {
		t.Fatalf("create s1: %v", err)
	}
	// The use-case result already carries the persisted assignment.
	if s1.ClassID == nil || *s1.ClassID != classX || s1.AcademicYearID == nil || *s1.AcademicYearID != yearY {
		t.Fatalf("create result missing class fields: %+v", s1)
	}
	if _, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Kojo", LastName: "Two", ClassID: &classY}); err != nil {
		t.Fatalf("create s2: %v", err)
	}
	if _, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Esi", LastName: "Three"}); err != nil {
		t.Fatalf("create s3: %v", err)
	}

	// Filter set: only the classX roster.
	roster, _, err := svc.List(ctx, actor, &classX, 10, "")
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(roster) != 1 || roster[0].ID != s1.ID {
		t.Fatalf("expected only student %s, got %+v", s1.ID, roster)
	}

	// Filter unset: all three.
	all, _, err := svc.List(ctx, actor, nil, 10, "")
	if err != nil {
		t.Fatalf("list unfiltered: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 students without filter, got %d", len(all))
	}

	// Non-UUID class_id is rejected before hitting the database.
	if _, _, err := svc.List(ctx, actor, strPtr("not-a-uuid"), 10, ""); err == nil {
		t.Fatal("expected validation error for non-UUID class_id")
	}

	// Cross-tenant: tenant B's roster for the same class_id is empty.
	actorB := auth.Actor{UserID: "user-2", TenantID: tenantB, Permissions: []string{application.PermRead}}
	bCtx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantB})
	bRoster, _, err := svc.List(bCtx, actorB, &classX, 10, "")
	if err != nil {
		t.Fatalf("list tenant B filtered: %v", err)
	}
	if len(bRoster) != 0 {
		t.Fatalf("tenant B should see an empty roster, got %+v", bRoster)
	}
}
