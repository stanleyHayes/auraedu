package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	announceTenantA = "11111111-1111-1111-1111-111111111111"
	announceTenantB = "22222222-2222-2222-2222-222222222222"
)

// newAnnouncementService builds a service with in-memory fakes and the
// notifications + announcements flags enabled for both tenants.
func newAnnouncementService() (*application.Service, *fakeMessageRepo, *fakeAnnouncementRepo) {
	messages := newFakeMessageRepo()
	announcements := newFakeAnnouncementRepo()
	gates := flags.NewStaticSnapshot()
	for _, tenantID := range []string{announceTenantA, announceTenantB} {
		gates.Set(tenantID, application.FeatureNotifications, true)
		gates.Set(tenantID, application.FeatureAnnouncements, true)
	}
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
		application.WithAnnouncementRepository(announcements),
	)
	return svc, messages, announcements
}

func announcementCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func announceActor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func TestNewAnnouncement_ValidatesInput(t *testing.T) {
	cases := []struct {
		name     string
		tenantID string
		title    string
		body     string
		audience string
		wantErr  bool
	}{
		{"valid", "t-1", "Title", "Body", "students", false},
		{"default audience", "t-1", "Title", "Body", "", false},
		{"missing tenant", "", "Title", "Body", "all", true},
		{"missing title", "t-1", "  ", "Body", "all", true},
		{"missing body", "t-1", "Title", " ", "all", true},
		{"invalid audience", "t-1", "Title", "Body", "aliens", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := domain.NewAnnouncement(tc.tenantID, tc.title, tc.body, tc.audience)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if a.Audience == "" {
				t.Fatal("expected audience defaulted")
			}
		})
	}

	a, err := domain.NewAnnouncement("t-1", "Title", "Body", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Audience != string(domain.AudienceAll) {
		t.Fatalf("expected default audience all, got %q", a.Audience)
	}
}

func TestCreateAnnouncement_PublishesInAppMessage(t *testing.T) {
	svc, messages, announcements := newAnnouncementService()
	ctx := announcementCtx(announceTenantA)
	actor := announceActor(announceTenantA, application.PermManage)

	a, err := svc.CreateAnnouncement(ctx, actor, application.CreateAnnouncementRequest{
		Title:    "School closed Friday",
		Body:     "Staff development day.",
		Audience: "guardians",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	stored, err := announcements.GetByID(ctx, announceTenantA, a.ID)
	if err != nil {
		t.Fatalf("announcement not persisted: %v", err)
	}
	if stored.Title != "School closed Friday" || stored.Audience != "guardians" {
		t.Fatalf("announcement mismatch: %+v", stored)
	}

	// Creating an announcement publishes it to the tenant in-app inbox via the
	// standard message machinery, delivered through the MockNotifier.
	got := messages.all(announceTenantA)
	if len(got) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(got))
	}
	m := got[0]
	if m.Channel != string(domain.ChannelInApp) {
		t.Fatalf("expected in_app channel, got %q", m.Channel)
	}
	if m.RecipientID != announceTenantA {
		t.Fatalf("expected tenant inbox recipient, got %q", m.RecipientID)
	}
	if m.Subject != a.Title || m.Body != a.Body {
		t.Fatalf("message content mismatch: %+v", m)
	}
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", m.Status)
	}
	if m.Metadata["announcement_id"] != a.ID {
		t.Fatalf("expected announcement_id metadata, got %v", m.Metadata)
	}
}

func TestAnnouncementCRUD_FlagGating(t *testing.T) {
	messages := newFakeMessageRepo()
	announcements := newFakeAnnouncementRepo()
	gates := flags.NewStaticSnapshot()
	gates.Set(announceTenantA, application.FeatureNotifications, true)
	// announcements flag deliberately left off.
	svc := application.NewService(messages, nil, newFakeSubscriptionRepo(),
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
		application.WithAnnouncementRepository(announcements),
	)

	ctx := announcementCtx(announceTenantA)
	manage := announceActor(announceTenantA, application.PermManage)
	read := announceActor(announceTenantA, application.PermRead)

	if _, err := svc.CreateAnnouncement(ctx, manage, application.CreateAnnouncementRequest{Title: "T", Body: "B"}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled on create, got %v", err)
	}
	if _, _, err := svc.ListAnnouncements(ctx, read, ports.AnnouncementFilter{}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled on list, got %v", err)
	}
	if _, err := svc.GetAnnouncement(ctx, read, "any"); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled on get, got %v", err)
	}
	if err := svc.DeleteAnnouncement(ctx, manage, "any"); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled on delete, got %v", err)
	}
}

