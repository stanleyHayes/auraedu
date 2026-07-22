package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestCommunicationJourneyPersistenceIsolationReplayCancellationAndFrequency(t *testing.T) {
	database := journeyTestDatabase(t)
	journeys := postgres.NewJourneyRepository(database)
	messages := postgres.NewMessageRepository(database)
	templates := postgres.NewTemplateRepository(database)
	ctx := withTenant(context.Background(), tenantA)

	template, err := domain.NewTemplate(tenantA, "Application follow-up", "email", "Application update", "Continue your application")
	if err != nil {
		t.Fatal(err)
	}
	if err := templates.Create(ctx, tenantA, template); err != nil {
		t.Fatal(err)
	}
	journey, err := domain.NewJourney(domain.NewJourneyInput{
		TenantID: tenantA, Name: "Application nurture", TriggerEvent: "application.started.v1",
		Timezone: "Africa/Accra", FrequencyWindowHours: 168, FrequencyLimit: 1,
		CancelOnEvents: []string{"application.submitted.v1"}, CreatedBy: recipientA,
		Steps: []domain.JourneyStep{{Channel: "email", TemplateID: template.ID, ConditionOperator: "always"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := journeys.CreateJourney(ctx, tenantA, journey); err != nil {
		t.Fatal(err)
	}
	if _, err := journeys.GetJourney(withTenant(context.Background(), tenantB), tenantB, journey.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant journey read err=%v", err)
	}
	if err := journey.Activate(recipientB, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := journeys.UpdateJourneyStatus(ctx, tenantA, journey, recipientB); err != nil {
		t.Fatal(err)
	}
	outbox, err := messages.ClaimPendingNotificationEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("journey audit outbox=%+v err=%v", outbox, err)
	}
	matchingOutbox := 0
	for _, item := range outbox {
		if item.EventType != "communication.journey_changed.v1" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			t.Fatalf("decode journey audit payload=%+v err=%v", payload, err)
		}
		if payload["journey_id"] != journey.ID {
			continue
		}
		matchingOutbox++
		if payload["changed_by"] == nil {
			t.Fatalf("invalid journey audit payload=%+v err=%v", payload, err)
		}
		if _, leaked := payload["name"]; leaked {
			t.Fatalf("journey name leaked to event: %+v", payload)
		}
	}
	if matchingOutbox != 2 {
		t.Fatalf("matching journey audit events=%d, want 2; claimed=%+v", matchingOutbox, outbox)
	}

	enrollmentID := uuid.NewString()
	due := time.Now().UTC().Add(-time.Minute)
	message, err := domain.NewMessage(tenantA, recipientA, "email", "Application update", "Continue your application", &template.ID,
		map[string]any{"journey_id": journey.ID, "journey_enrollment_id": enrollmentID, "lead_id": recipientA}, &due)
	if err != nil {
		t.Fatal(err)
	}
	enrollment := ports.JourneyEnrollment{ID: enrollmentID, TenantID: tenantA, JourneyID: journey.ID,
		EventID: "event-journey-1", TriggerEvent: journey.TriggerEvent, LeadID: recipientA, Messages: []*domain.Message{message}}
	created, err := journeys.EnrollJourney(ctx, enrollment)
	if err != nil || !created {
		t.Fatalf("enroll created=%v err=%v", created, err)
	}
	created, err = journeys.EnrollJourney(ctx, enrollment)
	if err != nil || created {
		t.Fatalf("replay created=%v err=%v", created, err)
	}
	stats, err := journeys.JourneyStats(ctx, tenantA, journey.ID)
	if err != nil || stats.Enrolled != 1 || stats.Scheduled != 1 {
		t.Fatalf("initial stats=%+v err=%v", stats, err)
	}

	message.MarkSent()
	if err := messages.Update(ctx, tenantA, message); err != nil {
		t.Fatal(err)
	}
	next, err := messages.NextJourneyDeliveryAllowedAt(ctx, tenantA, journey.ID, recipientA, 168*time.Hour, 1)
	if err != nil || next == nil || !next.After(time.Now()) {
		t.Fatalf("frequency next=%v err=%v", next, err)
	}

	secondEnrollmentID := uuid.NewString()
	second, err := domain.NewMessage(tenantA, recipientA, "email", "Second", "Second", &template.ID,
		map[string]any{"journey_id": journey.ID, "journey_enrollment_id": secondEnrollmentID, "lead_id": recipientA}, &due)
	if err != nil {
		t.Fatal(err)
	}
	created, err = journeys.EnrollJourney(ctx, ports.JourneyEnrollment{ID: secondEnrollmentID, TenantID: tenantA, JourneyID: journey.ID,
		EventID: "event-journey-2", TriggerEvent: journey.TriggerEvent, LeadID: recipientA, Messages: []*domain.Message{second}})
	if err != nil || !created {
		t.Fatalf("second enrollment created=%v err=%v", created, err)
	}
	cancelled, err := journeys.CancelJourneysForEvent(ctx, tenantA, recipientA, "event-submitted", "application.submitted.v1")
	if err != nil || cancelled != 1 {
		t.Fatalf("cancelled=%d err=%v", cancelled, err)
	}
	stored, err := messages.GetByID(ctx, tenantA, second.ID)
	if err != nil || stored.Status != "cancelled" || stored.Metadata["journey_cancellation_event"] != "application.submitted.v1" {
		t.Fatalf("cancelled message=%+v err=%v", stored, err)
	}
}

func journeyTestDatabase(t *testing.T) *platformdb.DB {
	t.Helper()
	if dsn := os.Getenv("AURA_NOTIFICATION_TEST_DATABASE_URL"); dsn != "" {
		database, err := platformdb.Open(context.Background(), platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
		if err != nil {
			t.Fatalf("open configured notification test database: %v", err)
		}
		t.Cleanup(database.Close)
		return database
	}
	return testkit.NewPostgres(context.Background(), t, "../../migrations").DB
}
