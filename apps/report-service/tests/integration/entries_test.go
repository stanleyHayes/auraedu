package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/auraedu/platform/flags"
	"github.com/auraedu/report-service/internal/adapters/pdf"
	"github.com/auraedu/report-service/internal/adapters/storage"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

const term1 = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
const term2 = "ffffffff-ffff-ffff-ffff-ffffffffffff"
const subject1 = "99999999-9999-9999-9999-999999999999"
const assessment1 = "12345678-1234-1234-1234-123456789abc"

func mustScoreEntry(t *testing.T, cardID string, score float64) *domain.ScoreEntry {
	t.Helper()
	maximum := 100.0
	e, err := domain.NewScoreEntry(tenantA, cardID, studentA, subject1, assessment1, "evt-1", score, &maximum)
	if err != nil {
		t.Fatalf("new score entry: %v", err)
	}
	return e
}

func TestRepository_ScoreEntryUpsertIdempotent(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)

	if err := repo.UpsertScoreEntry(ctx, tenantA, mustScoreEntry(t, card.ID, 72)); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Replay of the same event (same natural key): still one row.
	if err := repo.UpsertScoreEntry(ctx, tenantA, mustScoreEntry(t, card.ID, 72)); err != nil {
		t.Fatalf("replay upsert: %v", err)
	}
	// Correction: new event, same assessment, new score value.
	if err := repo.UpsertScoreEntry(ctx, tenantA, mustScoreEntry(t, card.ID, 81)); err != nil {
		t.Fatalf("correction upsert: %v", err)
	}

	entries, err := repo.ListScoreEntries(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after replay+correction, got %d", len(entries))
	}
	if entries[0].Score != 81 || entries[0].SubjectID != subject1 {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestRepository_AttendanceEntryUpsertRemark(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)
	card := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)

	mk := func(status string) *domain.AttendanceEntry {
		e, err := domain.NewAttendanceEntry(tenantA, card.ID, studentA, "2026-07-08", status, "evt-1")
		if err != nil {
			t.Fatalf("new attendance entry: %v", err)
		}
		return e
	}
	if err := repo.UpsertAttendanceEntry(ctx, tenantA, mk("present")); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertAttendanceEntry(ctx, tenantA, mk("late")); err != nil {
		t.Fatalf("remark upsert: %v", err)
	}

	entries, err := repo.ListAttendanceEntries(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after re-mark, got %d", len(entries))
	}
	if entries[0].Status != domain.AttendanceStatusLate {
		t.Fatalf("expected late, got %q", entries[0].Status)
	}
}

