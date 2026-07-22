package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/admissions-service/internal/adapters/postgres"
	"github.com/auraedu/admissions-service/internal/application"
	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

type allowVerifier struct{}

func (allowVerifier) Verify(context.Context, string, string, string) error { return nil }

func TestAdmissionsLifecycleAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	svc := application.NewService(postgres.NewRepository(database), application.WithClock(func() time.Time { return now }), application.WithDocumentVerifier(allowVerifier{}))
	applicant := auth.Actor{UserID: "applicant-1", TenantID: "school-one", Role: "applicant", Permissions: []string{application.PermCreate, application.PermUpdate, application.PermSubmit, application.PermOfferAccept}}
	manager := auth.Actor{UserID: "manager", TenantID: "school-one", Role: "school_admin", Permissions: []string{application.PermCatalogue, application.PermRead}}
	programme, err := svc.CreateProgramme(ctx, manager, application.CreateProgrammeInput{Code: "SCI", Name: "General Science", Slug: "general-science", Summary: "Science programme", Description: "Verified science programme"})
	if err != nil {
		t.Fatal(err)
	}
	intake, err := svc.CreateIntake(ctx, manager, programme.ID, application.CreateIntakeInput{Name: "September 2026", StartsAt: now.Add(60 * 24 * time.Hour), ApplicationOpensAt: now.Add(-time.Hour), ApplicationClosesAt: now.Add(30 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	published, open := domain.ProgrammePublished, domain.IntakeOpen
	if _, err = svc.UpdateProgramme(ctx, manager, programme.ID, application.UpdateProgrammeInput{Status: &published}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateIntake(ctx, manager, intake.ID, application.UpdateIntakeInput{Status: &open}); err != nil {
		t.Fatal(err)
	}
	item, err := svc.Start(ctx, applicant, nil, programme.ID, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	legal, email, phone := "Ama Mensah", "ama@example.com", "+233240000000"
	item, err = svc.Update(ctx, applicant, item.ID, application.UpdateInput{LegalName: &legal, Email: &email, Phone: &phone})
	if err != nil {
		t.Fatal(err)
	}
	item, err = svc.AttachDocument(ctx, applicant, item.ID, uuid.NewString(), "transcript", "results.pdf")
	if err != nil {
		t.Fatal(err)
	}
	item, err = svc.Submit(ctx, applicant, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	reviewer := auth.Actor{UserID: "reviewer", TenantID: "school-one", Role: "admissions_officer", Permissions: []string{application.PermReview}}
	item, err = svc.Review(ctx, reviewer, item.ID, "admitted", "Verified manually")
	if err != nil {
		t.Fatal(err)
	}
	issuer := auth.Actor{UserID: "registrar", TenantID: "school-one", Role: "registrar", Permissions: []string{application.PermOfferIssue}}
	item, err = svc.IssueOffer(ctx, issuer, item.ID, "Bring originals", now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	item, err = svc.AcceptOffer(ctx, applicant, item.ID)
	if err != nil || item.OfferStatus != "accepted" {
		t.Fatalf("accept=%+v err=%v", item, err)
	}
	outbox, err := postgres.NewRepository(database).ClaimPending(ctx, 10)
	if err != nil || len(outbox) != 9 {
		t.Fatalf("transactional outbox=%+v err=%v", outbox, err)
	}
	wantEvents := map[string]bool{"programme.created.v1": false, "intake.created.v1": false, "programme.updated.v1": false, "intake.updated.v1": false}
	for _, event := range outbox {
		if _, ok := wantEvents[event.EventType]; ok {
			wantEvents[event.EventType] = true
		}
	}
	for eventType, found := range wantEvents {
		if !found {
			t.Fatalf("missing catalogue outbox event %s: %+v", eventType, outbox)
		}
	}
	other := auth.Actor{UserID: "staff", TenantID: "school-two", Permissions: []string{application.PermRead}}
	if _, err = svc.Get(ctx, other, item.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross tenant read=%v", err)
	}
	public, err := svc.PublicProgrammes(ctx, "school-one", 50)
	if err != nil || len(public) != 1 || len(public[0].Intakes) != 1 {
		t.Fatalf("public catalogue=%+v err=%v", public, err)
	}
	otherPublic, err := svc.PublicProgrammes(ctx, "school-two", 50)
	if err != nil || len(otherPublic) != 0 {
		t.Fatalf("other public catalogue=%+v err=%v", otherPublic, err)
	}
}
