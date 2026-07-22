package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/platform/flags"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

const tenantB = "22222222-2222-2222-2222-222222222222"
const term1 = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
const term2 = "ffffffff-ffff-ffff-ffff-ffffffffffff"
const subject1 = "99999999-9999-9999-9999-999999999999"
const assessment1 = "12345678-1234-1234-1234-123456789abc"

func enabledGates(tenants ...string) *flags.StaticSnapshot {
	g := flags.NewStaticSnapshot()
	for _, tenant := range tenants {
		g.Set(tenant, application.FeatureReportCards, true)
	}
	return g
}

func scoreInput(tenantID string) application.ScoreRecordedInput {
	maximum := 100.0
	return application.ScoreRecordedInput{
		EventID:      "evt-1",
		TenantID:     tenantID,
		StudentID:    studentA,
		SubjectID:    subject1,
		AssessmentID: assessment1,
		TermID:       term1,
		Score:        72,
		MaxScore:     &maximum,
	}
}

func TestMaterializeScore_AutoCreatesDraftAndEntry(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))

	if err := svc.MaterializeScore(context.Background(), scoreInput(tenantA)); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	card, err := repo.FindDraftReportCard(context.Background(), tenantA, studentA, term1)
	if err != nil {
		t.Fatalf("expected auto-created draft: %v", err)
	}
	if card.Status != string(domain.ReportCardStatusDraft) {
		t.Fatalf("expected draft status, got %q", card.Status)
	}
	if card.TermID != term1 {
		t.Fatalf("expected term %q, got %q", term1, card.TermID)
	}

	entries, err := repo.ListScoreEntries(context.Background(), tenantA, card.ID)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 score entry, got %d", len(entries))
	}
	if entries[0].Score != 72 || entries[0].SourceKey != assessment1 || entries[0].SubjectID != subject1 {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestMaterializeScore_IdempotentReplayAndCorrection(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	ctx := context.Background()

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Exact replay of the same event.
	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("replay: %v", err)
	}
	// Correction: new event id, same assessment (natural key), new score.
	corrected := scoreInput(tenantA)
	corrected.EventID = "evt-2"
	corrected.Score = 81
	if err := svc.MaterializeScore(ctx, corrected); err != nil {
		t.Fatalf("correction: %v", err)
	}

	card, err := repo.FindDraftReportCard(ctx, tenantA, studentA, term1)
	if err != nil {
		t.Fatalf("find draft: %v", err)
	}
	entries, err := repo.ListScoreEntries(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("replay/correction must converge to one entry, got %d", len(entries))
	}
	if entries[0].Score != 81 {
		t.Fatalf("expected corrected score 81, got %v", entries[0].Score)
	}
	if entries[0].LastEventID != "evt-2" {
		t.Fatalf("expected last_event_id evt-2, got %q", entries[0].LastEventID)
	}
}

func TestMaterializeScore_UsesExistingDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	ctx := context.Background()

	// API-created draft with a template and no term assigned: matches any term.
	existing, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("new card: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, existing); err != nil {
		t.Fatalf("create card: %v", err)
	}

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	entries, err := repo.ListScoreEntries(ctx, tenantA, existing.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected entry on the existing draft, got %d", len(entries))
	}

	cards, _, err := repo.ListReportCards(ctx, tenantA, listAll())
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("no extra draft should be created, got %d cards", len(cards))
	}
}

func TestMaterializeScore_SkipsWhenFeatureDisabled(t *testing.T) {
	repo := newFakeRepo()
	gates := flags.NewStaticSnapshot() // everything disabled by default
	svc := application.NewService(repo, application.WithFeatureGate(gates))

	if err := svc.MaterializeScore(context.Background(), scoreInput(tenantA)); err != nil {
		t.Fatalf("disabled feature must be a silent skip, got %v", err)
	}
	cards, _, err := repo.ListReportCards(context.Background(), tenantA, listAll())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("no draft should be created when the feature is off, got %d", len(cards))
	}
}

func TestMaterializeScore_FeatureFlagTenantMatrix(t *testing.T) {
	repo := newFakeRepo()
	gates := enabledGates(tenantA)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	ctx := context.Background()

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("enabled tenant materialize: %v", err)
	}
	disabled := scoreInput(tenantB)
	disabled.EventID = "evt-disabled-tenant"
	if err := svc.MaterializeScore(ctx, disabled); err != nil {
		t.Fatalf("disabled tenant must be a silent worker skip: %v", err)
	}

	cardsA, _, err := repo.ListReportCards(ctx, tenantA, listAll())
	if err != nil {
		t.Fatalf("list enabled tenant: %v", err)
	}
	cardsB, _, err := repo.ListReportCards(ctx, tenantB, listAll())
	if err != nil {
		t.Fatalf("list disabled tenant: %v", err)
	}
	if len(cardsA) != 1 || len(cardsB) != 0 {
		t.Fatalf("matrix violated: enabled tenant cards=%d disabled tenant cards=%d", len(cardsA), len(cardsB))
	}
}

