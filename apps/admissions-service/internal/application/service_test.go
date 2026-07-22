package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/admissions-service/internal/adapters/memory"
	"github.com/auraedu/admissions-service/internal/application"
	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/google/uuid"
)

func actor(id, role string, perms ...string) auth.Actor {
	return auth.Actor{UserID: id, TenantID: "school-one", Role: role, Permissions: perms}
}

func openCatalogue(t *testing.T, svc *application.Service, now time.Time) (string, string) {
	t.Helper()
	manager := actor("admissions-manager", "school_admin", application.PermCatalogue, application.PermRead)
	programme, err := svc.CreateProgramme(context.Background(), manager, application.CreateProgrammeInput{
		Code: "SCI", Name: "General Science", Slug: "general-science", Summary: "Science programme", Description: "A verified science programme.",
	})
	if err != nil {
		t.Fatal(err)
	}
	intake, err := svc.CreateIntake(context.Background(), manager, programme.ID, application.CreateIntakeInput{
		Name: "September 2026", StartsAt: now.Add(60 * 24 * time.Hour), ApplicationOpensAt: now.Add(-time.Hour), ApplicationClosesAt: now.Add(30 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	published := domain.ProgrammePublished
	if _, err = svc.UpdateProgramme(context.Background(), manager, programme.ID, application.UpdateProgrammeInput{Status: &published}); err != nil {
		t.Fatal(err)
	}
	open := domain.IntakeOpen
	if _, err = svc.UpdateIntake(context.Background(), manager, intake.ID, application.UpdateIntakeInput{Status: &open}); err != nil {
		t.Fatal(err)
	}
	return programme.ID, intake.ID
}

type allowVerifier struct{}

func (allowVerifier) Verify(context.Context, string, string, string) error { return nil }

type denyVerifier struct{ err error }

func (d denyVerifier) Verify(context.Context, string, string, string) error { return d.err }

func TestApplicationChecklistHumanDecisionAndOffer(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }), application.WithDocumentVerifier(allowVerifier{}))
	applicant := actor("applicant-1", "applicant", application.PermCreate, application.PermUpdate, application.PermSubmit, application.PermOfferAccept)
	programmeID, intakeID := openCatalogue(t, svc, now)
	item, err := svc.Start(context.Background(), applicant, nil, programmeID, intakeID)
	if err != nil {
		t.Fatal(err)
	}
	legal, email, phone := "Ama Mensah", "ama@example.com", "+233240000000"
	item, err = svc.Update(context.Background(), applicant, item.ID, application.UpdateInput{LegalName: &legal, Email: &email, Phone: &phone, Answers: map[string]any{"qualification": "WASSCE"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Submit(context.Background(), applicant, item.ID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("incomplete submit=%v", err)
	}
	item, err = svc.AttachDocument(context.Background(), applicant, item.ID, uuid.NewString(), "transcript", "transcript.pdf")
	if err != nil || item.CompletionPercentage != 100 {
		t.Fatalf("attach=%+v err=%v", item, err)
	}
	item, err = svc.Submit(context.Background(), applicant, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	ai := actor("ai-1", "ai_service_account", application.PermReview)
	if _, err = svc.Review(context.Background(), ai, item.ID, "admitted", "Automated decision"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AI decision=%v", err)
	}
	reviewer := actor("reviewer", "admissions_officer", application.PermReview)
	item, err = svc.Review(context.Background(), reviewer, item.ID, "admitted", "Requirements verified")
	if err != nil || item.Status != domain.StatusAdmitted {
		t.Fatalf("review=%+v err=%v", item, err)
	}
	issuer := actor("registrar", "registrar", application.PermOfferIssue)
	item, err = svc.IssueOffer(context.Background(), issuer, item.ID, "Submit originals at enrolment", now.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	item, err = svc.AcceptOffer(context.Background(), applicant, item.ID)
	if err != nil || item.OfferStatus != "accepted" {
		t.Fatalf("accept=%+v err=%v", item, err)
	}
}
func TestAttachDocumentFailsClosedWhenVerificationFails(t *testing.T) {
	applicant := actor("applicant-1", "applicant", application.PermCreate, application.PermUpdate)
	for name, verifier := range map[string]application.Option{
		"unconfigured": application.WithDocumentVerifier(nil),
		"rejected":     application.WithDocumentVerifier(denyVerifier{err: domain.ErrForbidden}),
	} {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			svc := application.NewService(memory.New(), verifier, application.WithClock(func() time.Time { return now }))
			programmeID, intakeID := openCatalogue(t, svc, now)
			item, err := svc.Start(context.Background(), applicant, nil, programmeID, intakeID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.AttachDocument(context.Background(), applicant, item.ID, uuid.NewString(), "transcript", "transcript.pdf")
			if err == nil {
				t.Fatal("expected verification failure")
			}
		})
	}
}
func TestApplicantOwnership(t *testing.T) {
	now := time.Now().UTC()
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	owner := actor("owner", "applicant", application.PermCreate)
	programmeID, intakeID := openCatalogue(t, svc, now)
	item, err := svc.Start(context.Background(), owner, nil, programmeID, intakeID)
	if err != nil {
		t.Fatal(err)
	}
	other := actor("other", "applicant")
	if _, err = svc.Get(context.Background(), other, item.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross applicant get=%v", err)
	}
}

func TestCatalogueVisibilityAvailabilityAndHumanManagement(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	programmeID, intakeID := openCatalogue(t, svc, now)

	public, err := svc.PublicProgrammes(context.Background(), "school-one", 50)
	if err != nil || len(public) != 1 || public[0].ID != programmeID || len(public[0].Intakes) != 1 || public[0].Intakes[0].ID != intakeID {
		t.Fatalf("public catalogue=%+v err=%v", public, err)
	}
	if other, err := svc.PublicProgrammes(context.Background(), "school-two", 50); err != nil || len(other) != 0 {
		t.Fatalf("cross-tenant public catalogue=%+v err=%v", other, err)
	}
	ai := actor("ai-1", "ai_service_account", application.PermCatalogue)
	if _, err := svc.CreateProgramme(context.Background(), ai, application.CreateProgrammeInput{Code: "ART", Name: "Arts", Slug: "arts", Summary: "Arts programme", Description: "Arts programme description"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("AI catalogue mutation=%v", err)
	}
	applicant := actor("applicant-2", "applicant", application.PermCreate)
	if _, err := svc.Start(context.Background(), applicant, nil, uuid.NewString(), intakeID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unpublished programme start=%v", err)
	}
}
