// Package adapters_test runs one contract against every website repository driver.
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
	mongoadapter "github.com/auraedu/website-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/google/uuid"
)

type websiteRepo interface {
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

// newPage builds the aggregate directly rather than through the domain
// constructor, which is what makes this a storage test.
func newPage(tenantID, slug, title string) *domain.Page {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Page{
		ID: uuid.NewString(), TenantID: tenantID, Slug: domain.NormalizeSlug(slug),
		Title: title, Status: string(domain.PageStatusDraft), Layout: string(domain.PageLayoutDefault),
		CreatedAt: now, UpdatedAt: now,
	}
}

// newSection builds a section aggregate directly.
func newSection(tenantID, pageID string, sectionType domain.SectionType) *domain.Section {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Section{
		ID: uuid.NewString(), TenantID: tenantID, PageID: pageID, Type: string(sectionType),
		Content: domain.Content{}, SortOrder: 0, Status: string(domain.SectionStatusDraft),
		CreatedAt: now, UpdatedAt: now,
	}
}

func runWebsiteContract(t *testing.T, repo websiteRepo) {
	t.Run("create then read back page within the tenant", func(t *testing.T) {
		p := newPage("upshs", "home", "Home")
		if err := repo.CreatePage(tenantCtx("upshs"), "upshs", p); err != nil {
			t.Fatalf("create page: %v", err)
		}
		got, err := repo.GetPageByID(tenantCtx("upshs"), "upshs", p.ID)
		if err != nil {
			t.Fatalf("get page: %v", err)
		}
		if got.Title != "Home" || got.TenantID != "upshs" {
			t.Fatalf("round trip changed the page: %+v", got)
		}
	})

	t.Run("another tenant cannot read the page", func(t *testing.T) {
		p := newPage("upshs", "about", "About")
		if err := repo.CreatePage(tenantCtx("upshs"), "upshs", p); err != nil {
			t.Fatalf("create page: %v", err)
		}
		if _, err := repo.GetPageByID(tenantCtx("aboom"), "aboom", p.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
	})

	t.Run("another tenant cannot update or delete the page", func(t *testing.T) {
		p := newPage("upshs", "services", "Services")
		if err := repo.CreatePage(tenantCtx("upshs"), "upshs", p); err != nil {
			t.Fatalf("create page: %v", err)
		}
		clone := *p
		clone.Title = "Hijacked"
		if err := repo.UpdatePage(tenantCtx("aboom"), "aboom", &clone); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant update returned %v; want ErrNotFound", err)
		}
		if err := repo.DeletePage(tenantCtx("aboom"), "aboom", p.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
		got, err := repo.GetPageByID(tenantCtx("upshs"), "upshs", p.ID)
		if err != nil || got.Title == "Hijacked" {
			t.Fatalf("the owner's page was altered across tenants: %+v (err=%v)", got, err)
		}
	})

	t.Run("delete removes the page from reads", func(t *testing.T) {
		p := newPage("upshs", "deleted", "Deleted")
		if err := repo.CreatePage(tenantCtx("upshs"), "upshs", p); err != nil {
			t.Fatalf("create page: %v", err)
		}
		if err := repo.DeletePage(tenantCtx("upshs"), "upshs", p.ID); err != nil {
			t.Fatalf("delete page: %v", err)
		}
		if _, err := repo.GetPageByID(tenantCtx("upshs"), "upshs", p.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("a deleted page is still readable: %v", err)
		}
	})

	t.Run("list pages is tenant scoped and pages by cursor", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		slugs := []string{"list-1", "list-2", "list-3"}
		for _, slug := range slugs {
			if err := repo.CreatePage(ctx, tenant, newPage(tenant, slug, "Title for "+slug)); err != nil {
				t.Fatalf("create page: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			page, next, err := repo.ListPages(ctx, tenant, 2, cursor, ports.PageFilter{})
			if err != nil {
				t.Fatalf("list pages: %v", err)
			}
			for _, p := range page {
				if p.TenantID != tenant {
					t.Fatalf("list leaked a page owned by %q", p.TenantID)
				}
				if seen[p.ID] {
					t.Fatalf("the cursor repeated page %s", p.ID)
				}
				seen[p.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < len(slugs) {
			t.Fatalf("paging saw %d pages; want at least the %d just created", len(seen), len(slugs))
		}
	})

	t.Run("get page by slug", func(t *testing.T) {
		p := newPage("upshs", "slug-test", "Slug Test")
		if err := repo.CreatePage(tenantCtx("upshs"), "upshs", p); err != nil {
			t.Fatalf("create page: %v", err)
		}
		got, err := repo.GetPageBySlug(tenantCtx("upshs"), "upshs", "slug-test")
		if err != nil {
			t.Fatalf("get page by slug: %v", err)
		}
		if got.ID != p.ID {
			t.Fatalf("got wrong page: %s != %s", got.ID, p.ID)
		}
	})

	t.Run("a lifecycle commit queues exactly one claimable event", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		drain(t, repo)

		p := newPage(tenant, "lc-page", "LC Page")
		payload := ports.PageEventData(p, map[string]any{"mutation": "create"})
		events := []ports.LifecycleEvent{{EventType: "page.created.v1", Payload: payload}}
		if err := repo.CommitWebsiteLifecycle(ctx, tenant, ports.WebsiteMutationPageCreate, p, nil, events); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}

		if _, err := repo.GetPageByID(ctx, tenant, p.ID); err != nil {
			t.Fatalf("lifecycle commit did not persist the page: %v", err)
		}

		claimed, err := repo.ClaimPendingWebsiteEvents(ctx, 100)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 {
			t.Fatalf("claimed %d events; want the one just queued", len(claimed))
		}
		if claimed[0].EventType != "page.created.v1" {
			t.Fatalf("unexpected event type %q", claimed[0].EventType)
		}

		if err := repo.MarkWebsiteEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		again, err := repo.ClaimPendingWebsiteEvents(ctx, 100)
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

		p := newPage(tenant, "fl-page", "FL Page")
		payload := ports.PageEventData(p, nil)
		events := []ports.LifecycleEvent{{EventType: "page.created.v1", Payload: payload}}
		if err := repo.CommitWebsiteLifecycle(ctx, tenant, ports.WebsiteMutationPageCreate, p, nil, events); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingWebsiteEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d events, err=%v", len(claimed), err)
		}
		if err := repo.MarkWebsiteEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}

		if err := repo.MarkWebsiteEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})

	t.Run("sections are tenant scoped", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		p := newPage(tenant, "sec-page", "Sections Page")
		if err := repo.CreatePage(ctx, tenant, p); err != nil {
			t.Fatalf("create page: %v", err)
		}

		s := newSection(tenant, p.ID, domain.SectionTypeHero)
		if err := repo.CreateSection(ctx, tenant, s); err != nil {
			t.Fatalf("create section: %v", err)
		}

		got, err := repo.GetSectionByID(ctx, tenant, s.ID)
		if err != nil {
			t.Fatalf("get section: %v", err)
		}
		if got.ID != s.ID {
			t.Fatalf("round trip changed the section: %+v", got)
		}

		other, err := repo.GetSectionByID(tenantCtx("aboom"), "aboom", s.ID)
		if err == nil && other != nil {
			t.Fatalf("section leaked to another tenant")
		}
	})

	t.Run("list sections pages by cursor", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		p := newPage(tenant, "list-sec-page", "List Sections Page")
		if err := repo.CreatePage(ctx, tenant, p); err != nil {
			t.Fatalf("create page: %v", err)
		}

		for i := 0; i < 3; i++ {
			s := newSection(tenant, p.ID, domain.SectionTypeText)
			s.SortOrder = i
			if err := repo.CreateSection(ctx, tenant, s); err != nil {
				t.Fatalf("create section: %v", err)
			}
		}

		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			sections, next, err := repo.ListSections(ctx, tenant, p.ID, 2, cursor, ports.SectionFilter{})
			if err != nil {
				t.Fatalf("list sections: %v", err)
			}
			for _, s := range sections {
				if s.TenantID != tenant {
					t.Fatalf("list leaked a section owned by %q", s.TenantID)
				}
				if seen[s.ID] {
					t.Fatalf("the cursor repeated section %s", s.ID)
				}
				seen[s.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) < 3 {
			t.Fatalf("paging saw %d sections; want at least 3 just created", len(seen))
		}
	})

	t.Run("delete sections by page", func(t *testing.T) {
		tenant := "upshs"
		ctx := tenantCtx(tenant)
		p := newPage(tenant, "del-sec-page", "Delete Sections Page")
		if err := repo.CreatePage(ctx, tenant, p); err != nil {
			t.Fatalf("create page: %v", err)
		}

		s1 := newSection(tenant, p.ID, domain.SectionTypeText)
		s2 := newSection(tenant, p.ID, domain.SectionTypeGallery)
		if err := repo.CreateSection(ctx, tenant, s1); err != nil {
			t.Fatalf("create section 1: %v", err)
		}
		if err := repo.CreateSection(ctx, tenant, s2); err != nil {
			t.Fatalf("create section 2: %v", err)
		}

		if err := repo.DeleteSectionsByPage(ctx, tenant, p.ID); err != nil {
			t.Fatalf("delete sections by page: %v", err)
		}

		if _, err := repo.GetSectionByID(ctx, tenant, s1.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("section 1 should be deleted: %v", err)
		}
		if _, err := repo.GetSectionByID(ctx, tenant, s2.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("section 2 should be deleted: %v", err)
		}
	})
}

// drain empties the outbox so a subtest counts only the events it queued.
func drain(t *testing.T, repo websiteRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingWebsiteEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkWebsiteEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresWebsiteRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runWebsiteContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoWebsiteRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runWebsiteContract(t, mongoadapter.NewRepository(mg.Store))
}
