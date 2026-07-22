package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/knowledge-service/internal/adapters/memory"
	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func TestKnowledgeRequiresReviewAndStrictTenantRetrieval(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	repo := memory.New()
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	manager := auth.Actor{UserID: "manager-1", TenantID: "school-one", Permissions: []string{application.PermManage, application.PermRead}}
	reviewer := auth.Actor{UserID: "reviewer-1", TenantID: "school-one", Permissions: []string{application.PermApprove}}

	source, err := svc.Create(context.Background(), manager, application.CreateInput{
		SourceType: "fees", Title: "2026 Undergraduate Fees", Owner: "Admissions Office",
		Content:         "The application fee is GHS 250 and must be paid through the official applicant portal.",
		Confidentiality: "public", EffectiveAt: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	results, err := svc.SearchApproved(context.Background(), "school-one", "application fee", "en", 5, now)
	if err != nil || len(results) != 0 {
		t.Fatalf("draft leaked into retrieval: results=%+v err=%v", results, err)
	}
	approved, err := svc.Approve(context.Background(), reviewer, source.ID, "Verified against the signed fee schedule")
	if err != nil || approved.Status != domain.StatusApproved {
		t.Fatalf("approve: source=%+v err=%v", approved, err)
	}
	results, err = svc.SearchApproved(context.Background(), "school-one", "application fee", "en-GH", 5, now)
	if err != nil || len(results) != 1 || results[0].SourceID != source.ID || results[0].Title == "" {
		t.Fatalf("approved source not cited: results=%+v err=%v", results, err)
	}
	other, err := svc.SearchApproved(context.Background(), "school-two", "application fee", "en", 5, now)
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-tenant retrieval: results=%+v err=%v", other, err)
	}
	if _, err := svc.Retire(context.Background(), reviewer, source.ID); err != nil {
		t.Fatalf("retire: %v", err)
	}
	results, err = svc.SearchApproved(context.Background(), "school-one", "application fee", "en", 5, now)
	if err != nil || len(results) != 0 {
		t.Fatalf("retired source leaked: results=%+v err=%v", results, err)
	}
}

func TestKnowledgePermissionsAndLifecycleFailClosed(t *testing.T) {
	now := time.Now().UTC()
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	input := application.CreateInput{SourceType: "faq", Title: "Admissions FAQ", Owner: "Admissions",
		Content:         "Applications close on the published deadline in the official admissions calendar.",
		Confidentiality: "public", EffectiveAt: now}
	if _, err := svc.Create(context.Background(), auth.Actor{TenantID: "school-one"}, input); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("create without permission: %v", err)
	}
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Permissions: []string{application.PermManage}}
	source, err := svc.Create(context.Background(), manager, input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Approve(context.Background(), manager, source.ID, "Looks good"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self approval without permission: %v", err)
	}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	if _, err := svc.Retire(context.Background(), reviewer, source.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("retired draft should conflict: %v", err)
	}
}

func TestInternalAndExpiredSourcesNeverReachPublicRetrieval(t *testing.T) {
	now := time.Now().UTC()
	repo := memory.New()
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Permissions: []string{application.PermManage}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	for _, tc := range []struct {
		confidentiality string
		expires         *time.Time
	}{{"internal", nil}, {"public", timePtr(now.Add(-time.Minute))}} {
		source, err := svc.Create(context.Background(), manager, application.CreateInput{SourceType: "policy", Title: "Admissions Policy",
			Owner: "Registry", Content: "Applicants must submit the complete official admissions form before review.",
			Confidentiality: tc.confidentiality, EffectiveAt: now.Add(-time.Hour), ExpiresAt: tc.expires})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := svc.Approve(context.Background(), reviewer, source.ID, "Policy version verified"); err != nil {
			t.Fatalf("approve: %v", err)
		}
	}
	results, err := svc.SearchApproved(context.Background(), "school-one", "official admissions form", "en", 5, now)
	if err != nil || len(results) != 0 {
		t.Fatalf("non-public source leaked: results=%+v err=%v", results, err)
	}
}

func TestKnowledgeRetrievalNeverCrossesLanguage(t *testing.T) {
	now := time.Now().UTC()
	repo := memory.New()
	svc := application.NewService(repo, application.WithClock(func() time.Time { return now }))
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Permissions: []string{application.PermManage}}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Permissions: []string{application.PermApprove}}
	for _, input := range []application.CreateInput{
		{SourceType: "fees", Title: "Application fee", Owner: "Admissions", Content: "The verified application fee is two hundred cedis.", Confidentiality: "public", Locale: "en", EffectiveAt: now},
		{SourceType: "fees", Title: "Frais de candidature", Owner: "Admissions", Content: "Les frais de candidature verifies sont de deux cents cedis.", Confidentiality: "public", Locale: "fr", EffectiveAt: now},
	} {
		source, err := svc.Create(context.Background(), manager, input)
		if err != nil {
			t.Fatalf("create %s: %v", input.Locale, err)
		}
		if _, err := svc.Approve(context.Background(), reviewer, source.ID, "Language and fee verified"); err != nil {
			t.Fatalf("approve %s: %v", input.Locale, err)
		}
	}
	english, err := svc.SearchApproved(context.Background(), "school-one", "application fee", "en-GH", 5, now)
	if err != nil || len(english) != 1 || english[0].Locale != "en" {
		t.Fatalf("english retrieval: results=%+v err=%v", english, err)
	}
	french, err := svc.SearchApproved(context.Background(), "school-one", "frais candidature", "fr", 5, now)
	if err != nil || len(french) != 1 || french[0].Locale != "fr" {
		t.Fatalf("french retrieval: results=%+v err=%v", french, err)
	}
	if _, err := svc.SearchApproved(context.Background(), "school-one", "application fee", "english", 5, now); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid locale should fail validation: %v", err)
	}
}

func timePtr(value time.Time) *time.Time { return &value }
