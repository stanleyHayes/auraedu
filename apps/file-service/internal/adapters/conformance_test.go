// Package adapters_test runs one contract against every file repository driver.
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
	mongoadapter "github.com/auraedu/file-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/google/uuid"
)

type fileRepo interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
}

// tenantCtx carries the tenant the way the application layer does: the Postgres
// adapter derives app.tenant_id from it so RLS applies, and the Mongo adapter's
// Scope reads the same context. Both drivers see one calling convention.
func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func newFile(tenantID string) *domain.FileUpload {
	now := time.Now().UTC().Truncate(time.Second)
	id, _ := uuid.NewV7()
	return &domain.FileUpload{
		ID:               id.String(),
		TenantID:         tenantID,
		OriginalFilename: "test.pdf",
		StoragePath:      "/uploads/test.pdf",
		StorageBackend:   string(domain.BackendLocal),
		ContentType:      "application/pdf",
		SizeBytes:        1024,
		Checksum:         "abc123",
		OwnerID:          "user-1",
		Purpose:          "document",
		Status:           string(domain.StatusActive),
		Metadata:         make(map[string]any),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func runFileContract(t *testing.T, repo fileRepo) {
	t.Run("create then read back within the tenant", func(t *testing.T) {
		f := newFile("upshs")
		if err := repo.Create(tenantCtx("upshs"), "upshs", f); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetByID(tenantCtx("upshs"), "upshs", f.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.OriginalFilename != "test.pdf" || got.TenantID != "upshs" {
			t.Fatalf("round trip changed the record: %+v", got)
		}
	})

	t.Run("another tenant cannot read the record", func(t *testing.T) {
		f := newFile("upshs")
		if err := repo.Create(tenantCtx("upshs"), "upshs", f); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.GetByID(tenantCtx("aboom"), "aboom", f.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("another tenant cannot update or delete the record", func(t *testing.T) {
		f := newFile("upshs")
		if err := repo.Create(tenantCtx("upshs"), "upshs", f); err != nil {
			t.Fatalf("create: %v", err)
		}
		clone := *f
		clone.OriginalFilename = "hijacked.pdf"
		if err := repo.Update(tenantCtx("aboom"), "aboom", &clone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant update returned %v; want ErrNotFound", err)
		}
		if err := repo.Delete(tenantCtx("aboom"), "aboom", f.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
		got, err := repo.GetByID(tenantCtx("upshs"), "upshs", f.ID)
		if err != nil || got.OriginalFilename == "hijacked.pdf" {
			t.Fatalf("the owner's record was altered across tenants: %+v (err=%v)", got, err)
		}
	})

	t.Run("delete removes the record from reads", func(t *testing.T) {
		f := newFile("upshs")
		if err := repo.Create(tenantCtx("upshs"), "upshs", f); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := repo.Delete(tenantCtx("upshs"), "upshs", f.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := repo.GetByID(tenantCtx("upshs"), "upshs", f.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a deleted record is still readable: %v", err)
		}
	})

	t.Run("list is tenant scoped and pages by cursor", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		for i := 0; i < 3; i++ {
			if err := repo.Create(ctx, tenant, newFile(tenant)); err != nil {
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
			for _, f := range page {
				if f.TenantID != tenant {
					t.Fatalf("list leaked a record owned by %q", f.TenantID)
				}
				if seen[f.ID] {
					t.Fatalf("the cursor repeated record %s", f.ID)
				}
				seen[f.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < 3 {
			t.Fatalf("paging saw %d records; want at least the 3 just created", len(seen))
		}
	})

	t.Run("a lifecycle commit queues exactly one claimable event", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		f := newFile(tenant)
		payload := map[string]any{
			"id":        f.ID,
			"tenant_id": f.TenantID,
			"status":    f.Status,
		}
		if err := repo.CommitFileLifecycle(ctx, tenant, f, ports.FileMutationCreate, "file.uploaded.v1", payload); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		// The record is readable, so the domain half of the write landed.
		if _, err := repo.GetByID(ctx, tenant, f.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the record: %v", err)
		}

		claimed, err := repo.ClaimPendingFileEvents(ctx, 100)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 {
			t.Fatalf("claimed %d events; want the one just queued", len(claimed))
		}
		if claimed[0].EventType != "file.uploaded.v1" {
			t.Fatalf("unexpected event type %q", claimed[0].EventType)
		}

		if err := repo.MarkFileEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := repo.ClaimPendingFileEvents(ctx, 100)
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

		f := newFile(tenant)
		if err := repo.CommitFileLifecycle(ctx, tenant, f, ports.FileMutationCreate,
			"file.uploaded.v1", map[string]any{
				"id":        f.ID,
				"tenant_id": f.TenantID,
			}); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingFileEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d events, err=%v", len(claimed), err)
		}
		if err := repo.MarkFileEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		// A failure must not lose the event: it is still there to acknowledge.
		if err := repo.MarkFileEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})
}

// drain empties the outbox so a subtest counts only the events it queued.
func drain(t *testing.T, repo fileRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingFileEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkFileEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresFileRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runFileContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoFileRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runFileContract(t, mongoadapter.NewRepository(mg.Store))
}
