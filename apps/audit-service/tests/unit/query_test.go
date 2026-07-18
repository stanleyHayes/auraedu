package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/memory"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

const (
	tenantAID = "11111111-1111-1111-1111-111111111111"
	tenantBID = "22222222-2222-2222-2222-222222222222"
)

func seedLog(t *testing.T, repo *memory.Repository, tenantID, eventType, actorID string) *domain.AuditLog {
	t.Helper()
	log, err := domain.NewAuditLogBuilder().
		TenantID(uuid.MustParse(tenantID)).
		EventID(uuid.NewString()).
		EventType(eventType).
		SourceService("test-service").
		Timestamp(time.Now().UTC()).
		ReceivedAt(time.Now().UTC()).
		ActorID(actorID).
		Action(eventType).
		ResourceType("student").
		ResourceID("stu-1").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := repo.Insert(context.Background(), log); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return log
}

func withTenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func tenantActor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Role: "tenant_admin", Permissions: perms}
}

func platformAdminActor() auth.Actor {
	return auth.Actor{UserID: "admin-1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
}

func TestQuery_ListAuditLogs_TenantScoped(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	seedLog(t, repo, tenantBID, "student.created.v1", "user-2")
	q := application.NewQuery(repo)

	logs, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead), 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].TenantID.String() != tenantAID {
		t.Fatalf("tenant mismatch: got %s, want %s", logs[0].TenantID, tenantAID)
	}
}

func TestQuery_ListAuditLogs_Unauthenticated(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())

	_, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), auth.Actor{}, 25, "")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestQuery_ListAuditLogs_MissingTenant(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())

	_, _, err := q.ListAuditLogs(context.Background(), tenantActor(tenantAID, application.PermRead), 25, "")
	if !errors.Is(err, domain.ErrMissingTenant) {
		t.Fatalf("expected ErrMissingTenant, got %v", err)
	}
}

func TestQuery_ListAuditLogs_TenantMismatch(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())

	_, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantBID, application.PermRead), 25, "")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestQuery_ListAuditLogs_MissingPermission(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())

	_, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID), 25, "")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestQuery_ListAuditLogs_PlatformAdminCrossTenant(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	seedLog(t, repo, tenantBID, "invoice.created.v1", "user-2")
	q := application.NewQuery(repo)

	// No tenant context: a platform super admin reads across tenants.
	logs, _, err := q.ListAuditLogs(context.Background(), platformAdminActor(), 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs across tenants, got %d", len(logs))
	}
}

func TestQuery_ListAuditLogs_PlatformAdminScopedTenant(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	seedLog(t, repo, tenantBID, "invoice.created.v1", "user-2")
	q := application.NewQuery(repo)

	// A platform super admin with a tenant context is scoped to that tenant.
	logs, _, err := q.ListAuditLogs(withTenantCtx(tenantBID), platformAdminActor(), 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].TenantID.String() != tenantBID {
		t.Fatalf("tenant mismatch: got %s, want %s", logs[0].TenantID, tenantBID)
	}
}

func TestQuery_ListAuditLogs_Pagination(t *testing.T) {
	repo := memory.NewRepository()
	first := seedLog(t, repo, tenantAID, "student.created.v1", "")
	time.Sleep(2 * time.Millisecond)
	second := seedLog(t, repo, tenantAID, "student.updated.v1", "")
	time.Sleep(2 * time.Millisecond)
	third := seedLog(t, repo, tenantAID, "student.deleted.v1", "")
	q := application.NewQuery(repo)

	page1, next, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead), 2, "")
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 logs on page 1, got %d", len(page1))
	}
	// Newest-first ordering.
	if page1[0].ID != third.ID || page1[1].ID != second.ID {
		t.Fatalf("page 1 order mismatch: got %s, %s", page1[0].ID, page1[1].ID)
	}
	if next == "" {
		t.Fatal("expected next cursor on page 1")
	}

	page2, next2, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead), 2, next)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 log on page 2, got %d", len(page2))
	}
	if page2[0].ID != first.ID {
		t.Fatalf("expected oldest log on page 2, got %s, want %s", page2[0].ID, first.ID)
	}
	if next2 != "" {
		t.Fatalf("expected no next cursor on last page, got %q", next2)
	}
}

func TestQuery_ListAuditLogs_InvalidCursor(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())

	_, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead), 25, "not-a-uuid")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
