package mongo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoReportGenerationFencing(t *testing.T) {
	ctx := context.Background()
	store := testkit.NewMongo(ctx, t).Store
	if err := EnsureIndexes(ctx, store); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(store)
	card, err := domain.NewEventDraftReportCard("tenant-a", "student", "year", "term")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.CreateReportCard(ctx, "tenant-a", card); err != nil {
		t.Fatal(err)
	}
	duplicate, err := domain.NewEventDraftReportCard("tenant-a", "student", "year", "term")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.CreateReportCard(ctx, "tenant-a", duplicate); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("draft uniqueness %v", err)
	}
	if _, err := r.GetReportCardByID(ctx, "tenant-b", card.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("card isolation %v", err)
	}
	e, err := domain.NewScoreEntry("tenant-a", card.ID, "student", "subject", "exam.$danger", "event", 42, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertScoreEntry(ctx, "tenant-a", e); err != nil {
		t.Fatal(err)
	}
	e.Score = 45
	if err := r.UpsertScoreEntry(ctx, "tenant-a", e); err != nil {
		t.Fatal(err)
	}
	scores, err := r.ListScoreEntries(ctx, "tenant-a", card.ID)
	if err != nil || len(scores) != 1 || scores[0].Score != 45 {
		t.Fatalf("score upsert %+v %v", scores, err)
	}
	attendance, err := domain.NewAttendanceEntry("tenant-a", card.ID, "student", "2026-09-12", "present", "event")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertAttendanceEntry(ctx, "tenant-b", attendance); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("entry isolation %v", err)
	}
	if err = r.UpsertAttendanceEntry(ctx, "tenant-a", attendance); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnqueueReportGeneration(ctx, "tenant-a", card.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnqueueReportGeneration(ctx, "tenant-a", card.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate enqueue %v", err)
	}
	var wg sync.WaitGroup
	claimed := make(chan *domain.GenerationJob, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := r.ClaimReportGeneration(ctx, time.Minute)
			if err == nil {
				claimed <- j
			} else if !errors.Is(err, domain.ErrNotFound) {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(claimed)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claim winners %d", len(claimed))
	}
	old := <-claimed
	// Expire the real durable lease without sleeping, then reclaim via another worker.
	s, err := store.Collection(CardCollection).ScopeTo("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateOne(ctx, bson.M{"_id": card.ID}, bson.M{"$set": bson.M{"generation.lease": time.Now().Add(-time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := NewRepository(store).ClaimReportGeneration(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Attempts != 2 {
		t.Fatalf("attempt %d", fresh.Attempts)
	}
	if _, err = r.CompleteReportGeneration(ctx, old, "stale.pdf"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale completion accepted %v", err)
	}
	if _, err = r.RetryReportGeneration(ctx, old, "stale", 1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale retry accepted %v", err)
	}
	published, err := r.CompleteReportGeneration(ctx, fresh, "cards/private.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if published.PDFPath == nil || *published.PDFPath != "cards/private.pdf" {
		t.Fatal("pdf path missing")
	}
	persisted, err := r.GetReportCardByID(ctx, "tenant-a", card.ID)
	if err != nil || persisted.PDFPath == nil {
		t.Fatalf("pdf persistence %+v %v", persisted, err)
	}
	events, err := r.ClaimPendingReportEvents(ctx, 100)
	if err != nil || len(events) != 1 || events[0].EventType != "report.published.v1" {
		t.Fatalf("publish outbox %+v %v", events, err)
	}
	if err = r.MarkReportEventPublished(ctx, events[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.EnqueueReportGeneration(ctx, "tenant-a", card.ID); err != nil {
		t.Fatal(err)
	}
	j, err := r.ClaimReportGeneration(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := r.RetryReportGeneration(ctx, j, "broken", 1)
	if err != nil || !terminal {
		t.Fatalf("terminal retry %v %v", terminal, err)
	}
	draft, err := r.GetReportCardByID(ctx, "tenant-a", card.ID)
	if err != nil || draft.Status != "draft" {
		t.Fatalf("retry card %+v %v", draft, err)
	}
	if err = r.CommitReportCardLifecycle(ctx, "tenant-a", draft, ports.ReportMutationDelete, "report.deleted.v1", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	events, err = r.ClaimPendingReportEvents(ctx, 100)
	if err != nil || len(events) != 1 {
		t.Fatalf("delete outbox %+v %v", events, err)
	}
}