func TestRepository_FindDraftReportCardRouting(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)
	tmpl := mustCreateTemplate(ctx, t, repo, "Midterm", ay1)

	// API-created draft, no term assigned.
	apiCard := mustCreateCard(ctx, t, repo, studentA, tmpl.ID)

	// Empty term (attendance-style): finds the NULL-term draft.
	got, err := repo.FindDraftReportCard(ctx, tenantA, studentA, "")
	if err != nil || got.ID != apiCard.ID {
		t.Fatalf("empty term should find the NULL-term draft, got %+v, err %v", got, err)
	}
	// Any term: NULL-term draft still matches (period not yet assigned).
	got, err = repo.FindDraftReportCard(ctx, tenantA, studentA, term1)
	if err != nil || got.ID != apiCard.ID {
		t.Fatalf("term lookup should fall back to the NULL-term draft, got %+v, err %v", got, err)
	}

	// Auto-created draft for term1: exact match must win over the NULL-term card.
	autoDraft, err := domain.NewEventDraftReportCard(tenantA, studentA, "", term1)
	if err != nil {
		t.Fatalf("new event draft: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, autoDraft); err != nil {
		t.Fatalf("create event draft: %v", err)
	}
	got, err = repo.FindDraftReportCard(ctx, tenantA, studentA, term1)
	if err != nil || got.ID != autoDraft.ID {
		t.Fatalf("exact term match should win, got %+v, err %v", got, err)
	}
	// Empty term: most recently created draft wins (the term1 auto-draft).
	got, err = repo.FindDraftReportCard(ctx, tenantA, studentA, "")
	if err != nil || got.ID != autoDraft.ID {
		t.Fatalf("empty term should find the latest draft, got %+v, err %v", got, err)
	}
	// term2 matches only the NULL-term card.
	got, err = repo.FindDraftReportCard(ctx, tenantA, studentA, term2)
	if err != nil || got.ID != apiCard.ID {
		t.Fatalf("term2 should fall back to the NULL-term draft, got %+v, err %v", got, err)
	}

	// Unknown student: not found.
	if _, err := repo.FindDraftReportCard(ctx, tenantA, studentB, term1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRepository_TenantIsolation_Entries(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	tmpl := mustCreateTemplate(aCtx, t, repo, "Midterm", ay1)
	card := mustCreateCard(aCtx, t, repo, studentA, tmpl.ID)
	if err := repo.UpsertScoreEntry(aCtx, tenantA, mustScoreEntry(t, card.ID, 72)); err != nil {
		t.Fatalf("upsert score: %v", err)
	}
	att, err := domain.NewAttendanceEntry(tenantA, card.ID, studentA, "2026-07-08", "present", "evt-1")
	if err != nil {
		t.Fatalf("new attendance entry: %v", err)
	}
	if err := repo.UpsertAttendanceEntry(aCtx, tenantA, att); err != nil {
		t.Fatalf("upsert attendance: %v", err)
	}

	bCtx := withTenant(ctx, tenantB)
	scores, err := repo.ListScoreEntries(bCtx, tenantB, card.ID)
	if err != nil {
		t.Fatalf("list scores B: %v", err)
	}
	if len(scores) != 0 {
		t.Fatalf("tenant B must not see tenant A score entries, got %d", len(scores))
	}
	attendance, err := repo.ListAttendanceEntries(bCtx, tenantB, card.ID)
	if err != nil {
		t.Fatalf("list attendance B: %v", err)
	}
	if len(attendance) != 0 {
		t.Fatalf("tenant B must not see tenant A attendance entries, got %d", len(attendance))
	}
	// The draft lookup must not leak across tenants either.
	if _, err := repo.FindDraftReportCard(bCtx, tenantB, studentA, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B must not find tenant A drafts, got %v", err)
	}
}

// TestService_MaterializeThenGenerate covers the end-to-end path: events
// materialize entries onto an auto-created draft, generation renders them.
func TestService_MaterializeThenGenerate(t *testing.T) {
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

	maximum := 100.0
	err = svc.MaterializeScore(ctx, application.ScoreRecordedInput{
		EventID: "evt-s1", TenantID: tenantA, StudentID: studentA,
		SubjectID: subject1, AssessmentID: assessment1, TermID: term1, Score: 72, MaxScore: &maximum,
	})
	if err != nil {
		t.Fatalf("materialize score: %v", err)
	}
	err = svc.MaterializeAttendance(ctx, application.AttendanceMarkedInput{
		EventID: "evt-a1", TenantID: tenantA, StudentID: studentA, Date: "2026-07-08", Status: "present",
	})
	if err != nil {
		t.Fatalf("materialize attendance: %v", err)
	}

	// Both events must land on the same card even though only the score
	// carries a term.
	cards, _, err := repo.ListReportCards(ctx, tenantA, ports.ReportCardListFilter{Limit: 10, StudentID: studentA})
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected a single auto-created draft, got %d", len(cards))
	}
	card := cards[0]

	if _, err := svc.RequestReportCardGeneration(ctx, actorWithPerms(tenantA, application.PermPublish), card.ID); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if processed, err := svc.ProcessNextGeneration(context.Background(), time.Minute, 5); err != nil || !processed {
		t.Fatalf("process: processed=%v err=%v", processed, err)
	}
	published, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get published: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published, got %q", published.Status)
	}

	reader, _, err := svc.DownloadReportCard(ctx, actorWithPerms(tenantA, application.PermRead), card.ID)
	if err != nil {
		t.Fatalf("download pdf: %v", err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close pdf: %v", err)
	}
	for _, want := range []string{subject1, "72", "100", "Present: 1", studentA, term1} {
		if !bytes.Contains(content, []byte(want)) {
			t.Errorf("generated PDF missing %q", want)
		}
	}
}
