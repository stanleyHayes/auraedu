package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/report-service/internal/adapters/pdf"
	"github.com/auraedu/report-service/internal/adapters/postgres"
	"github.com/auraedu/report-service/internal/adapters/storage"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestService_TranscriptUsesPublishedTenantEvidence(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	template := mustCreateTemplate(ctx, t, repo, "Transcript", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, template.ID)
	now := time.Now().UTC()
	card.Status = string(domain.ReportCardStatusPublished)
	card.GeneratedAt = &now
	if err := repo.UpdateReportCard(ctx, tenantA, card); err != nil {
		t.Fatal(err)
	}
	maximum := 100.0
	score, err := domain.NewScoreEntry(tenantA, card.ID, studentA, "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", "assessment-1", "event-1", 91, &maximum)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertScoreEntry(ctx, tenantA, score); err != nil {
		t.Fatal(err)
	}
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	reader := actorWithPerms(tenantA, application.PermRead)
	transcript, err := svc.GetTranscript(ctx, reader, studentA)
	if err != nil {
		t.Fatalf("get transcript: %v", err)
	}
	if len(transcript.Entries) != 1 || len(transcript.Entries[0].Scores) != 1 || transcript.Entries[0].Scores[0].Score != 91 {
		t.Fatalf("unexpected transcript: %+v", transcript)
	}
	foreign, err := repo.ListTranscriptReportCards(withTenant(context.Background(), tenantB), tenantB, studentA)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("cross-tenant transcript leaked: %+v err=%v", foreign, err)
	}
}

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"
const ay1 = "cccccccc-cccc-cccc-cccc-cccccccccccc"
const ay2 = "dddddddd-dddd-dddd-dddd-dddddddddddd"
const studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
const studentB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

func newRepo(t *testing.T) ports.Repository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateTemplate(ctx context.Context, t *testing.T, repo ports.Repository, name, academicYearID string) *domain.ReportTemplate {
	t.Helper()
	tmpl, err := domain.NewReportTemplate(tenantA, name, academicYearID, "# "+name)
	if err != nil {
		t.Fatalf("new report template: %v", err)
	}
	if err := repo.CreateReportTemplate(ctx, tenantA, tmpl); err != nil {
		t.Fatalf("create report template: %v", err)
	}
	return tmpl
}

