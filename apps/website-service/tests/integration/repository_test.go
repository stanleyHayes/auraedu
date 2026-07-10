package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/jackc/pgx/v5/pgconn"
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

func mustCreatePage(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, slug, title string) *domain.Page {
	t.Helper()
	page, err := domain.NewPage(tenantID, slug, title)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	if err := repo.CreatePage(ctx, tenantID, page); err != nil {
		t.Fatalf("create page: %v", err)
	}
	return page
}

func mustCreateSection(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, pageID string, sectionType domain.SectionType, order int) *domain.Section {
	t.Helper()
	section, err := domain.NewSection(tenantID, pageID, sectionType, domain.Content{"title": "Section"}, order)
	if err != nil {
		t.Fatalf("new section: %v", err)
	}
	if err := repo.CreateSection(ctx, tenantID, section); err != nil {
		t.Fatalf("create section: %v", err)
	}
	return section
}

func TestRepository_CreateAndGetPage(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")

	got, err := repo.GetPageByID(ctx, tenantA, page.ID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if got.ID != page.ID || got.Slug != "home" {
		t.Fatalf("page mismatch: %+v", got)
	}
}

func TestRepository_GetPageBySlug(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "about", "About")

	got, err := repo.GetPageBySlug(ctx, tenantA, "about")
	if err != nil {
		t.Fatalf("get page by slug: %v", err)
	}
	if got.ID != page.ID {
		t.Fatalf("page mismatch: %+v", got)
	}
}

func TestRepository_ListPagesPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreatePage(t, ctx, repo, tenantA, "page-a", "Page A")
	page2 := mustCreatePage(t, ctx, repo, tenantA, "page-b", "Page B")

	page, next, err := repo.ListPages(ctx, tenantA, 1, "", ports.PageFilter{})
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2List, _, err := repo.ListPages(ctx, tenantA, 1, next, ports.PageFilter{})
	if err != nil {
		t.Fatalf("list pages cursor: %v", err)
	}
	if len(page2List) != 1 || page2List[0].ID != page2.ID {
		t.Fatalf("expected second page, got %+v", page2List)
	}
}

func TestRepository_ListPagesWithFilter(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreatePage(t, ctx, repo, tenantA, "landing-a", "Landing A")
	page := mustCreatePage(t, ctx, repo, tenantA, "landing-b", "Landing B")
	layout := string(domain.PageLayoutLanding)
	if _, err := page.ApplyUpdate(nil, nil, nil, nil, &layout); err != nil {
		t.Fatalf("apply layout update: %v", err)
	}
	if err := repo.UpdatePage(ctx, tenantA, page); err != nil {
		t.Fatalf("update page layout: %v", err)
	}

	filter := ports.PageFilter{Layout: &layout}
	list, _, err := repo.ListPages(ctx, tenantA, 10, "", filter)
	if err != nil {
		t.Fatalf("list pages with filter: %v", err)
	}
	if len(list) != 1 || list[0].ID != page.ID {
		t.Fatalf("expected 1 landing page, got %+v", list)
	}
}

