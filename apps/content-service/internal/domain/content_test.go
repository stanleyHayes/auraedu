package domain

import (
	"errors"
	"testing"
	"time"
)

func testProfile(t *testing.T, now time.Time) BrandProfile {
	t.Helper()
	profile, err := NewBrandProfile(BrandProfileInput{TenantID: "school-a", ToneOfVoice: "Warm, factual and encouraging", Locale: "en-GH", UpdatedBy: "admin", RequiredDisclaimers: []string{"Terms apply"}, ProhibitedClaims: []string{"guaranteed admission"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func testDraft(t *testing.T, now time.Time, content string) Draft {
	t.Helper()
	profile := testProfile(t, now)
	draft, err := NewDraft(DraftInput{TenantID: "school-a", ContentType: "social_post", Title: "Open day", Brief: "Create an invitation for our next open day.", Audience: "Prospective families", Locale: "en-GH", KeyMessages: []string{"Meet our teachers"}, Facts: []Fact{{Label: "Date", Value: "30 August"}}, Content: content, Generator: "test:model", BrandProfileVersion: 1, CreatedBy: "author"}, profile, now)
	if err != nil {
		t.Fatal(err)
	}
	return draft
}

func TestComplianceBlocksProhibitedClaimAndMissingDisclaimer(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	draft := testDraft(t, now, "We offer guaranteed admission.")
	if draft.ComplianceStatus != ComplianceFail || len(draft.ComplianceFindings) != 2 {
		t.Fatalf("expected fail with two findings, got %s %#v", draft.ComplianceStatus, draft.ComplianceFindings)
	}
	if err := draft.Submit("author", 1, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("noncompliant draft must not submit, got %v", err)
	}
}

func TestFourEyesApprovalAndImmutableRevision(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	draft := testDraft(t, now, "Meet our teachers on 30 August. Terms apply")
	if err := draft.Submit("author", 1, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := draft.Review("author", "Looks good", 1, true, now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("self approval must fail, got %v", err)
	}
	if err := draft.Review("reviewer", "Facts and brand rules verified", 1, true, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := draft.Revise("author", "Changed copy. Terms apply", "New campaign angle", 1, nil, testProfile(t, now), now.Add(3*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("approved version must not edit in place, got %v", err)
	}
}

func TestRejectedContentCanBecomeNewVersion(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	draft := testDraft(t, now, "Meet our teachers on 30 August. Terms apply")
	if err := draft.Submit("author", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := draft.Review("reviewer", "Use a more welcoming opening", 1, false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := draft.Revise("author", "You are warmly invited to meet our teachers. Terms apply", "Applied reviewer feedback", 1, nil, testProfile(t, now), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if draft.Version != 2 || draft.Status != StatusDraft || draft.ReviewedBy != nil {
		t.Fatalf("expected clean v2 draft, got %#v", draft)
	}
}