func mustCreateCard(ctx context.Context, t *testing.T, repo ports.Repository, studentID, templateID string) *domain.ReportCard {
	t.Helper()
	card, err := domain.NewReportCard(tenantA, studentID, ay1, templateID)
	if err != nil {
		t.Fatalf("new report card: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, card); err != nil {
		t.Fatalf("create report card: %v", err)
	}
	return card
}

func TestRepository_CreateAndGetReportTemplate(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)

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
	repo := newRepo(t)

	mustCreateTemplate(ctx, t, repo, "Q1", ay1)
	t2 := mustCreateTemplate(ctx, t, repo, "Q2", ay1)

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
	repo := newRepo(t)

	mustCreateTemplate(ctx, t, repo, "Q1", ay1)
	mustCreateTemplate(ctx, t, repo, "Q2", ay2)

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
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
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
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	if err := repo.DeleteReportTemplate(ctx, tenantA, tmpl.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetReportTemplateByID(ctx, tenantA, tmpl.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_CreateAndGetReportCard(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)

	got, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get report card: %v", err)
	}
	if got.ID != card.ID || got.StudentID != studentA {
		t.Fatalf("report card mismatch: %+v", got)
	}
}

func TestRepository_ReportGenerationQueueLifecycle(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	tmpl := mustCreateTemplate(ctx, t, repo, "Final", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)

	queued, err := repo.EnqueueReportGeneration(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("enqueue generation: %v", err)
	}
	if queued.Status != string(domain.ReportCardStatusGenerating) {
		t.Fatalf("queued card status = %q", queued.Status)
	}
	if _, err := repo.EnqueueReportGeneration(ctx, tenantA, card.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate enqueue should conflict, got %v", err)
	}

	job, err := repo.ClaimReportGeneration(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("claim generation: %v", err)
	}
	if job.ReportCardID != card.ID || job.TenantID != tenantA || job.Attempts != 1 {
		t.Fatalf("unexpected generation job: %+v", job)
	}
	if _, err := repo.ClaimReportGeneration(context.Background(), time.Minute); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("active lease should hide job, got %v", err)
	}

	published, err := repo.CompleteReportGeneration(ctx, job, "reports/tenant-a/card.pdf")
	if err != nil {
		t.Fatalf("complete generation: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) || published.PDFPath == nil {
		t.Fatalf("unexpected published card: %+v", published)
	}
	outboxRepo, ok := repo.(ports.OutboxRepository)
	if !ok {
		t.Fatal("repository does not expose the outbox contract")
	}
	outbox, err := outboxRepo.ClaimPendingReportEvents(context.Background(), 10)
	if err != nil || len(outbox) != 1 {
		t.Fatalf("published transition outbox=%+v err=%v", outbox, err)
	}
	if outbox[0].TenantID != tenantA || outbox[0].EventType != "report.published.v1" {
		t.Fatalf("unexpected published outbox event: %+v", outbox[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(outbox[0].Payload, &payload); err != nil {
		t.Fatalf("decode published outbox payload: %v", err)
	}
	if payload["report_card_id"] != card.ID || payload["file_url"] != "/api/v1/report-cards/"+card.ID+"/download" {
		t.Fatalf("unexpected published payload: %+v", payload)
	}
	if _, leaked := payload["pdf_path"]; leaked {
		t.Fatalf("outbox leaked private storage path: %+v", payload)
	}
	if err := outboxRepo.MarkReportEventPublished(context.Background(), outbox[0].ID); err != nil {
		t.Fatalf("mark published event: %v", err)
	}
	if pending, err := outboxRepo.ClaimPendingReportEvents(context.Background(), 10); err != nil || len(pending) != 0 {
		t.Fatalf("published outbox must be drained: pending=%+v err=%v", pending, err)
	}
	if _, err := repo.ClaimReportGeneration(context.Background(), time.Minute); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("completed job should not be reclaimable, got %v", err)
	}

	failing := mustCreateCard(ctx, t, repo, studentB, tmpl.ID)
	if _, err := repo.EnqueueReportGeneration(ctx, tenantA, failing.ID); err != nil {
		t.Fatal(err)
	}
	failedJob, err := repo.ClaimReportGeneration(context.Background(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := repo.RetryReportGeneration(ctx, failedJob, "renderer failed", 1)
	if err != nil || !terminal {
		t.Fatalf("terminal retry: terminal=%v err=%v", terminal, err)
	}
	failedCard, err := repo.GetReportCardByID(ctx, tenantA, failing.ID)
	if err != nil || failedCard.Status != string(domain.ReportCardStatusDraft) {
		t.Fatalf("terminal failure must restore draft: card=%+v err=%v", failedCard, err)
	}
}

func TestRepository_ListReportCardsFilter(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	mustCreateCard(ctx, t, repo, studentA, tmpl.ID)
	mustCreateCard(ctx, t, repo, studentB, tmpl.ID)

	page, _, err := repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10, StudentID: studentA})
	if err != nil {
		t.Fatalf("list report cards: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 report card, got %d", len(page))
	}
	page, _, err = repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10, StudentIDs: []string{studentA}})
	if err != nil {
		t.Fatalf("list learner-scoped report cards: %v", err)
	}
	if len(page) != 1 || page[0].StudentID != studentA {
		t.Fatalf("expected only learner-scoped report card, got %+v", page)
	}
	page, _, err = repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10, StudentIDs: []string{}})
	if err != nil {
		t.Fatalf("list empty learner scope: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("expected empty learner scope to fail closed, got %+v", page)
	}
}

func TestRepository_UpdateReportCard(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)
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
	repo := newRepo(t)

	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)
	if err := repo.DeleteReportCard(ctx, tenantA, card.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetReportCardByID(ctx, tenantA, card.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation_ReportTemplates(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	tmpl := mustCreateTemplate(aCtx, t, repo, "Midterm", ay1)

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
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	tmpl := mustCreateTemplate(aCtx, t, repo, "Midterm", ay1)
	card := mustCreateCard(aCtx, t, repo, studentA, tmpl.ID)

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
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureReportCards, false)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermPublish)

	_, err := svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name:           "Midterm",
		AcademicYearID: ay1,
		BodyTemplate:   "# Template",
	})
	if !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled error, got %v", err)
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermPublish)

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