func TestRepository_UpdatePage(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	title := "Welcome"
	if _, err := page.ApplyUpdate(nil, &title, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdatePage(ctx, tenantA, page); err != nil {
		t.Fatalf("update page: %v", err)
	}

	got, err := repo.GetPageByID(ctx, tenantA, page.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != title {
		t.Fatalf("title not updated: %q", got.Title)
	}
}

func TestRepository_DeletePage(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	if err := repo.DeletePage(ctx, tenantA, page.ID); err != nil {
		t.Fatalf("delete page: %v", err)
	}
	if _, err := repo.GetPageByID(ctx, tenantA, page.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_SlugUniquenessPerTenant(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreatePage(t, ctx, repo, tenantA, "home", "Home A")

	// Same slug in a different tenant should succeed.
	bCtx := withTenant(context.Background(), tenantB)
	mustCreatePage(t, bCtx, repo, tenantB, "home", "Home B")

	// Same slug in the same tenant should fail.
	page, err := domain.NewPage(tenantA, "home", "Home Again")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	err = repo.CreatePage(ctx, tenantA, page)
	if err == nil {
		t.Fatal("expected error for duplicate slug in same tenant")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected unique violation, got %v", err)
	}
}

func TestRepository_TenantIsolationPages(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	page := mustCreatePage(t, aCtx, repo, tenantA, "home", "Home")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetPageByID(bCtx, tenantB, page.ID); err == nil {
		t.Fatal("tenant B should not see tenant A page")
	}

	list, _, err := repo.ListPages(bCtx, tenantB, 10, "", ports.PageFilter{})
	if err != nil {
		t.Fatalf("list tenant B pages: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 pages, got %d", len(list))
	}
}

func TestRepository_CreateAndGetSection(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	section := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)

	got, err := repo.GetSectionByID(ctx, tenantA, section.ID)
	if err != nil {
		t.Fatalf("get section: %v", err)
	}
	if got.ID != section.ID || got.PageID != page.ID {
		t.Fatalf("section mismatch: %+v", got)
	}
}

func TestRepository_ListSections(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)
	section2 := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeText, 1)

	filter := ports.SectionFilter{Type: ptr(string(domain.SectionTypeText))}
	list, _, err := repo.ListSections(ctx, tenantA, page.ID, 10, "", filter)
	if err != nil {
		t.Fatalf("list sections: %v", err)
	}
	if len(list) != 1 || list[0].ID != section2.ID {
		t.Fatalf("expected 1 text section, got %+v", list)
	}
}

func TestRepository_ListSectionsPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)
	section2 := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeText, 1)

	list, next, err := repo.ListSections(ctx, tenantA, page.ID, 1, "", ports.SectionFilter{})
	if err != nil {
		t.Fatalf("list sections: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	list2, _, err := repo.ListSections(ctx, tenantA, page.ID, 1, next, ports.SectionFilter{})
	if err != nil {
		t.Fatalf("list sections cursor: %v", err)
	}
	if len(list2) != 1 || list2[0].ID != section2.ID {
		t.Fatalf("expected second section, got %+v", list2)
	}
}

func TestRepository_UpdateSection(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	section := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)

	sectionType := domain.SectionTypeText
	content := domain.Content{"body": "Updated"}
	if _, err := section.ApplyUpdate(&sectionType, &content, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateSection(ctx, tenantA, section); err != nil {
		t.Fatalf("update section: %v", err)
	}

	got, err := repo.GetSectionByID(ctx, tenantA, section.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Type != string(sectionType) {
		t.Fatalf("type not updated: got %q", got.Type)
	}
}

func TestRepository_DeleteSection(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	section := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)

	if err := repo.DeleteSection(ctx, tenantA, section.ID); err != nil {
		t.Fatalf("delete section: %v", err)
	}
	if _, err := repo.GetSectionByID(ctx, tenantA, section.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_DeletePageCascadesSections(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	page := mustCreatePage(t, ctx, repo, tenantA, "home", "Home")
	section := mustCreateSection(t, ctx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)

	if err := repo.DeletePage(ctx, tenantA, page.ID); err != nil {
		t.Fatalf("delete page: %v", err)
	}
	if _, err := repo.GetSectionByID(ctx, tenantA, section.ID); err == nil {
		t.Fatal("expected section to be deleted with page")
	}
}

func TestRepository_TenantIsolationSections(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	page := mustCreatePage(t, aCtx, repo, tenantA, "home", "Home")
	section := mustCreateSection(t, aCtx, repo, tenantA, page.ID, domain.SectionTypeHero, 0)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetSectionByID(bCtx, tenantB, section.ID); err == nil {
		t.Fatal("tenant B should not see tenant A section")
	}

	list, _, err := repo.ListSections(bCtx, tenantB, page.ID, 10, "", ports.SectionFilter{})
	if err != nil {
		t.Fatalf("list tenant B sections: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 sections, got %d", len(list))
	}
}

func ptr(s string) *string { return &s }