func TestAnnouncementCRUD_RequiresPermissions(t *testing.T) {
	svc, _, _ := newAnnouncementService()
	ctx := announcementCtx(announceTenantA)

	// notifications.read only: may list/get but not create/delete.
	reader := announceActor(announceTenantA, application.PermRead)
	if _, err := svc.CreateAnnouncement(ctx, reader, application.CreateAnnouncementRequest{Title: "T", Body: "B"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden on create without manage, got %v", err)
	}

	// No permissions at all: may not even list.
	noPerms := announceActor(announceTenantA)
	if _, _, err := svc.ListAnnouncements(ctx, noPerms, ports.AnnouncementFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden on list without read, got %v", err)
	}

	// Unauthenticated actor.
	if _, _, err := svc.ListAnnouncements(ctx, auth.Actor{}, ports.AnnouncementFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for anonymous actor, got %v", err)
	}
}

func TestAnnouncementCRUD_TenantScoping(t *testing.T) {
	svc, _, _ := newAnnouncementService()
	ctxA := announcementCtx(announceTenantA)
	manageA := announceActor(announceTenantA, application.PermManage, application.PermRead)

	a, err := svc.CreateAnnouncement(ctxA, manageA, application.CreateAnnouncementRequest{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Tenant B cannot read, list or delete tenant A's announcement.
	ctxB := announcementCtx(announceTenantB)
	manageB := announceActor(announceTenantB, application.PermManage, application.PermRead)

	if _, err := svc.GetAnnouncement(ctxB, manageB, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found across tenants, got %v", err)
	}
	list, _, err := svc.ListAnnouncements(ctxB, manageB, ports.AnnouncementFilter{})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 announcements, got %d", len(list))
	}
	if err := svc.DeleteAnnouncement(ctxB, manageB, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found deleting across tenants, got %v", err)
	}

	// An actor from tenant B cannot use tenant A's context at all.
	if _, err := svc.GetAnnouncement(ctxA, manageB, a.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden for cross-tenant actor, got %v", err)
	}

	// Tenant A can read and delete its own announcement.
	if _, err := svc.GetAnnouncement(ctxA, manageA, a.ID); err != nil {
		t.Fatalf("get own announcement: %v", err)
	}
	if err := svc.DeleteAnnouncement(ctxA, manageA, a.ID); err != nil {
		t.Fatalf("delete own announcement: %v", err)
	}
	if _, err := svc.GetAnnouncement(ctxA, manageA, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestAnnouncementReadsAreScopedToRoleAudience(t *testing.T) {
	svc, _, _ := newAnnouncementService()
	ctx := announcementCtx(announceTenantA)
	manager := announceActor(announceTenantA, application.PermManage, application.PermRead)
	all, err := svc.CreateAnnouncement(ctx, manager, application.CreateAnnouncementRequest{Title: "Everyone", Body: "All", Audience: "all"})
	if err != nil {
		t.Fatalf("create all-audience announcement: %v", err)
	}
	guardians, err := svc.CreateAnnouncement(ctx, manager, application.CreateAnnouncementRequest{Title: "Parents", Body: "Guardian only", Audience: "guardians"})
	if err != nil {
		t.Fatalf("create guardian announcement: %v", err)
	}
	staff, err := svc.CreateAnnouncement(ctx, manager, application.CreateAnnouncementRequest{Title: "Staff", Body: "Staff only", Audience: "staff"})
	if err != nil {
		t.Fatalf("create staff announcement: %v", err)
	}
	parent := auth.Actor{UserID: "parent-1", TenantID: announceTenantA, Role: "parent", Permissions: []string{application.PermRead}}
	list, _, err := svc.ListAnnouncements(ctx, parent, ports.AnnouncementFilter{Limit: 20})
	if err != nil || len(list) != 2 {
		t.Fatalf("parent audience list=%+v err=%v", list, err)
	}
	if _, err = svc.GetAnnouncement(ctx, parent, staff.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("parent read staff announcement=%v", err)
	}
	if _, err = svc.GetAnnouncement(ctx, parent, all.ID); err != nil {
		t.Fatalf("parent read all announcement=%v", err)
	}
	if _, err = svc.GetAnnouncement(ctx, parent, guardians.ID); err != nil {
		t.Fatalf("parent read guardian announcement=%v", err)
	}
}
