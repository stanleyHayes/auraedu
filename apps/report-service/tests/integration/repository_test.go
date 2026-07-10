package integration

import (
	"context"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/report-service/internal/adapters/pdf"
	"github.com/auraedu/report-service/internal/adapters/postgres"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"
const ay1 = "cccccccc-cccc-cccc-cccc-cccccccccccc"
const ay2 = "dddddddd-dddd-dddd-dddd-dddddddddddd"
const studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
const studentB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
const template1 = "77777777-7777-7777-7777-777777777777"
const template2 = "88888888-8888-8888-8888-888888888888"

func newRepo(t *testing.T) (ports.Repository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateTemplate(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, name, academicYearID string) *domain.ReportTemplate {
	t.Helper()
	tmpl, err := domain.NewReportTemplate(tenantID, name, academicYearID, "# "+name)
	if err != nil {
		t.Fatalf("new report template: %v", err)
	}
	if err := repo.CreateReportTemplate(ctx, tenantID, tmpl); err != nil {
		t.Fatalf("create report template: %v", err)
	}
	return tmpl
}

func mustCreateCard(t *testing.T, ctx context.Context, repo ports.Repository, tenantID, studentID, academicYearID, templateID string) *domain.ReportCard {
	t.Helper()
	card, err := domain.NewReportCard(tenantID, studentID, academicYearID, templateID)
	if err != nil {
		t.Fatalf("new report card: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantID, card); err != nil {
		t.Fatalf("create report card: %v", err)
	}
	return card
}

func TestRepository_CreateAndGetReportTemplate(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)

	got, err := repo.GetReportTemplateByID(ctx, tenantA, tmpl.ID)
	if err != nil {
		t.Fatalf("get report template: %v", err)
	}
	if got.ID != tmpl.ID || got.Name != "Midterm" {
		t.Fatalf("report template mismatch: %+v", got)
	}
}

func TestRepository_ListReportTemplatePagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateTemplate(t, ctx, repo, tenantA, "Q1", ay1)
	t2 := mustCreateTemplate(t, ctx, repo, tenantA, "Q2", ay1)

	page, next, err := repo.ListReportTemplates(ctx, tenantA, ports.ReportTemplateListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.ListReportTemplates(ctx, tenantA, ports.ReportTemplateListFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != t2.ID {
		t.Fatalf("expected second template, got %+v", page2)
	}
}

func TestRepository_ListReportTemplateFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	mustCreateTemplate(t, ctx, repo, tenantA, "Q1", ay1)
	mustCreateTemplate(t, ctx, repo, tenantA, "Q2", ay2)

	cases := []struct {
		name   string
		filter ports.ReportTemplateListFilter
		want   int
	}{
		{"by academic_year_id", ports.ReportTemplateListFilter{Limit: 10, AcademicYearID: ay1}, 1},
		{"by status", ports.ReportTemplateListFilter{Limit: 10, Status: "draft"}, 2},
		{"combined", ports.ReportTemplateListFilter{Limit: 10, AcademicYearID: ay1, Status: "draft"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.ListReportTemplates(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d templates, got %d", tc.want, len(page))
			}
		})
	}
}

func TestRepository_UpdateReportTemplate(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	name := "Final"
	if _, err := tmpl.ApplyUpdate(&name, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateReportTemplate(ctx, tenantA, tmpl); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetReportTemplateByID(ctx, tenantA, tmpl.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != name {
		t.Fatalf("report template not updated: %+v", got)
	}
}

func TestRepository_DeleteReportTemplate(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	if err := repo.DeleteReportTemplate(ctx, tenantA, tmpl.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetReportTemplateByID(ctx, tenantA, tmpl.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_CreateAndGetReportCard(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	card := mustCreateCard(t, ctx, repo, tenantA, studentA, ay1, tmpl.ID)

	got, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get report card: %v", err)
	}
	if got.ID != card.ID || got.StudentID != studentA {
		t.Fatalf("report card mismatch: %+v", got)
	}
}

func TestRepository_ListReportCardsFilter(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	mustCreateCard(t, ctx, repo, tenantA, studentA, ay1, tmpl.ID)
	mustCreateCard(t, ctx, repo, tenantA, studentB, ay1, tmpl.ID)

	page, _, err := repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10, StudentID: studentA})
	if err != nil {
		t.Fatalf("list report cards: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 report card, got %d", len(page))
	}
}

func TestRepository_UpdateReportCard(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	card := mustCreateCard(t, ctx, repo, tenantA, studentA, ay1, tmpl.ID)
	student := studentB
	if _, err := card.ApplyUpdate(&student, nil, nil, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.UpdateReportCard(ctx, tenantA, card); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.StudentID != student {
		t.Fatalf("report card not updated: %+v", got)
	}
}

func TestRepository_DeleteReportCard(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	tmpl := mustCreateTemplate(t, ctx, repo, tenantA, "Midterm", ay1)
	card := mustCreateCard(t, ctx, repo, tenantA, studentA, ay1, tmpl.ID)
	if err := repo.DeleteReportCard(ctx, tenantA, card.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetReportCardByID(ctx, tenantA, card.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation_ReportTemplates(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	tmpl := mustCreateTemplate(t, aCtx, repo, tenantA, "Midterm", ay1)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetReportTemplateByID(bCtx, tenantB, tmpl.ID); err == nil {
		t.Fatal("tenant B should not see tenant A report template")
	}

	list, _, err := repo.ListReportTemplates(bCtx, tenantB, ports.ReportTemplateListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 report templates, got %d", len(list))
	}
}

func TestRepository_TenantIsolation_ReportCards(t *testing.T) {
	ctx := context.Background()
	repo, _ := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	tmpl := mustCreateTemplate(t, aCtx, repo, tenantA, "Midterm", ay1)
	card := mustCreateCard(t, aCtx, repo, tenantA, studentA, ay1, tmpl.ID)

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetReportCardByID(bCtx, tenantB, card.ID); err == nil {
		t.Fatal("tenant B should not see tenant A report card")
	}

	list, _, err := repo.ListReportCards(bCtx, tenantB, ports.ReportCardListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B report cards: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 report cards, got %d", len(list))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureReportCards, false)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermRead)

	_, err := svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name:           "Midterm",
		AcademicYearID: ay1,
		BodyTemplate:   "# Template",
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermRead)

	tmpl, err := svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name:           "Midterm",
		AcademicYearID: ay1,
		BodyTemplate:   "# Template",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tmpl.ID == "" {
		t.Fatal("expected report template id")
	}
}

func TestService_GenerateReportCardRoundtrip(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)

	svc := application.NewService(repo,
		application.WithFeatureGate(gates),
		application.WithPDFGenerator(pdf.NewGenerator()),
		application.WithReportOutputDir(t.TempDir()),
	)
	reader := actorWithPerms(tenantA, application.PermRead)
	publisher := actorWithPerms(tenantA, application.PermPublish)

	tmpl, err := svc.CreateReportTemplate(ctx, reader, application.CreateReportTemplateRequest{
		Name:           "Midterm",
		AcademicYearID: ay1,
		BodyTemplate:   "# Template",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	card, err := svc.CreateReportCard(ctx, reader, application.CreateReportCardRequest{
		StudentID:      studentA,
		AcademicYearID: ay1,
		TemplateID:     tmpl.ID,
	})
	if err != nil {
		t.Fatalf("create report card: %v", err)
	}

	published, err := svc.GenerateReportCard(ctx, publisher, card.ID)
	if err != nil {
		t.Fatalf("generate report card: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published status, got %q", published.Status)
	}
	if published.PDFPath == nil || *published.PDFPath == "" {
		t.Fatal("expected pdf_path to be set")
	}
}
