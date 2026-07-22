package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestFileLifecycleCommitsEventsAndDeferredCleanup(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	file, err := domain.NewFileUpload(tenantA, "proof.pdf", "application/pdf", "user-1", "report", 100, "checksum")
	if err != nil {
		t.Fatal(err)
	}
	file.StoragePath = "/tmp/proof-" + file.ID
	file.Status = string(domain.StatusActive)
	if err := repo.CommitFileLifecycle(ctx, tenantA, file, ports.FileMutationCreate, "file.uploaded.v1", ports.FileEventData(file, nil)); err != nil {
		t.Fatal(err)
	}
	name := "renamed.pdf"
	changed, err := file.ApplyUpdate(&name, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitFileLifecycle(ctx, tenantA, file, ports.FileMutationUpdate, "file.updated.v1", ports.FileEventData(file, map[string]any{"changed_fields": changed})); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitFileLifecycle(ctx, tenantA, file, ports.FileMutationDelete, "file.deleted.v1", ports.FileEventData(file, nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, tenantA, file.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted file remained: %v", err)
	}
	outbox, err := repo.ClaimPendingFileEvents(context.Background(), 10)
	if err != nil || len(outbox) != 3 {
		t.Fatalf("outbox=%+v err=%v", outbox, err)
	}
	counts := map[string]int{}
	for _, item := range outbox {
		counts[item.EventType]++
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil || payload["file_id"] != file.ID {
			t.Fatalf("payload=%+v err=%v", payload, err)
		}
		if item.EventType == "file.deleted.v1" && item.CleanupPath != file.StoragePath {
			t.Fatalf("cleanup path=%q", item.CleanupPath)
		}
	}
	if counts["file.uploaded.v1"] != 1 || counts["file.updated.v1"] != 1 || counts["file.deleted.v1"] != 1 {
		t.Fatalf("counts=%+v", counts)
	}
}

func TestFileLifecycleRollsBackWithoutOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	file, err := domain.NewFileUpload(tenantA, "rollback.pdf", "application/pdf", "user-1", "report", 100, "checksum")
	if err != nil {
		t.Fatal(err)
	}
	file.StoragePath = "/tmp/" + file.ID
	file.Status = string(domain.StatusActive)
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE file_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CommitFileLifecycle(ctx, tenantA, file, ports.FileMutationCreate, "file.uploaded.v1", ports.FileEventData(file, nil)); err == nil {
		t.Fatal("create must fail without outbox")
	}
	if _, err := repo.GetByID(ctx, tenantA, file.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mutation escaped rollback: %v", err)
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

func mustCreate(ctx context.Context, t *testing.T, repo ports.Repository) *domain.FileUpload {
	t.Helper()
	f, err := domain.NewFileUpload(tenantA, "report.pdf", "application/pdf", "user-1", "report", 1024, "checksum")
	if err != nil {
		t.Fatalf("new file upload: %v", err)
	}
	f.StoragePath = "/tmp/report-" + f.ID + ".pdf"
	f.Status = string(domain.StatusActive)
	if err := repo.Create(ctx, tenantA, f); err != nil {
		t.Fatalf("create file upload: %v", err)
	}
	return f
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	f := mustCreate(ctx, t, repo)

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
	repo := newRepo(t)

	mustCreate(ctx, t, repo)
	f2 := mustCreate(ctx, t, repo)

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
	repo := newRepo(t)

	f := mustCreate(ctx, t, repo)
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
	repo := newRepo(t)

	f := mustCreate(ctx, t, repo)
	if err := repo.Delete(ctx, tenantA, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, f.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	f := mustCreate(aCtx, t, repo)

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

func TestFileUsageRLSWithRuntimeRole(t *testing.T) {
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	_, err := tdb.DB.Pool().Exec(ctx, `
		CREATE ROLE file_runtime LOGIN PASSWORD 'runtime-test';
		GRANT USAGE ON SCHEMA public TO file_runtime;
		GRANT SELECT, INSERT ON file_usage TO file_runtime;
		INSERT INTO file_usage (tenant_id, date, bytes_stored) VALUES
			('upshs', CURRENT_DATE, 100),
			('aboom-ame-zion-c', CURRENT_DATE, 200);
	`)
	if err != nil {
		t.Fatalf("seed runtime RLS probe: %v", err)
	}

	runtimeDSN := strings.Replace(tdb.DSN, "test:test@", "file_runtime:runtime-test@", 1)
	runtime, err := pgxpool.New(ctx, runtimeDSN)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	defer runtime.Close()

	tx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatalf("begin runtime read: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", "upshs"); err != nil {
		t.Fatalf("set runtime tenant: %v", err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM file_usage`).Scan(&count); err != nil {
		t.Fatalf("query runtime usage: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected runtime tenant to see one usage row, got %d", count)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit runtime read: %v", err)
	}

	crossTx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatalf("begin cross-tenant write: %v", err)
	}
	defer crossTx.Rollback(ctx) //nolint:errcheck // Cleanup is best effort after the test's expected policy rejection.
	if _, err := crossTx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", "upshs"); err != nil {
		t.Fatalf("set cross-tenant context: %v", err)
	}
	if _, err := crossTx.Exec(ctx, `
		INSERT INTO file_usage (tenant_id, date, bytes_stored)
		VALUES ('aboom-ame-zion-c', CURRENT_DATE - 1, 300)
	`); err == nil {
		t.Fatal("expected cross-tenant usage write to be denied by RLS")
	}
}
