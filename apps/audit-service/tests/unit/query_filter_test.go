package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/memory"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/audit-service/internal/domain"
	"github.com/google/uuid"
)

func seedFullLog(t *testing.T, repo *memory.Repository, tenantID, eventType, sourceService, actorID string, ts time.Time) *domain.AuditLog {
	t.Helper()
	log, err := domain.NewAuditLogBuilder().
		TenantID(tenantID).
		EventID(uuid.NewString()).
		EventType(eventType).
		SourceService(sourceService).
		Timestamp(ts).
		ReceivedAt(ts).
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

func TestQuery_ListAuditLogs_FilterByEventType(t *testing.T) {
	repo := memory.NewRepository()
	now := time.Now().UTC()
	want := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", now)
	seedFullLog(t, repo, tenantAID, "invoice.created.v1", "billing-service", "", now)
	q := application.NewQuery(repo)

	logs, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead),
		domain.ListFilter{EventType: "student.created.v1"}, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != want.ID {
		t.Fatalf("expected only the student.created.v1 log, got %d logs", len(logs))
	}
}

func TestQuery_ListAuditLogs_FilterByActorAndSource(t *testing.T) {
	repo := memory.NewRepository()
	now := time.Now().UTC()
	actorID := uuid.NewString()
	want := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", actorID, now)
	seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", uuid.NewString(), now)
	seedFullLog(t, repo, tenantAID, "student.created.v1", "billing-service", actorID, now)
	q := application.NewQuery(repo)

	logs, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead),
		domain.ListFilter{ActorID: actorID, SourceService: "student-service"}, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != want.ID {
		t.Fatalf("expected only the matching actor+source log, got %d logs", len(logs))
	}
}

func TestQuery_ListAuditLogs_FilterByTimeRange(t *testing.T) {
	repo := memory.NewRepository()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	old := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", base.Add(-48*time.Hour))
	mid := seedFullLog(t, repo, tenantAID, "student.updated.v1", "student-service", "", base)
	recent := seedFullLog(t, repo, tenantAID, "student.deleted.v1", "student-service", "", base.Add(48*time.Hour))
	q := application.NewQuery(repo)
	actor := tenantActor(tenantAID, application.PermRead)

	// Inclusive bounds: from == mid and to == recent keep both edges.
	logs, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), actor,
		domain.ListFilter{From: &base, To: &recent.Timestamp}, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 2 || logs[0].ID != recent.ID || logs[1].ID != mid.ID {
		t.Fatalf("expected mid and recent logs, got %d", len(logs))
	}

	// An upper bound alone excludes later records.
	logs, _, err = q.ListAuditLogs(withTenantCtx(tenantAID), actor,
		domain.ListFilter{To: &base}, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 2 || logs[0].ID != mid.ID || logs[1].ID != old.ID {
		t.Fatalf("expected old and mid logs, got %d", len(logs))
	}
}

func TestQuery_ListAuditLogs_FilterFromAfterTo(t *testing.T) {
	q := application.NewQuery(memory.NewRepository())
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	later := base.Add(time.Hour)

	_, _, err := q.ListAuditLogs(withTenantCtx(tenantAID), tenantActor(tenantAID, application.PermRead),
		domain.ListFilter{From: &later, To: &base}, 25, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestQuery_ListAuditLogs_FilterCrossTenant(t *testing.T) {
	repo := memory.NewRepository()
	now := time.Now().UTC()
	want := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", now)
	seedFullLog(t, repo, tenantBID, "invoice.created.v1", "billing-service", "", now)
	q := application.NewQuery(repo)

	// A platform super admin can filter across tenants by source service.
	logs, _, err := q.ListAuditLogs(context.Background(), platformAdminActor(),
		domain.ListFilter{SourceService: "student-service"}, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != want.ID {
		t.Fatalf("expected only the student-service log, got %d logs", len(logs))
	}
}

func TestQuery_ListAuditLogs_FilterPaginationPreserved(t *testing.T) {
	repo := memory.NewRepository()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	first := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", base)
	time.Sleep(2 * time.Millisecond)
	second := seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", base.Add(time.Hour))
	// A record that never matches the filter must not affect pagination.
	seedFullLog(t, repo, tenantAID, "invoice.created.v1", "billing-service", "", base.Add(2*time.Hour))
	q := application.NewQuery(repo)
	actor := tenantActor(tenantAID, application.PermRead)
	filter := domain.ListFilter{EventType: "student.created.v1"}

	page1, next, err := q.ListAuditLogs(withTenantCtx(tenantAID), actor, filter, 1, "")
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 1 || page1[0].ID != second.ID || next == "" {
		t.Fatalf("page 1 mismatch: got %d logs, next=%q", len(page1), next)
	}

	page2, next2, err := q.ListAuditLogs(withTenantCtx(tenantAID), actor, filter, 1, next)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != first.ID || next2 == "" {
		t.Fatalf("page 2 mismatch: got %d logs, next=%q", len(page2), next2)
	}
}

// Regression: with the gateway route TenantOptional, a tenantless request from
// a NON-platform actor must fail closed — never fall through to the
// cross-tenant ListAll path.
func TestQuery_ListAuditLogs_TenantlessNonPlatformFailsClosed(t *testing.T) {
	repo := memory.NewRepository()
	seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", "", time.Now().UTC())
	seedFullLog(t, repo, tenantBID, "invoice.created.v1", "billing-service", "", time.Now().UTC())
	q := application.NewQuery(repo)

	// Authenticated tenant actor with audit.read, but no tenant context.
	_, _, err := q.ListAuditLogs(context.Background(), tenantActor(tenantAID, application.PermRead),
		domain.ListFilter{}, 25, "")
	if !errors.Is(err, domain.ErrMissingTenant) {
		t.Fatalf("expected ErrMissingTenant, got %v", err)
	}

	// A platform-role claim without the platform admin flag is still rejected.
	impostor := tenantActor(tenantAID, application.PermRead)
	impostor.Role = "platform_super_admin"
	impostor.TenantID = ""
	_, _, err = q.ListAuditLogs(context.Background(), impostor, domain.ListFilter{}, 25, "")
	if !errors.Is(err, domain.ErrMissingTenant) {
		t.Fatalf("expected ErrMissingTenant for unverified platform role, got %v", err)
	}
}
