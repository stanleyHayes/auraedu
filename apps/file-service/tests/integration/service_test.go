package integration

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/adapters/storage"
	"github.com/auraedu/file-service/internal/application"
	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
)

func TestService_UploadDownloadRoundtrip(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)

	dir := t.TempDir()
	store := storage.NewLocalStorage(dir)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureFileManagement, true)

	svc := application.NewService(repo, store, application.WithFeatureGate(gates))
	actor := auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: []string{application.PermCreate, application.PermRead}}

	content := []byte("hello auraedu file service")
	file, err := svc.Create(ctx, actor, application.CreateFileRequest{
		OriginalFilename: "hello.txt",
		ContentType:      "text/plain",
		Purpose:          "document",
		Data:             content,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if file.SizeBytes != int64(len(content)) {
		t.Fatalf("size mismatch: got %d want %d", file.SizeBytes, len(content))
	}
	if file.Checksum != domain.ComputeChecksum(content) {
		t.Fatalf("checksum mismatch")
	}

	got, rc, err := svc.Download(ctx, actor, file.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer rc.Close()

	downloaded, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(downloaded, content) {
		t.Fatalf("download content mismatch: got %q want %q", downloaded, content)
	}
	if got.ID != file.ID {
		t.Fatalf("download metadata mismatch")
	}
}

func TestService_FeatureFlagDisabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	store := storage.NewLocalStorage(t.TempDir())

	// Gate with file_management disabled for tenantA.
	svc := application.NewService(repo, store, application.WithFeatureGate(flags.NewStaticSnapshot()))
	actor := auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: []string{application.PermCreate}}

	_, err := svc.Create(ctx, actor, application.CreateFileRequest{
		OriginalFilename: "x.txt",
		ContentType:      "text/plain",
		Data:             []byte("x"),
	})
	if err == nil {
		t.Fatal("expected error when feature flag is disabled")
	}
}

func TestService_TenantIsolation(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	store := storage.NewLocalStorage(t.TempDir())

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureFileManagement, true)
	gates.Set(tenantB, application.FeatureFileManagement, true)

	svc := application.NewService(repo, store, application.WithFeatureGate(gates))
	actorA := auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: []string{application.PermCreate, application.PermRead}}
	actorB := auth.Actor{UserID: "user-2", TenantID: tenantB, Permissions: []string{application.PermRead}}

	file, err := svc.Create(ctx, actorA, application.CreateFileRequest{
		OriginalFilename: "tenant-a.txt",
		ContentType:      "text/plain",
		Data:             []byte("a"),
	})
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}

	bCtx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantB})
	if _, err := svc.Get(bCtx, actorB, file.ID); err == nil {
		t.Fatal("tenant B should not see tenant A file")
	}
}
