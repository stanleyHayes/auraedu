// Package adapters_test runs one contract against every student repository driver.
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
	mongoadapter "github.com/auraedu/student-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/student-service/internal/adapters/postgres"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
)

type studentRepo interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
}

// tenantCtx carries the tenant the way the application layer does: the Postgres
// adapter derives app.tenant_id from it for RLS, and the Mongo adapter's Scope
// reads the same context.
func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func newStudent(tenantID string) *domain.Student {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Student{
		ID: uuid.NewString(), TenantID: tenantID, FirstName: "Kwame", LastName: "Asante",
		StudentCode: "ST-" + uuid.NewString()[:8], Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
}

func newGuardian(tenantID string) *domain.Guardian {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Guardian{
		ID: uuid.NewString(), TenantID: tenantID, FirstName: "Akosua", LastName: "Asante",
		Relationship: "mother", CreatedAt: now, UpdatedAt: now,
	}
}

func runStudentContract(t *testing.T, repo studentRepo) {
	const tenant = "upshs"
	const other = "aboom"
	ctx := tenantCtx(tenant)

	t.Run("create then read back within the tenant", func(t *testing.T) {
		s := newStudent(tenant)
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetByID(ctx, tenant, s.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.StudentCode != s.StudentCode || got.TenantID != tenant {
			t.Fatalf("round trip changed the record: %+v", got)
		}
	})

	t.Run("another tenant cannot read, update or delete the record", func(t *testing.T) {
		s := newStudent(tenant)
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.GetByID(tenantCtx(other), other, s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
		clone := *s
		clone.LastName = "Hijacked"
		if err := repo.Update(tenantCtx(other), other, &clone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant update returned %v; want ErrNotFound", err)
		}
		if err := repo.Delete(tenantCtx(other), other, s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
		got, err := repo.GetByID(ctx, tenant, s.ID)
		if err != nil || got.LastName == "Hijacked" {
			t.Fatalf("the owner's record was altered across tenants: %+v err=%v", got, err)
		}
	})

	t.Run("delete removes the record from reads", func(t *testing.T) {
		s := newStudent(tenant)
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := repo.Delete(ctx, tenant, s.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := repo.GetByID(ctx, tenant, s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a deleted record is still readable: %v", err)
		}
	})

	t.Run("list pages by cursor without repeating", func(t *testing.T) {
		for range 3 {
			if err := repo.Create(ctx, tenant, newStudent(tenant)); err != nil {
				t.Fatalf("create: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			page, next, err := repo.List(ctx, tenant, nil, 2, cursor)
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
		if len(seen) < 3 {
			t.Fatalf("paging saw %d records; want at least the 3 created", len(seen))
		}
	})

	t.Run("a class filter narrows the roster", func(t *testing.T) {
		classID := uuid.NewString()
		s := newStudent(tenant)
		s.ClassID = &classID
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, _, err := repo.List(ctx, tenant, &classID, 25, "")
		if err != nil || len(got) != 1 || got[0].ID != s.ID {
			t.Fatalf("class roster returned %d records err=%v", len(got), err)
		}
		ids, err := repo.ListStudentIDsByClassIDs(ctx, tenant, []string{classID})
		if err != nil || len(ids) != 1 || ids[0] != s.ID {
			t.Fatalf("class ids = %v err=%v", ids, err)
		}
	})

	t.Run("a student is reachable by its linked identity user", func(t *testing.T) {
		userID := uuid.NewString()
		s := newStudent(tenant)
		s.UserID = &userID
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetStudentByUserID(ctx, tenant, userID)
		if err != nil || got.ID != s.ID {
			t.Fatalf("get by user id: %+v err=%v", got, err)
		}
		if _, err := repo.GetStudentByUserID(tenantCtx(other), other, userID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a linked student leaked across tenants: %v", err)
		}
	})

	t.Run("guardians link to students in both directions", func(t *testing.T) {
		s := newStudent(tenant)
		g := newGuardian(tenant)
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create student: %v", err)
		}
		if err := repo.CreateGuardian(ctx, tenant, g); err != nil {
			t.Fatalf("create guardian: %v", err)
		}
		link, err := domain.NewStudentGuardian(tenant, s.ID, g.ID, nil, true)
		if err != nil {
			t.Fatalf("new link: %v", err)
		}
		if err := repo.LinkGuardianToStudent(ctx, tenant, link); err != nil {
			t.Fatalf("link: %v", err)
		}

		children, err := repo.ListStudentsByGuardian(ctx, tenant, g.ID)
		if err != nil || len(children) != 1 || children[0].ID != s.ID {
			t.Fatalf("students by guardian: %d records err=%v", len(children), err)
		}
		guardians, _, err := repo.ListGuardiansByStudent(ctx, tenant, s.ID, 25, "")
		if err != nil || len(guardians) != 1 || guardians[0].ID != g.ID {
			t.Fatalf("guardians by student: %d records err=%v", len(guardians), err)
		}

		// The link must not be visible to another tenant.
		leaked, err := repo.ListStudentsByGuardian(tenantCtx(other), other, g.ID)
		if err == nil && len(leaked) != 0 {
			t.Fatalf("a guardian link leaked across tenants: %+v", leaked)
		}

		if err := repo.UnlinkGuardianFromStudent(ctx, tenant, s.ID, g.ID); err != nil {
			t.Fatalf("unlink: %v", err)
		}
		after, err := repo.ListStudentsByGuardian(ctx, tenant, g.ID)
		if err != nil || len(after) != 0 {
			t.Fatalf("unlink left %d links err=%v", len(after), err)
		}
	})

	t.Run("a lifecycle commit persists the record and queues one event", func(t *testing.T) {
		drain(t, repo)
		s := newStudent(tenant)
		err := repo.CommitStudentLifecycle(ctx, tenant,
			ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: s},
			"student.created.v1", ports.StudentEventData(s, nil))
		if err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		if _, err := repo.GetByID(ctx, tenant, s.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the record: %v", err)
		}

		claimed, err := repo.ClaimPendingStudentEvents(ctx, 100)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 || claimed[0].EventType != "student.created.v1" {
			t.Fatalf("claimed %d events: %+v", len(claimed), claimed)
		}
		if err := repo.MarkStudentEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := repo.ClaimPendingStudentEvents(ctx, 100)
		if err != nil || len(again) != 0 {
			t.Fatalf("a published event was claimed again: %d err=%v", len(again), err)
		}
	})

	t.Run("a lifecycle delete still emits its event", func(t *testing.T) {
		drain(t, repo)
		s := newStudent(tenant)
		if err := repo.Create(ctx, tenant, s); err != nil {
			t.Fatalf("create: %v", err)
		}
		err := repo.CommitStudentLifecycle(ctx, tenant,
			ports.LifecycleMutation{Kind: ports.MutationStudentDelete, Student: s},
			"student.deleted.v1", ports.StudentEventData(s, nil))
		if err != nil {
			t.Fatalf("commit delete: %v", err)
		}
		if _, err := repo.GetByID(ctx, tenant, s.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a lifecycle-deleted record is still readable: %v", err)
		}
		claimed, err := repo.ClaimPendingStudentEvents(ctx, 100)
		if err != nil || len(claimed) != 1 || claimed[0].EventType != "student.deleted.v1" {
			t.Fatalf("delete event not queued: %d %+v err=%v", len(claimed), claimed, err)
		}
		if err := repo.MarkStudentEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		drain(t, repo)
		s := newStudent(tenant)
		if err := repo.CommitStudentLifecycle(ctx, tenant,
			ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: s},
			"student.created.v1", ports.StudentEventData(s, nil)); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingStudentEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d err=%v", len(claimed), err)
		}
		if err := repo.MarkStudentEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		if err := repo.MarkStudentEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})
}

func drain(t *testing.T, repo studentRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingStudentEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkStudentEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresStudentRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runStudentContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoStudentRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runStudentContract(t, mongoadapter.NewRepository(mg.Store))
}