func TestService_ReportLifecycleUsesTransactionalOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermPublish)

	tmpl, err := svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name: "Midterm", AcademicYearID: ay1, BodyTemplate: "# Template",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	card, err := svc.CreateReportCard(ctx, actor, application.CreateReportCardRequest{
		StudentID: studentA, AcademicYearID: ay1, TemplateID: tmpl.ID,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	name := "Final"
	if _, err := svc.UpdateReportTemplate(ctx, actor, tmpl.ID, application.UpdateReportTemplateRequest{Name: &name}); err != nil {
		t.Fatalf("update template: %v", err)
	}
	student := studentB
	if _, err := svc.UpdateReportCard(ctx, actor, card.ID, application.UpdateReportCardRequest{StudentID: &student}); err != nil {
		t.Fatalf("update card: %v", err)
	}
	if err := svc.DeleteReportCard(ctx, actor, card.ID); err != nil {
		t.Fatalf("delete card: %v", err)
	}
	if err := svc.DeleteReportTemplate(ctx, actor, tmpl.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}

	events, err := repo.ClaimPendingReportEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("claim lifecycle events: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("expected six durable lifecycle events, got %d", len(events))
	}
	counts := map[string]int{}
	for _, event := range events {
		if event.ID == "" || event.TenantID != tenantA || len(event.Payload) == 0 {
			t.Fatalf("invalid outbox event: %+v", event)
		}
		counts[event.EventType]++
	}
	for _, eventType := range []string{"report.created.v1", "report.updated.v1", "report.deleted.v1"} {
		if counts[eventType] != 2 {
			t.Fatalf("expected two %s events, got %d", eventType, counts[eventType])
		}
	}
}

func TestService_ReportLifecycleRollsBackWhenOutboxUnavailable(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermPublish)

	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE report_outbox`); err != nil {
		t.Fatalf("remove outbox: %v", err)
	}
	tmpl, err := svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name: "Must rollback", AcademicYearID: ay1, BodyTemplate: "# Template",
	})
	if err == nil {
		t.Fatal("expected create to fail when its lifecycle event cannot be committed")
	}
	if tmpl != nil {
		t.Fatalf("failed create returned an aggregate: %+v", tmpl)
	}
	page, _, listErr := repo.ListReportTemplates(ctx, tenantA, ports.ReportTemplateListFilter{Limit: 10})
	if listErr != nil {
		t.Fatalf("list after rollback: %v", listErr)
	}
	if len(page) != 0 {
		t.Fatalf("template committed without its event: %+v", page)
	}

	seed, seedErr := domain.NewReportTemplate(tenantA, "Existing", ay1, "# Existing")
	if seedErr != nil {
		t.Fatalf("build seed template: %v", seedErr)
	}
	if err := repo.CreateReportTemplate(ctx, tenantA, seed); err != nil {
		t.Fatalf("seed template directly: %v", err)
	}
	card, err := svc.CreateReportCard(ctx, actor, application.CreateReportCardRequest{
		StudentID: studentA, AcademicYearID: ay1, TemplateID: seed.ID,
	})
	if err == nil {
		t.Fatal("expected card create to fail when its lifecycle event cannot be committed")
	}
	if card != nil {
		t.Fatalf("failed card create returned an aggregate: %+v", card)
	}
	cards, _, listCardsErr := repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10})
	if listCardsErr != nil {
		t.Fatalf("list cards after rollback: %v", listCardsErr)
	}
	if len(cards) != 0 {
		t.Fatalf("report card committed without its event: %+v", cards)
	}
}

func TestService_GenerateReportCardRoundtrip(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureReportCards, true)

	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo,
		application.WithFeatureGate(gates),
		application.WithPDFGenerator(pdf.NewGenerator()),
		application.WithStorage(store),
	)
	publisher := actorWithPerms(tenantA, application.PermPublish)

	tmpl, err := svc.CreateReportTemplate(ctx, publisher, application.CreateReportTemplateRequest{
		Name:           "Midterm",
		AcademicYearID: ay1,
		BodyTemplate:   "# Template",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	card, err := svc.CreateReportCard(ctx, publisher, application.CreateReportCardRequest{
		StudentID:      studentA,
		AcademicYearID: ay1,
		TemplateID:     tmpl.ID,
	})
	if err != nil {
		t.Fatalf("create report card: %v", err)
	}

	if _, err := svc.RequestReportCardGeneration(ctx, publisher, card.ID); err != nil {
		t.Fatalf("queue report card: %v", err)
	}
	if processed, err := svc.ProcessNextGeneration(context.Background(), time.Minute, 5); err != nil || !processed {
		t.Fatalf("process report card: processed=%v err=%v", processed, err)
	}
	published, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get published report card: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published status, got %q", published.Status)
	}
	if published.PDFPath == nil || *published.PDFPath == "" {
		t.Fatal("expected pdf_path to be set")
	}
}
