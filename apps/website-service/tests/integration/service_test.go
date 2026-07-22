package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/application"
	"github.com/auraedu/website-service/internal/ports"
)

func newService(t *testing.T, enabled bool) (*application.Service, *postgres.Repository) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	gate := flags.NewStaticSnapshot()
	gate.Set(tenantA, application.FeaturePublicWebsite, enabled)
	svc := application.NewService(repo,
		application.WithFeatureGate(gate),
	)
	return svc, repo
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
	svc, repo := newService(t, true)
	ctx := withTenant(context.Background(), tenantA)

	page, err := svc.CreatePage(ctx, actorWithPerm(), application.CreatePageRequest{
		Slug:  "home",
		Title: "Home",
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if page.ID == "" {
		t.Fatal("created page has no id")
	}

	queued, err := repo.ClaimPendingWebsiteEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range queued {
		if e.EventType == "website.page_created.v1" {
			found = true
			if err := repo.MarkWebsiteEventPublished(context.Background(), e.ID); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected durable page_created event, got %v", queued)
	}
}

func TestService_PublishPageFiresPagePublishedEvent(t *testing.T) {
	svc, repo := newService(t, true)
	ctx := withTenant(context.Background(), tenantA)

	page, err := svc.CreatePage(ctx, actorWithPerm(), application.CreatePageRequest{
		Slug:  "home",
		Title: "Home",
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	created, err := repo.ClaimPendingWebsiteEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range created {
		if err := repo.MarkWebsiteEventPublished(context.Background(), event.ID); err != nil {
			t.Fatal(err)
		}
	}

	status := "published"
	_, err = svc.UpdatePage(ctx, actorWithPerm(), page.ID, application.UpdatePageRequest{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("update page: %v", err)
	}

	queued, err := repo.ClaimPendingWebsiteEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range queued {
		if e.EventType == "website.page_published.v1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected durable page_published event, got %v", queued)
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
