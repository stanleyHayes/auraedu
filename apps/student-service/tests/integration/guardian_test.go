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

func newGuardianService(t *testing.T) (*application.Service, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	gates.Set(tenantB, application.FeatureStudentManagement, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	return svc, tdb
}

func TestGuardian_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc, _ := newGuardianService(t)

	actor := actorWith(application.PermCreate, application.PermRead)
	created, err := svc.CreateGuardian(ctx, actor, application.CreateGuardianRequest{
		FirstName:    "Father",
		LastName:     "Guardian",
		Relationship: "father",
	})
	if err != nil {
		t.Fatalf("create guardian: %v", err)
	}

	got, err := svc.GetGuardian(ctx, actor, created.ID)
	if err != nil {
		t.Fatalf("get guardian: %v", err)
	}
	if got.FullName() != "Father Guardian" {
		t.Fatalf("guardian mismatch: %+v", got)
	}
}

func TestGuardian_LinkToStudent(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc, _ := newGuardianService(t)
	actor := actorWith(application.PermCreate, application.PermRead, application.PermUpdate)

	student, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Child", LastName: "Student"})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	guardian, err := svc.CreateGuardian(ctx, actor, application.CreateGuardianRequest{
		FirstName: "Mother", LastName: "Guardian", Relationship: "mother",
	})
	if err != nil {
		t.Fatalf("create guardian: %v", err)
	}

	link, err := svc.LinkGuardian(ctx, actor, student.ID, application.LinkGuardianRequest{
		GuardianID: guardian.ID,
		IsPrimary:  true,
	})
	if err != nil {
		t.Fatalf("link guardian: %v", err)
	}
	if link.StudentID != student.ID || link.GuardianID != guardian.ID {
		t.Fatalf("link mismatch: %+v", link)
	}

	guardians, _, err := svc.ListStudentGuardians(ctx, actor, student.ID, 10, "")
	if err != nil {
		t.Fatalf("list guardians: %v", err)
	}
	if len(guardians) != 1 || guardians[0].ID != guardian.ID {
		t.Fatalf("expected 1 guardian, got %+v", guardians)
	}

	if err := svc.UnlinkGuardian(ctx, actor, student.ID, guardian.ID); err != nil {
		t.Fatalf("unlink guardian: %v", err)
	}
	guardians, _, err = svc.ListStudentGuardians(ctx, actor, student.ID, 10, "")
	if err != nil {
		t.Fatalf("list after unlink: %v", err)
	}
	if len(guardians) != 0 {
		t.Fatalf("expected 0 guardians after unlink, got %d", len(guardians))
	}
}

func TestGuardian_TenantIsolation(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc, _ := newGuardianService(t)

	actorA := actorWith(application.PermCreate, application.PermRead)
	actorB := auth.Actor{UserID: "user-2", TenantID: tenantB, Permissions: []string{application.PermRead}}

	guardian, err := svc.CreateGuardian(ctx, actorA, application.CreateGuardianRequest{
		FirstName: "Tenant", LastName: "A", Relationship: "parent",
	})
	if err != nil {
		t.Fatalf("create tenant A guardian: %v", err)
	}

	bCtx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantB})
	if _, err := svc.GetGuardian(bCtx, actorB, guardian.ID); err == nil {
		t.Fatal("tenant B should not see tenant A guardian")
	}
}
