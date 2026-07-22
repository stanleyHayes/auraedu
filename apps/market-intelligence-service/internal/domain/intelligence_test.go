package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/auraedu/market-intelligence-service/internal/domain"
)

func TestLawfulSourceAndFourEyesObservationLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	source, err := domain.NewSource("school-one", domain.KindReputation, "Official school review profile", "https://reviews.example.edu/school-one", "manual", "Manual collection approved by policy LEG-14", "researcher", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = domain.NewObservation(source, "mention", "Concern about admissions support", "A public comment requesting clearer admissions guidance.", "negative", nil, nil, "Thank you; our admissions team can help.", "researcher", now, now); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unapproved source accepted: %v", err)
	}
	if err = source.Review("researcher", "approved", "Looks lawful", now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("self review accepted: %v", err)
	}
	if err = source.Review("compliance", "approved", "Official page and manual collection verified", now); err != nil {
		t.Fatal(err)
	}
	item, err := domain.NewObservation(source, "mention", "Concern about admissions support", "A public comment requesting clearer admissions guidance.", "negative", nil, nil, "Thank you; our admissions team can help.", "researcher", now, now)
	if err != nil {
		t.Fatal(err)
	}
	if item.EvidenceSHA256 == "" || item.Status != domain.StatusPending {
		t.Fatalf("unexpected observation: %+v", item)
	}
	if err = item.Review("researcher", "approved", "Accurate", now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("self review accepted: %v", err)
	}
	if err = item.Review("communications-lead", "approved", "Evidence and draft checked; no automatic publication", now); err != nil {
		t.Fatal(err)
	}
	if err = item.Resolve("communications-lead", "Applicant-facing guidance was updated and the issue closed", now); err != nil {
		t.Fatal(err)
	}
	if item.Status != domain.StatusResolved {
		t.Fatalf("status=%s", item.Status)
	}
}

func TestCompetitorCategoryAndCollectionMethodConstraints(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewSource("school-one", domain.KindCompetitor, "Competitor", "https://competitor.example", "crawler", "robots allow", "analyst", now); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("uncontrolled crawler accepted: %v", err)
	}
	source, err := domain.NewSource("school-one", domain.KindCompetitor, "Competitor", "https://competitor.example", "official_api", "Official programme API terms", "analyst", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Review("legal", "approved", "API use verified", now); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.NewObservation(source, "mention", "A mention", "Evidence", "neutral", nil, nil, "", "analyst", now, now); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("reputation category accepted for competitor: %v", err)
	}
}

func TestAlertRuleAndAcknowledgementAreBoundedAndExplainable(t *testing.T) {
	now := time.Now().UTC()
	if _, err := domain.NewAlertRule("school-one", 1, 30, "admin", now); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unsafe threshold accepted: %v", err)
	}
	rule, err := domain.NewAlertRule("school-one", 3, 14, "admin", now)
	if err != nil || rule.WindowDays != 14 {
		t.Fatalf("rule=%+v err=%v", rule, err)
	}
	alert := domain.Alert{ID: "alert-1", TenantID: "school-one", Category: "misinformation", ObservationCount: 3, Threshold: 3, WindowDays: 14, Reason: domain.AlertReason("misinformation", 3, 3, 14), Status: "open", CreatedAt: now, UpdatedAt: now}
	if err = alert.Acknowledge("communications-lead", "Guidance owner assigned", now); err != nil {
		t.Fatal(err)
	}
	if alert.Status != "acknowledged" || alert.AcknowledgementNote == nil {
		t.Fatalf("alert=%+v", alert)
	}
}

func TestCompetitorSummaryBoundsEvidenceAndRequiresIndependentReview(t *testing.T) {
	now := time.Now().UTC()
	long := strings.Repeat("public programme detail ", 30)
	previous := "Earlier official evidence"
	summary, err := domain.NewCompetitorSummary("school-one", "analyst", now.Add(-7*24*time.Hour), now, now, []domain.SummaryItem{{SourceID: "source-1", Category: "fee", ChangeType: "changed", LatestTitle: "Fee updated", LatestExcerpt: long, LatestEvidenceSHA256: strings.Repeat("a", 64), LatestObservedAt: now, PreviousExcerpt: &previous}})
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString(summary.Items[0].LatestExcerpt) > 280 {
		t.Fatalf("excerpt was not bounded: %d", utf8.RuneCountInString(summary.Items[0].LatestExcerpt))
	}
	if err = summary.Review("analyst", "approved", "self review", now); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("self review accepted: %v", err)
	}
	if err = summary.Review("market-lead", "approved", "Version evidence independently checked", now); err != nil {
		t.Fatal(err)
	}
}
