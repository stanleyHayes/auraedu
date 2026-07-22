package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

type recordedOnboardingEvent struct {
	EventType string
	Payload   map[string]any
}

type onboardingPublisher struct{ Events []recordedOnboardingEvent }

func (p *onboardingPublisher) Publish(_ context.Context, eventType, _ string, payload map[string]any) error {
	p.Events = append(p.Events, recordedOnboardingEvent{EventType: eventType, Payload: payload})
	return nil
}

func validOnboarding() application.SubmitOnboardingInput {
	phone := "+233200000000"
	priorities := "Attendance and parent communication"
	return application.SubmitOnboardingInput{
		SchoolName: "Production Readiness Academy", AdministratorName: "Ama Mensah",
		Email: "admin@readiness.example", Phone: &phone, CountryCode: "GH",
		Plan: "growth", Priorities: &priorities, PrivacyNoticeVersion: "2026-07-18",
		AcceptedTerms: true,
	}
}

func TestSubmitOnboardingIsIdempotentAndRejectsConflictingReplay(t *testing.T) {
	svc := application.NewService(memory.New())
	first, created, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000001", validOnboarding())
	if err != nil || !created {
		t.Fatalf("first submit: created=%v err=%v", created, err)
	}
	replay, created, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000001", validOnboarding())
	if err != nil || created || replay.ID != first.ID {
		t.Fatalf("replay: got=%+v created=%v err=%v", replay, created, err)
	}
	changed := validOnboarding()
	changed.Plan = "professional"
	if _, _, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000001", changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflicting replay, got %v", err)
	}
}

func TestApproveOnboardingRequiresPlatformAdminAndPublishesPrivacySafeEvents(t *testing.T) {
	repo := memory.New()
	pub := &onboardingPublisher{}
	svc := application.NewService(repo, application.WithPublisher(pub))
	request, _, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000002", validOnboarding())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.ApproveOnboarding(context.Background(), auth.Actor{}, request.ID, "readiness-academy"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("anonymous approval should be forbidden, got %v", err)
	}
	admin := auth.Actor{UserID: "platform-1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	approved, err := svc.ApproveOnboarding(context.Background(), admin, request.ID, "readiness-academy")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != domain.OnboardingApproved || approved.TenantCode == nil || *approved.TenantCode != "readiness-academy" {
		t.Fatalf("unexpected approved request: %+v", approved)
	}
	created, err := svc.GetTenant(context.Background(), admin, "readiness-academy")
	if err != nil || created.Status != "onboarding" || created.Plan != "growth" {
		t.Fatalf("created tenant: %+v err=%v", created, err)
	}
	if len(pub.Events) != 2 || pub.Events[0].EventType != "tenant.created.v1" || pub.Events[1].EventType != "tenant.onboarding_approved.v1" {
		t.Fatalf("unexpected events: %+v", pub.Events)
	}
	for _, event := range pub.Events {
		for _, forbidden := range []string{"email", "administrator_name", "phone"} {
			if _, found := event.Payload[forbidden]; found {
				t.Fatalf("event %s leaked %s", event.EventType, forbidden)
			}
		}
	}
}

func TestActivateOnboardingTenantIsIdempotent(t *testing.T) {
	repo := memory.New()
	pub := &onboardingPublisher{}
	svc := application.NewService(repo, application.WithPublisher(pub))

	if err := svc.ActivateOnboardingTenant(context.Background(), "cape-coast-prep"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := svc.ActivateOnboardingTenant(context.Background(), "cape-coast-prep"); err != nil {
		t.Fatalf("idempotent activation: %v", err)
	}
	admin := auth.Actor{PlatformAdmin: true}
	tenant, err := svc.GetTenant(context.Background(), admin, "cape-coast-prep")
	if err != nil || tenant.Status != "active" {
		t.Fatalf("tenant after activation: %+v err=%v", tenant, err)
	}
	if len(pub.Events) != 1 || pub.Events[0].EventType != "tenant.activated.v1" {
		t.Fatalf("activation should publish exactly once: %+v", pub.Events)
	}
}

func TestSubmitOnboardingValidationAndHoneypot(t *testing.T) {
	svc := application.NewService(memory.New())
	invalid := validOnboarding()
	invalid.AcceptedTerms = false
	if _, _, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000003", invalid); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("terms must be required, got %v", err)
	}
	bot := validOnboarding()
	bot.Website = "https://spam.invalid"
	if _, _, err := svc.SubmitOnboarding(context.Background(), "onboarding-key-00000004", bot); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("honeypot must be rejected, got %v", err)
	}
}