func TestMaterializeScore_TenantScoping(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA, tenantB)))
	ctx := context.Background()

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("materialize A: %v", err)
	}

	// Tenant B must not see tenant A's draft: a separate draft is created.
	bInput := scoreInput(tenantB)
	bInput.EventID = "evt-b1"
	if err := svc.MaterializeScore(ctx, bInput); err != nil {
		t.Fatalf("materialize B: %v", err)
	}

	cardA, err := repo.FindDraftReportCard(ctx, tenantA, studentA, term1)
	if err != nil {
		t.Fatalf("find A: %v", err)
	}
	cardB, err := repo.FindDraftReportCard(ctx, tenantB, studentA, term1)
	if err != nil {
		t.Fatalf("find B: %v", err)
	}
	if cardA.ID == cardB.ID {
		t.Fatal("tenants must get separate draft cards")
	}

	entriesA, err := repo.ListScoreEntries(ctx, tenantA, cardA.ID)
	if err != nil {
		t.Fatalf("list tenant A scores: %v", err)
	}
	entriesB, err := repo.ListScoreEntries(ctx, tenantB, cardB.ID)
	if err != nil {
		t.Fatalf("list tenant B scores: %v", err)
	}
	if len(entriesA) != 1 || len(entriesB) != 1 {
		t.Fatalf("expected one entry per tenant, got A=%d B=%d", len(entriesA), len(entriesB))
	}
	// Cross-tenant reads see nothing.
	leaked, err := repo.ListScoreEntries(ctx, tenantB, cardA.ID)
	if err != nil {
		t.Fatalf("list cross-tenant scores: %v", err)
	}
	if len(leaked) != 0 {
		t.Fatalf("tenant B must not list tenant A entries, got %d", len(leaked))
	}
}

func TestMaterializeScore_PeriodsGetSeparateDrafts(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	ctx := context.Background()

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("term1: %v", err)
	}
	other := scoreInput(tenantA)
	other.EventID = "evt-3"
	other.TermID = term2
	if err := svc.MaterializeScore(ctx, other); err != nil {
		t.Fatalf("term2: %v", err)
	}

	cards, _, err := repo.ListReportCards(ctx, tenantA, listAll())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected one draft per period, got %d", len(cards))
	}
}

func TestMaterializeScore_CreateConflictRereadsDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	ctx := context.Background()

	// Simulate the race: the "concurrent worker" has already created the draft,
	// but our first lookup misses it and our create loses on the unique index.
	// The use case must re-read and attach the entry to the winner's card.
	winner, err := domain.NewEventDraftReportCard(tenantA, studentA, "", term1)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, winner); err != nil {
		t.Fatalf("create winner: %v", err)
	}
	repo.hideNextFindDraft = true
	repo.failNextCreateWithConflict = true

	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	entries, err := repo.ListScoreEntries(ctx, tenantA, winner.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected the entry to land on the winner draft, got %d", len(entries))
	}
	cards, _, err := repo.ListReportCards(ctx, tenantA, listAll())
	if err != nil {
		t.Fatalf("list cards after conflict: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("conflict path must not duplicate drafts, got %d", len(cards))
	}
}

func TestMaterializeScore_ValidationError(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))

	bad := scoreInput(tenantA)
	bad.Score = -1
	if err := svc.MaterializeScore(context.Background(), bad); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	missing := scoreInput(tenantA)
	missing.StudentID = ""
	if err := svc.MaterializeScore(context.Background(), missing); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for empty student, got %v", err)
	}

	noTenant := scoreInput("")
	if err := svc.MaterializeScore(context.Background(), noTenant); !errors.Is(err, domain.ErrMissingTenant) {
		t.Fatalf("expected missing-tenant error, got %v", err)
	}
}

func TestMaterializeAttendance_IdempotentRemark(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	ctx := context.Background()

	in := application.AttendanceMarkedInput{
		EventID: "evt-a1", TenantID: tenantA, StudentID: studentA, Date: "2026-07-08", Status: "present",
	}
	if err := svc.MaterializeAttendance(ctx, in); err != nil {
		t.Fatalf("first: %v", err)
	}
	in.EventID = "evt-a2"
	in.Status = "late" // re-mark of the same day
	if err := svc.MaterializeAttendance(ctx, in); err != nil {
		t.Fatalf("remark: %v", err)
	}

	card, err := repo.FindDraftReportCard(ctx, tenantA, studentA, "")
	if err != nil {
		t.Fatalf("find draft: %v", err)
	}
	entries, err := repo.ListAttendanceEntries(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("re-mark must converge to one entry, got %d", len(entries))
	}
	if entries[0].Status != domain.AttendanceStatusLate {
		t.Fatalf("expected late, got %q", entries[0].Status)
	}
}

func TestMaterializeAttendance_SkipsWhenFeatureDisabled(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(flags.NewStaticSnapshot()))

	in := application.AttendanceMarkedInput{
		EventID: "evt-a1", TenantID: tenantA, StudentID: studentA, Date: "2026-07-08", Status: "present",
	}
	if err := svc.MaterializeAttendance(context.Background(), in); err != nil {
		t.Fatalf("disabled feature must be a silent skip, got %v", err)
	}
	cards, _, err := repo.ListReportCards(context.Background(), tenantA, listAll())
	if err != nil {
		t.Fatalf("list cards for disabled feature: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("no draft should be created when the feature is off, got %d", len(cards))
	}
}

func TestMaterializeAttendance_InvalidStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo, application.WithFeatureGate(enabledGates(tenantA)))
	in := application.AttendanceMarkedInput{
		EventID: "evt-a1", TenantID: tenantA, StudentID: studentA, Date: "2026-07-08", Status: "unknown",
	}
	if err := svc.MaterializeAttendance(context.Background(), in); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func listAll() ports.ReportCardListFilter { return ports.ReportCardListFilter{Limit: 100} }
