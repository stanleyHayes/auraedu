// Package adapters_test runs one contract against every staff repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	mongoadapter "github.com/auraedu/staff-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/staff-service/internal/adapters/postgres"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
	"github.com/google/uuid"
)

type staffRepo interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
	ports.AssignmentRepository
}

// tenantCtx carries the tenant the way the application layer does: the Postgres
// adapter derives app.tenant_id from it so RLS applies, and the Mongo adapter's
// Scope reads the same context. Both drivers see one calling convention.
func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

// newStaff builds the aggregate directly rather than through the domain
// constructor, which is what makes this a storage test. Note that PostgreSQL
// also enforces the staff_type enumeration with a CHECK constraint while
// MongoDB has no equivalent: the domain constructor is the only guard that
// exists on both drivers.
func newStaff(tenantID, code string) *domain.Staff {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Staff{
		ID: uuid.NewString(), TenantID: tenantID, FirstName: "Efua", LastName: "Mensah",
		StaffType: string(domain.StaffTypeTeacher), StaffCode: code, Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
}

func runStaffContract(t *testing.T, repo staffRepo) {
	t.Run("create then read back within the tenant", func(t *testing.T) {
		s := newStaff("upshs", "ST-100")
		if err := repo.Create(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetByID(tenantCtx("upshs"), "upshs", s.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.StaffCode != "ST-100" || got.TenantID != "upshs" {
			t.Fatalf("round trip changed the record: %+v", got)
		}
	})

	t.Run("another tenant cannot read the record", func(t *testing.T) {
		s := newStaff("upshs", "ST-101")
		if err := repo.Create(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.GetByID(tenantCtx("aboom"), "aboom", s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("another tenant cannot update or delete the record", func(t *testing.T) {
		s := newStaff("upshs", "ST-102")
		if err := repo.Create(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create: %v", err)
		}
		clone := *s
		clone.LastName = "Hijacked"
		if err := repo.Update(tenantCtx("aboom"), "aboom", &clone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant update returned %v; want ErrNotFound", err)
		}
		if err := repo.Delete(tenantCtx("aboom"), "aboom", s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
		got, err := repo.GetByID(tenantCtx("upshs"), "upshs", s.ID)
		if err != nil || got.LastName == "Hijacked" {
			t.Fatalf("the owner's record was altered across tenants: %+v (err=%v)", got, err)
		}
	})

	t.Run("delete removes the record from reads", func(t *testing.T) {
		s := newStaff("upshs", "ST-103")
		if err := repo.Create(tenantCtx("upshs"), "upshs", s); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := repo.Delete(tenantCtx("upshs"), "upshs", s.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := repo.GetByID(tenantCtx("upshs"), "upshs", s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a deleted record is still readable: %v", err)
		}
	})

	t.Run("list is tenant scoped and pages by cursor", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		codes := []string{"LP-A", "LP-B", "LP-C"}
		for _, code := range codes {
			if err := repo.Create(ctx, tenant, newStaff(tenant, code)); err != nil {
				t.Fatalf("create: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			page, next, err := repo.List(ctx, tenant, 2, cursor)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			for _, s := range page {
				if s.TenantID != tenant {
					t.Fatalf("list leaked a record owned by %q", s.TenantID)
				}
				if seen[s.ID] {
					t.Fatalf("the cursor repeated record %s", s.ID)
				}
				seen[s.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < len(codes) {
			t.Fatalf("paging saw %d records; want at least the %d just created", len(seen), len(codes))
		}
	})

	t.Run("a lifecycle commit queues exactly one claimable event", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo) // start from a quiet outbox

		s := newStaff(tenant, "LC-1")
		payload := ports.StaffEventData(s, map[string]any{"mutation": "create"})
		if err := repo.CommitStaffLifecycle(ctx, tenant, s, ports.StaffMutationCreate, "staff.created.v1", payload); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		// The record is readable, so the domain half of the write landed.
		if _, err := repo.GetByID(ctx, tenant, s.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the record: %v", err)
		}

		claimed, err := repo.ClaimPendingStaffEvents(ctx, 100)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 {
			t.Fatalf("claimed %d events; want the one just queued", len(claimed))
		}
		if claimed[0].EventType != "staff.created.v1" {
			t.Fatalf("unexpected event type %q", claimed[0].EventType)
		}

		if err := repo.MarkStaffEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := repo.ClaimPendingStaffEvents(ctx, 100)
		if err != nil {
			t.Fatalf("second claim: %v", err)
		}
		if len(again) != 0 {
			t.Fatalf("a published event was claimed again: %+v", again)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		s := newStaff(tenant, "FL-1")
		if err := repo.CommitStaffLifecycle(ctx, tenant, s, ports.StaffMutationCreate,
			"staff.created.v1", ports.StaffEventData(s, nil)); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingStaffEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d events, err=%v", len(claimed), err)
		}
		if err := repo.MarkStaffEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		// A failure must not lose the event: it is still there to acknowledge.
		if err := repo.MarkStaffEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})

	t.Run("assignments are tenant scoped and expose their ids", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		s := newStaff(tenant, "AS-1")
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create staff: %v", err)
		}
		classID := uuid.NewString()
		subjectID := uuid.NewString()
		assignment, err := domain.NewAssignment(tenant, s.ID, classID, &subjectID, nil)
		if err != nil {
			t.Fatalf("new assignment: %v", err)
		}
		if err := repo.CreateAssignment(ctx, tenant, assignment, ports.AssignmentEventData(assignment)); err != nil {
			t.Fatalf("create assignment: %v", err)
		}

		got, _, err := repo.ListAssignments(ctx, tenant, s.ID, 25, "")
		if err != nil || len(got) != 1 {
			t.Fatalf("list assignments: %d records, err=%v", len(got), err)
		}
		classes, err := repo.ListAssignmentClassIDs(ctx, tenant, s.ID)
		if err != nil || len(classes) != 1 || classes[0] != classID {
			t.Fatalf("class ids = %v, err=%v", classes, err)
		}
		subjects, err := repo.ListAssignmentSubjectIDs(ctx, tenant, s.ID)
		if err != nil || len(subjects) != 1 || subjects[0] != subjectID {
			t.Fatalf("subject ids = %v, err=%v", subjects, err)
		}

		other, err := repo.ListAssignmentClassIDs(tenantCtx("aboom"), "aboom", s.ID)
		if err == nil && len(other) != 0 {
			t.Fatalf("assignments leaked to another tenant: %v", other)
		}
	})
}

// drain empties the outbox so a subtest counts only the events it queued.
func drain(t *testing.T, repo staffRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingStaffEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkStaffEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresStaffRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runStaffContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoStaffRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runStaffContract(t, mongoadapter.NewRepository(mg.Store))
}
