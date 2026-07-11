package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/website-service/internal/adapters/events"
	"github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/application"
	"github.com/auraedu/website-service/internal/ports"
)

func newService(t *testing.T, enabled bool) (*application.Service, *events.RecordingPublisher) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	pub := events.NewRecordingPublisher()
	gate := flags.NewStaticSnapshot()
	gate.Set(tenantA, application.FeaturePublicWebsite, enabled)
	svc := application.NewService(repo,
		application.WithPublisher(pub),
		application.WithFeatureGate(gate),
	)
	return svc, pub
}

func actorWithPerm() auth.Actor {
	return auth.Actor{
		UserID:        "user-1",
		TenantID:      tenantA,
		Role:          "school_admin",
		Permissions:   []string{application.PermManage},
		PlatformAdmin: false,
	}
}

func TestService_FeatureFlagGatesCreate(t *testing.T) {
	svc, _ := newService(t, false)
	ctx := withTenant(context.Background(), tenantA)

	_, err := svc.CreatePage(ctx, actorWithPerm(), application.CreatePageRequest{
		Slug:  "home",
		Title: "Home",
	})
	if err == nil {
		t.Fatal("expected error when public_website feature is disabled")
	}
}

func TestService_CreatePagePublishesEvent(t *testing.T) {
	svc, pub := newService(t, true)
	ctx := withTenant(context.Background(), tenantA)

	page, err := svc.CreatePage(ctx, actorWithPerm(), application.CreatePageRequest{
		Slug:  "home",
		Title: "Home",
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	found := false
	for _, e := range pub.Events {
		if e.Type == "website.page_created.v1" && e.SubjectID == page.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected page_created event, got %v", pub.Events)
	}
}

func TestService_PublishPageFiresPagePublishedEvent(t *testing.T) {
	svc, pub := newService(t, true)
	ctx := withTenant(context.Background(), tenantA)

	page, err := svc.CreatePage(ctx, actorWithPerm(), application.CreatePageRequest{
		Slug:  "home",
		Title: "Home",
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	pub.Events = nil

	status := "published"
	_, err = svc.UpdatePage(ctx, actorWithPerm(), page.ID, application.UpdatePageRequest{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("update page: %v", err)
	}

	found := false
	for _, e := range pub.Events {
		if e.Type == "website.page_published.v1" && e.SubjectID == page.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected page_published event, got %v", pub.Events)
	}
}

func TestService_RBACGatesRead(t *testing.T) {
	svc, _ := newService(t, true)
	ctx := withTenant(context.Background(), tenantA)

	actor := auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: []string{"other.read"}}
	if _, _, err := svc.ListPages(ctx, actor, 10, "", ports.PageFilter{}); err == nil {
		t.Fatal("expected error when actor lacks website.read permission")
	}
}
