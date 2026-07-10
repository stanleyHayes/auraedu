package integration

import (
	"context"
	"testing"

	"github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

func newRepo(t *testing.T) (ports.Repository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func mustCreate(t *testing.T, ctx context.Context, repo ports.Repository, tenantID string) *domain.FileUpload {
	t.Helper()
	f, err := domain.NewFileUpload(tenantID, "report.pdf", "application/pdf", "user-1", "report", 1024, "checksum")
	if err != nil {
		t.Fatalf("new file upload: %v", err)
	}
	f.StoragePath = "/tmp/report-" + f.ID + ".pdf"
	f.Status = string(domain.StatusActive)
	if err := repo.Create(ctx, tenantID, f); err != nil {
		t.Fatalf("create file upload: %v", err)
	}
	return f
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	f := mustCreate(t, ctx, repo, tenantA)

	got, err := repo.GetByID(ctx, tenantA, f.ID)
	if err != nil {
		t.Fatalf("get file upload: %v", err)
	}
	if got.ID != f.ID || got.OriginalFilename != "report.pdf" {
		t.Fatalf("file upload mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreate(t, ctx, repo, tenantA)
	f2 := mustCreate(t, ctx, repo, tenantA)

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
	if len(page2) != 1 || page2[0].ID != f2.ID {
		t.Fatalf("expected second file upload, got %+v", page2)
	}
}

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	f := mustCreate(t, ctx, repo, tenantA)
	name := "updated.pdf"
	if _, err := f.ApplyUpdate(&name, nil, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, f); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, f.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.OriginalFilename != "updated.pdf" {
		t.Fatalf("filename not updated: %q", got.OriginalFilename)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	f := mustCreate(t, ctx, repo, tenantA)
	if err := repo.Delete(ctx, tenantA, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, f.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	f := mustCreate(t, aCtx, repo, tenantA)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, f.ID); err == nil {
		t.Fatal("tenant B should not see tenant A file upload")
	}

	list, _, err := repo.List(bCtx, tenantB, 10, "")
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 file uploads, got %d", len(list))
	}
}
