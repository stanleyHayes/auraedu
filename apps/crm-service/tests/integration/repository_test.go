package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/crm-service/internal/adapters/postgres"
	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func tenantContext(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func TestAdmissionsProjectionIsIdempotentAndNeverRegresses(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	ctx := tenantContext("school-a")
	created, err := repo.Capture(ctx, lead(t, "school-a", "funnel@example.com"), "projection-key", "projection-request", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	acceptedEvent := uuid.NewString()
	if err = repo.ProjectAdmissionsStage(ctx, "school-a", created.Lead.ID, acceptedEvent, "offer.accepted.v1", domain.StageOfferAccepted, now); err != nil {
		t.Fatal(err)
	}
	if err = repo.ProjectAdmissionsStage(ctx, "school-a", created.Lead.ID, acceptedEvent, "offer.accepted.v1", domain.StageOfferAccepted, now); err != nil {
		t.Fatal(err)
	}
	if err = repo.ProjectAdmissionsStage(ctx, "school-a", created.Lead.ID, uuid.NewString(), "application.started.v1", domain.StageApplicationStarted, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetLead(ctx, "school-a", created.Lead.ID)
	if err != nil || got.Stage != domain.StageOfferAccepted {
		t.Fatalf("projected=%+v err=%v", got, err)
	}
	interactions, _, err := repo.ListInteractions(ctx, "school-a", created.Lead.ID, 10, "")
	if err != nil || len(interactions) != 1 {
		t.Fatalf("interactions=%d err=%v", len(interactions), err)
	}
}

func lead(t *testing.T, tenantID, email string) *domain.Lead {
	t.Helper()
	item, err := domain.NewLead(tenantID, "Ama", "Mensah", &email, nil, "website", domain.Consent{PrivacyNoticeVersion: "2026-01", Email: true})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestCaptureIdempotencyDedupeAndTenantIsolation(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)

	ctxA := tenantContext("school-a")
	first, err := repo.Capture(ctxA, lead(t, "school-a", "ama@example.com"), "key-a", "request-a", nil)
	if err != nil || !first.Created || first.Replay {
		t.Fatalf("first capture: %+v err=%v", first, err)
	}
	replay, err := repo.Capture(ctxA, lead(t, "school-a", "ama@example.com"), "key-a", "request-a", nil)
	if err != nil || !replay.Replay || replay.Lead.ID != first.Lead.ID {
		t.Fatalf("replay: %+v err=%v", replay, err)
	}
	if _, err := repo.Capture(ctxA, lead(t, "school-a", "other@example.com"), "key-a", "changed", nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	dedupe, err := repo.Capture(ctxA, lead(t, "school-a", "ama@example.com"), "key-b", "request-b", nil)
	if err != nil || dedupe.Created || dedupe.Lead.ID != first.Lead.ID {
		t.Fatalf("dedupe: %+v err=%v", dedupe, err)
	}

	ctxB := tenantContext("school-b")
	if _, err := repo.GetLead(ctxB, "school-b", first.Lead.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant B read should be hidden, got %v", err)
	}
	secondTenant, err := repo.Capture(ctxB, lead(t, "school-b", "ama@example.com"), "key-a", "request-a", nil)
	if err != nil || !secondTenant.Created || secondTenant.Lead.ID == first.Lead.ID {
		t.Fatalf("tenant-local contact uniqueness failed: %+v err=%v", secondTenant, err)
	}
}

func TestListLeadsSearchIsParameterBoundAgainstSQLInjection(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	ctx := tenantContext("school-a")
	for index, email := range []string{"ama@example.com", "kojo@example.com"} {
		if _, err := repo.Capture(ctx, lead(t, "school-a", email), "inject-key-"+string(rune('a'+index)), "inject-request-"+string(rune('a'+index)), nil); err != nil {
			t.Fatalf("seed lead: %v", err)
		}
	}

	items, _, err := repo.ListLeads(ctx, "school-a", 25, "", ports.LeadFilter{Search: `%' OR 1=1 --`})
	if err != nil {
		t.Fatalf("injection-shaped search must remain valid data: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("parameter binding bypassed search predicate: returned %d leads", len(items))
	}

	all, _, err := repo.ListLeads(ctx, "school-a", 25, "", ports.LeadFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("table integrity changed after probe: leads=%d err=%v", len(all), err)
	}
}

func TestInteractionCannotCrossTenantLeadBoundary(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	created, err := repo.Capture(tenantContext("school-a"), lead(t, "school-a", "a@example.com"), "key-a", "request-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	interaction, err := domain.NewInteraction("school-b", created.Lead.ID, "email", "outbound", "staff", "Follow-up", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateInteraction(tenantContext("school-b"), "school-b", interaction); err == nil {
		t.Fatal("tenant B interaction must not reference tenant A lead")
	}
}

func TestExplainableScorePersistsHistoryAndIsReplaySafe(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	ctx := tenantContext("school-a")
	item := lead(t, "school-a", "score@example.com")
	programme, intake := uuid.NewString(), uuid.NewString()
	item.PreferredProgrammeIDs, item.PreferredIntakeID = []string{programme}, &intake
	captured, err := repo.Capture(ctx, item, "score-key", "score-request", nil)
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := domain.NewInteraction("school-a", captured.Lead.ID, "website", "inbound", "prospect", "Requested an application guide", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateInteraction(ctx, "school-a", inbound); err != nil {
		t.Fatal(err)
	}
	evidence, err := repo.GetScoringEvidence(ctx, "school-a", captured.Lead.ID)
	if err != nil || evidence.InboundProspectInteractions != 1 {
		t.Fatalf("evidence=%+v err=%v", evidence, err)
	}
	score := domain.ScoreLead(*captured.Lead, evidence, time.Now().UTC())
	changed, err := repo.SaveLeadScore(ctx, "school-a", captured.Lead.ID, "test", score)
	if err != nil || !changed {
		t.Fatalf("first save changed=%v err=%v", changed, err)
	}
	changed, err = repo.SaveLeadScore(ctx, "school-a", captured.Lead.ID, "test-replay", score)
	if err != nil || changed {
		t.Fatalf("replay changed=%v err=%v", changed, err)
	}
	got, err := repo.GetLead(ctx, "school-a", captured.Lead.ID)
	if err != nil || got.Score == nil || *got.Score != score.Score || len(got.ScorePositiveFactors) == 0 || got.ScoreConfidence == nil {
		t.Fatalf("stored score=%+v err=%v", got, err)
	}
	if _, err := repo.GetScoringEvidence(tenantContext("school-b"), "school-b", captured.Lead.ID); err != nil {
		t.Fatalf("tenant-safe empty evidence should not leak or fail: %v", err)
	}
}

func TestFeedbackIsIdempotentAndTenantScoped(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	rating := 5
	feedback, err := domain.NewFeedback("school-a", nil, nil, "helpful", &rating, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.SubmitFeedback(tenantContext("school-a"), feedback, "feedback-key", "request-a")
	if err != nil || first.Replay {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayFeedback, createErr := domain.NewFeedback("school-a", nil, nil, "helpful", &rating, nil)
	if createErr != nil {
		t.Fatal(createErr)
	}
	replay, err := repo.SubmitFeedback(tenantContext("school-a"), replayFeedback, "feedback-key", "request-a")
	if err != nil || !replay.Replay || replay.Feedback.ID != first.Feedback.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	conflictFeedback, createErr := domain.NewFeedback("school-a", nil, nil, "unhelpful", &rating, nil)
	if createErr != nil {
		t.Fatal(createErr)
	}
	if _, err := repo.SubmitFeedback(tenantContext("school-a"), conflictFeedback, "feedback-key", "request-b"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestCallbackRequestIsReplaySafeAndTenantIsolated(t *testing.T) {
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	repo := postgres.NewRepository(tdb.DB)
	ctxA, ctxB := tenantContext("school-a"), tenantContext("school-b")
	captured, err := repo.Capture(ctxA, lead(t, "school-a", "callback@example.com"), "lead-key", "lead-request", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	callback, err := domain.NewCallbackRequest("school-a", captured.Lead.ID, now.Add(2*time.Hour), "Africa/Accra", "en-GH", now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.ScheduleCallback(ctxA, callback, "callback-key", "callback-request")
	if err != nil || first.Replay {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replayCandidate, err := domain.NewCallbackRequest("school-a", captured.Lead.ID, now.Add(2*time.Hour), "Africa/Accra", "en-GH", now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repo.ScheduleCallback(ctxA, replayCandidate, "callback-key", "callback-request")
	if err != nil || !replay.Replay || replay.Callback.ID != first.Callback.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err := repo.ScheduleCallback(ctxA, replayCandidate, "callback-key", "changed-request"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	items, err := repo.ListCallbacks(ctxA, "school-a", domain.CallbackRequested, 25)
	if err != nil || len(items) != 1 || items[0].ID != first.Callback.ID {
		t.Fatalf("tenant A items=%+v err=%v", items, err)
	}
	items, err = repo.ListCallbacks(ctxB, "school-b", "", 25)
	if err != nil || len(items) != 0 {
		t.Fatalf("tenant B must not see callback: %+v err=%v", items, err)
	}
	if _, found, err := repo.FindCallbackReplay(ctxB, "school-b", "callback-key", "callback-request"); err != nil || found {
		t.Fatalf("tenant B replay lookup found=%v err=%v", found, err)
	}
}
