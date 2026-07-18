package integration

import (
	"context"
	"testing"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func newAnnouncementRepo(t *testing.T) ports.AnnouncementRepository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewAnnouncementRepository(tdb.DB)
}

func newProcessedEventRepo(t *testing.T) ports.ProcessedEventRepository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewProcessedEventRepository(tdb.DB)
}

func mustCreateAnnouncement(ctx context.Context, t *testing.T, repo ports.AnnouncementRepository, tenantID, title, audience string) *domain.Announcement {
	t.Helper()
	a, err := domain.NewAnnouncement(tenantID, title, "Body for "+title, audience)
	if err != nil {
		t.Fatalf("new announcement: %v", err)
	}
	if err := repo.Create(ctx, tenantID, a); err != nil {
		t.Fatalf("create announcement: %v", err)
	}
	return a
}

func TestAnnouncementRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newAnnouncementRepo(t)

	a := mustCreateAnnouncement(ctx, t, repo, tenantA, "Sports day", "students")

	got, err := repo.GetByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("get announcement: %v", err)
	}
	if got.ID != a.ID || got.Title != "Sports day" || got.Audience != "students" {
		t.Fatalf("announcement mismatch: %+v", got)
	}
}

func TestAnnouncementRepository_ListFiltersAndPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newAnnouncementRepo(t)

	mustCreateAnnouncement(ctx, t, repo, tenantA, "One", "all")
	second := mustCreateAnnouncement(ctx, t, repo, tenantA, "Two", "guardians")

	page, next, err := repo.List(ctx, tenantA, ports.AnnouncementFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 || next == "" {
		t.Fatalf("expected 1 item with cursor, got %d items cursor %q", len(page), next)
	}
	page2, _, err := repo.List(ctx, tenantA, ports.AnnouncementFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != second.ID {
		t.Fatalf("expected second announcement, got %+v", page2)
	}

	guardians, _, err := repo.List(ctx, tenantA, ports.AnnouncementFilter{Limit: 10, Audience: "guardians"})
	if err != nil {
		t.Fatalf("list by audience: %v", err)
	}
	if len(guardians) != 1 || guardians[0].ID != second.ID {
		t.Fatalf("expected only the guardians announcement, got %+v", guardians)
	}
}

func TestAnnouncementRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newAnnouncementRepo(t)

	a := mustCreateAnnouncement(ctx, t, repo, tenantA, "Goodbye", "all")
	if err := repo.Delete(ctx, tenantA, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, a.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestAnnouncementRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newAnnouncementRepo(t)

	aCtx := withTenant(ctx, tenantA)
	a := mustCreateAnnouncement(aCtx, t, repo, tenantA, "Tenant A news", "all")

	// Tenant B must not see tenant A's announcement — the repo scopes by tenant
	// and Postgres RLS enforces the same boundary.
	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, a.ID); err == nil {
		t.Fatal("tenant B should not see tenant A announcement")
	}
	list, _, err := repo.List(bCtx, tenantB, ports.AnnouncementFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 announcements, got %d", len(list))
	}
	if err := repo.Delete(bCtx, tenantB, a.ID); err != nil {
		t.Fatalf("cross-tenant delete should be a no-op, got %v", err)
	}
	if _, err := repo.GetByID(aCtx, tenantA, a.ID); err != nil {
		t.Fatalf("tenant A announcement must survive tenant B delete: %v", err)
	}
}

func TestProcessedEventRepository_ClaimAndRelease(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newProcessedEventRepo(t)

	claimed, err := repo.Claim(ctx, tenantA, "evt-1", "payment.received")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !claimed {
		t.Fatal("first claim should succeed")
	}

	dup, err := repo.Claim(ctx, tenantA, "evt-1", "payment.received")
	if err != nil {
		t.Fatalf("duplicate claim: %v", err)
	}
	if dup {
		t.Fatal("duplicate claim should report false")
	}

	// The same event id under another tenant is an independent claim.
	otherCtx := withTenant(context.Background(), tenantB)
	otherClaimed, err := repo.Claim(otherCtx, tenantB, "evt-1", "payment.received")
	if err != nil {
		t.Fatalf("tenant B claim: %v", err)
	}
	if !otherClaimed {
		t.Fatal("tenant B claim should succeed independently")
	}

	if err := repo.Release(ctx, tenantA, "evt-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	reclaimed, err := repo.Claim(ctx, tenantA, "evt-1", "payment.received")
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if !reclaimed {
		t.Fatal("claim after release should succeed")
	}
}

// TestService_CreateAnnouncement_EndToEnd drives the full create flow against
// Postgres: announcement row plus a delivered in-app inbox message.
func TestService_CreateAnnouncement_EndToEnd(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	mRepo := postgres.NewMessageRepository(tdb.DB)
	sRepo := postgres.NewSubscriptionRepository(tdb.DB)
	aRepo := postgres.NewAnnouncementRepository(tdb.DB)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureNotifications, true)
	gates.Set(tenantA, application.FeatureAnnouncements, true)

	svc := application.NewService(mRepo, nil, sRepo,
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
		application.WithAnnouncementRepository(aRepo),
	)
	actor := actorWithPerms(tenantA, application.PermManage, application.PermRead)

	a, err := svc.CreateAnnouncement(ctx, actor, application.CreateAnnouncementRequest{
		Title:    "Term starts Monday",
		Body:     "All students report by 8am.",
		Audience: "all",
	})
	if err != nil {
		t.Fatalf("create announcement: %v", err)
	}

	stored, err := aRepo.GetByID(ctx, tenantA, a.ID)
	if err != nil {
		t.Fatalf("announcement not persisted: %v", err)
	}
	if stored.Title != "Term starts Monday" {
		t.Fatalf("announcement mismatch: %+v", stored)
	}

	inbox, _, err := mRepo.List(ctx, tenantA, ports.MessageFilter{Limit: 10, Channel: string(domain.ChannelInApp)})
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}
	if len(inbox) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(inbox))
	}
	m := inbox[0]
	if m.RecipientID != tenantA || m.Subject != a.Title {
		t.Fatalf("inbox message mismatch: %+v", m)
	}
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent inbox message, got %q", m.Status)
	}
}
