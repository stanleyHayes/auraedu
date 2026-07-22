package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/adapters/postgres"
	"github.com/auraedu/market-intelligence-service/internal/application"
	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/testkit"
)

func actor(tenant, user, role string, permissions ...string) auth.Actor {
	return auth.Actor{TenantID: tenant, UserID: user, Role: role, Permissions: permissions}
}

func TestPostgresReviewLifecycleIsolationAndTransactionalOutbox(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	creator := actor("school-one", "researcher", "growth_analyst", application.PermManage, application.PermRead)
	reviewer := actor("school-one", "compliance", "compliance_officer", application.PermReview)
	source, err := svc.CreateSource(ctx, creator, application.CreateSourceInput{Kind: domain.KindReputation, Name: "Official review profile", CanonicalURL: "https://reviews.example.edu/school-one", CollectionMethod: "manual", TermsReference: "Manual research authority LEG-14"})
	if err != nil {
		t.Fatal(err)
	}
	source, err = svc.ReviewSource(ctx, reviewer, source.ID, "approved", "Manual collection authority verified")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := svc.UpdateAlertRule(ctx, creator, 3, 14)
	if err != nil || rule.Threshold != 3 || rule.WindowDays != 14 {
		t.Fatalf("rule=%+v err=%v", rule, err)
	}
	observation, err := svc.CreateObservation(ctx, creator, application.CreateObservationInput{SourceID: source.ID, Category: "misinformation", Title: "Outdated admissions deadline", EvidenceExcerpt: "A public post contains an outdated deadline.", Sentiment: "negative", ResponseDraft: "Please see the official deadline page.", ObservedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	observation, err = svc.ReviewObservation(ctx, actor("school-one", "editor", "communications_lead", application.PermReview), observation.ID, "approved", "Evidence checked and response retained as internal draft")
	if err != nil {
		t.Fatal(err)
	}
	observation, err = svc.ResolveObservation(ctx, actor("school-one", "editor", "communications_lead", application.PermManage), observation.ID, "Owned admissions guidance was corrected")
	if err != nil || observation.Status != domain.StatusResolved {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
	for index := 2; index <= 3; index++ {
		item, createErr := svc.CreateObservation(ctx, creator, application.CreateObservationInput{SourceID: source.ID, Category: "misinformation", Title: "Repeated outdated deadline", EvidenceExcerpt: "Another approved public record repeats the same outdated deadline.", Sentiment: "negative", ObservedAt: now.Add(-time.Duration(index) * time.Hour)})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = svc.ReviewObservation(ctx, actor("school-one", "editor", "communications_lead", application.PermReview), item.ID, "approved", "Independent evidence review completed"); createErr != nil {
			t.Fatal(createErr)
		}
	}
	alerts, err := svc.ListAlerts(ctx, creator, "open", 100)
	if err != nil || len(alerts) != 1 || alerts[0].ObservationCount != 3 || alerts[0].Threshold != 3 {
		t.Fatalf("alerts=%+v err=%v", alerts, err)
	}
	alert, err := svc.AcknowledgeAlert(ctx, creator, alerts[0].ID, "Communications owner assigned and official deadline guidance queued")
	if err != nil || alert.Status != "acknowledged" {
		t.Fatalf("alert=%+v err=%v", alert, err)
	}
	competitorSource, err := svc.CreateSource(ctx, actor("school-one", "market-analyst", "market_analyst", application.PermManage, application.PermRead), application.CreateSourceInput{Kind: domain.KindCompetitor, Name: "Official competitor programme API", CanonicalURL: "https://competitor.example.edu/api/programmes", CollectionMethod: "official_api", TermsReference: "Official API terms permit metadata comparison"})
	if err != nil {
		t.Fatal(err)
	}
	competitorSource, err = svc.ReviewSource(ctx, reviewer, competitorSource.ID, "approved", "Official API terms and rate limits verified")
	if err != nil {
		t.Fatal(err)
	}
	marketActor := actor("school-one", "market-analyst", "market_analyst", application.PermManage, application.PermRead)
	marketReviewer := actor("school-one", "market-lead", "market_lead", application.PermReview)
	for index, evidence := range []string{"Official fee was GHS 4,000.", "Official fee is now GHS 4,500."} {
		observed := now.Add(-10 * 24 * time.Hour)
		if index == 1 {
			observed = now.Add(-24 * time.Hour)
		}
		item, createErr := svc.CreateObservation(ctx, marketActor, application.CreateObservationInput{SourceID: competitorSource.ID, Category: "fee", Title: "Engineering programme fee", EvidenceExcerpt: evidence, ObservedAt: observed})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = svc.ReviewObservation(ctx, marketReviewer, item.ID, "approved", "Official API evidence independently verified"); createErr != nil {
			t.Fatal(createErr)
		}
	}
	summary, err := svc.GenerateSummary(ctx, marketActor, now.Add(-7*24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ItemCount != 1 || summary.Items[0].ChangeType != "changed" || summary.Items[0].PreviousExcerpt == nil {
		t.Fatalf("summary=%+v", summary)
	}
	if _, err = svc.ReviewSummary(ctx, actor("school-one", "market-analyst", "market_analyst", application.PermReview), summary.ID, "approved", "self review"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("summary self review=%v", err)
	}
	summary, err = svc.ReviewSummary(ctx, marketReviewer, summary.ID, "approved", "Previous and latest official evidence verified")
	if err != nil || summary.Status != domain.StatusApproved {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	other, err := svc.ListObservations(ctx, actor("school-two", "other", "analyst", application.PermRead), domain.KindReputation, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("cross-tenant records leaked: %+v", other)
	}
	otherAlerts, err := svc.ListAlerts(ctx, actor("school-two", "other", "analyst", application.PermRead), "", 100)
	if err != nil || len(otherAlerts) != 0 {
		t.Fatalf("cross-tenant alerts=%+v err=%v", otherAlerts, err)
	}
	otherSummaries, err := svc.ListSummaries(ctx, actor("school-two", "other", "analyst", application.PermRead), "", 100)
	if err != nil || len(otherSummaries) != 0 {
		t.Fatalf("cross-tenant summaries=%+v err=%v", otherSummaries, err)
	}
	events, err := repo.ClaimPending(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 20 {
		t.Fatalf("expected 20 transactional lifecycle events, got %d", len(events))
	}
	for _, event := range events {
		if event.TenantID != "school-one" {
			t.Fatalf("wrong event tenant: %+v", event)
		}
	}
}
