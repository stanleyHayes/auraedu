package unit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

const (
	journeyCreator  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	journeyReviewer = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	journeyLead     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

type fakeJourneyRepo struct {
	journeys    map[string]*domain.Journey
	enrollments map[string]ports.JourneyEnrollment
	messages    *fakeMessageRepo
}

func newFakeJourneyRepo(messages *fakeMessageRepo) *fakeJourneyRepo {
	return &fakeJourneyRepo{journeys: map[string]*domain.Journey{}, enrollments: map[string]ports.JourneyEnrollment{}, messages: messages}
}

func (r *fakeJourneyRepo) CreateJourney(_ context.Context, tenantID string, journey *domain.Journey) error {
	r.journeys[tenantID+"/"+journey.ID] = journey
	return nil
}

func (r *fakeJourneyRepo) GetJourney(_ context.Context, tenantID, id string) (*domain.Journey, error) {
	journey, ok := r.journeys[tenantID+"/"+id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return journey, nil
}

func (r *fakeJourneyRepo) ListJourneys(_ context.Context, tenantID string, filter ports.JourneyFilter) ([]*domain.Journey, error) {
	var result []*domain.Journey
	for _, journey := range r.journeys {
		if journey.TenantID == tenantID && (filter.Status == "" || journey.Status == filter.Status) &&
			(filter.TriggerEvent == "" || journey.TriggerEvent == filter.TriggerEvent) {
			result = append(result, journey)
		}
	}
	return result, nil
}

func (r *fakeJourneyRepo) UpdateJourneyStatus(_ context.Context, tenantID string, journey *domain.Journey, _ string) error {
	r.journeys[tenantID+"/"+journey.ID] = journey
	return nil
}

func (r *fakeJourneyRepo) ListActiveJourneysByTrigger(ctx context.Context, tenantID, eventType string) ([]*domain.Journey, error) {
	return r.ListJourneys(ctx, tenantID, ports.JourneyFilter{Status: "active", TriggerEvent: eventType})
}

func (r *fakeJourneyRepo) EnrollJourney(ctx context.Context, enrollment ports.JourneyEnrollment) (bool, error) {
	key := enrollment.TenantID + "/" + enrollment.JourneyID + "/" + enrollment.EventID
	if _, exists := r.enrollments[key]; exists {
		return false, nil
	}
	r.enrollments[key] = enrollment
	for _, message := range enrollment.Messages {
		if err := r.messages.Create(ctx, enrollment.TenantID, message); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *fakeJourneyRepo) CancelJourneysForEvent(_ context.Context, tenantID, leadID, eventID, eventType string) (int64, error) {
	var cancelled int64
	for key, enrollment := range r.enrollments {
		journey := r.journeys[tenantID+"/"+enrollment.JourneyID]
		if enrollment.TenantID != tenantID || enrollment.LeadID != leadID || journey == nil {
			continue
		}
		matches := false
		for _, cancelEvent := range journey.CancelOnEvents {
			matches = matches || cancelEvent == eventType
		}
		if !matches {
			continue
		}
		for _, message := range enrollment.Messages {
			if message.Status == "pending" {
				message.MarkCancelled(eventType)
				cancelled++
			}
		}
		delete(r.enrollments, key)
	}
	return cancelled, nil
}

func (r *fakeJourneyRepo) FinalizeJourneyEnrollment(context.Context, string, string) error {
	return nil
}
func (r *fakeJourneyRepo) JourneyStats(context.Context, string, string) (ports.JourneyStats, error) {
	return ports.JourneyStats{}, nil
}

type fakeJourneyTemplateRepo struct{ templates map[string]*domain.Template }

func (r *fakeJourneyTemplateRepo) Create(_ context.Context, tenantID string, template *domain.Template) error {
	r.templates[tenantID+"/"+template.ID] = template
	return nil
}
func (r *fakeJourneyTemplateRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Template, error) {
	template, ok := r.templates[tenantID+"/"+id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return template, nil
}
func (r *fakeJourneyTemplateRepo) List(context.Context, string, ports.TemplateFilter) ([]*domain.Template, string, error) {
	return nil, "", nil
}
func (r *fakeJourneyTemplateRepo) Update(context.Context, string, *domain.Template) error { return nil }
func (r *fakeJourneyTemplateRepo) Delete(context.Context, string, string) error           { return nil }

func journeyFixture(t *testing.T) (*application.Service, *fakeJourneyRepo, *fakeMessageRepo, string, context.Context) {
	t.Helper()
	messages := newFakeMessageRepo()
	templates := &fakeJourneyTemplateRepo{templates: map[string]*domain.Template{}}
	template, err := domain.NewTemplate(workerTenant, "Application started", "email", "Hello {{first_name}}", "Continue the {{programme_id}} application.")
	if err != nil {
		t.Fatal(err)
	}
	templates.templates[workerTenant+"/"+template.ID] = template
	journeys := newFakeJourneyRepo(messages)
	gates := flags.NewStaticSnapshot()
	for _, feature := range []string{"notifications", "growth_crm", "email_notifications", "sms_notifications", "whatsapp_notifications", "growth_whatsapp"} {
		gates.Set(workerTenant, feature, true)
	}
	svc := application.NewService(messages, templates, newFakeSubscriptionRepo(),
		application.WithJourneyRepository(journeys),
		application.WithFeatureGate(gates),
		application.WithLeadResolver(welcomeResolver{recipient: ports.LeadWelcomeRecipient{
			Email: "ama@example.com", FirstName: "Ama", EmailEligible: true,
		}}),
		application.WithNotifiers(notifier.Registry()),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: workerTenant})
	return svc, journeys, messages, template.ID, ctx
}

func journeyActor(userID string) auth.Actor {
	return auth.Actor{UserID: userID, TenantID: workerTenant, Permissions: []string{application.PermRead, application.PermManage}}
}

func TestCommunicationJourneyRequiresIndependentActivation(t *testing.T) {
	svc, _, _, templateID, ctx := journeyFixture(t)
	journey, err := svc.CreateJourney(ctx, journeyActor(journeyCreator), application.CreateJourneyRequest{
		Name: "Application nurture", TriggerEvent: "application.started.v1", Timezone: "Africa/Accra",
		FrequencyWindowHours: 168, FrequencyLimit: 3, CancelOnEvents: []string{"application.submitted.v1"},
		Steps: []domain.JourneyStep{{Channel: "email", TemplateID: templateID, ConditionOperator: "always"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.ActivateJourney(ctx, journeyActor(journeyCreator), journey.ID); err == nil {
		t.Fatal("creator activated their own journey")
	}
	activated, err := svc.ActivateJourney(ctx, journeyActor(journeyReviewer), journey.ID)
	if err != nil || activated.Status != "active" {
		t.Fatalf("independent activation: journey=%+v err=%v", activated, err)
	}
}

func TestCommunicationJourneyEnrollmentIsReplaySafeAndCancellable(t *testing.T) {
	svc, journeys, messages, templateID, ctx := journeyFixture(t)
	journey, err := svc.CreateJourney(ctx, journeyActor(journeyCreator), application.CreateJourneyRequest{
		Name: "Application nurture", TriggerEvent: "application.started.v1", Timezone: "Africa/Accra",
		FrequencyWindowHours: 168, FrequencyLimit: 3, CancelOnEvents: []string{"application.submitted.v1"},
		Steps: []domain.JourneyStep{{Channel: "email", TemplateID: templateID, ConditionOperator: "always"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ActivateJourney(ctx, journeyActor(journeyReviewer), journey.ID); err != nil {
		t.Fatal(err)
	}
	event := tenancy.CloudEvent{ID: "journey-event-1", Type: "application.started.v1", TenantID: workerTenant,
		Data: mustJSON(t, map[string]any{"lead_id": journeyLead, "programme_id": "dddddddd-dddd-dddd-dddd-dddddddddddd"})}
	if err := svc.HandleJourneyEvent(ctx, event); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := svc.HandleJourneyEvent(ctx, event); err != nil {
		t.Fatalf("replay: %v", err)
	}
	got := messages.all(workerTenant)
	if len(got) != 1 || got[0].Body != "Continue the dddddddd-dddd-dddd-dddd-dddddddddddd application." || got[0].Metadata["delivery_address"] != "ama@example.com" {
		t.Fatalf("unexpected scheduled journey message: %+v", got)
	}
	if len(journeys.enrollments) != 1 {
		t.Fatalf("duplicate enrollment: %+v", journeys.enrollments)
	}
	cancel := tenancy.CloudEvent{ID: "journey-event-2", Type: "application.submitted.v1", TenantID: workerTenant,
		Data: mustJSON(t, map[string]any{"lead_id": journeyLead})}
	if err := svc.HandleJourneyEvent(ctx, cancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got[0].Status != "cancelled" {
		t.Fatalf("pending step was not cancelled: %+v", got[0])
	}
}

func TestCommunicationJourneyRechecksConsentBeforeDelivery(t *testing.T) {
	svc, journeys, messages, templateID, ctx := journeyFixture(t)
	journey, err := svc.CreateJourney(ctx, journeyActor(journeyCreator), application.CreateJourneyRequest{
		Name: "Consent recheck", TriggerEvent: "application.started.v1", Timezone: "Africa/Accra",
		FrequencyWindowHours: 168, FrequencyLimit: 3,
		Steps: []domain.JourneyStep{{Channel: "email", TemplateID: templateID, ConditionOperator: "always"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ActivateJourney(ctx, journeyActor(journeyReviewer), journey.ID); err != nil {
		t.Fatal(err)
	}
	event := tenancy.CloudEvent{ID: "journey-event-consent", Type: "application.started.v1", TenantID: workerTenant,
		Data: mustJSON(t, map[string]any{"lead_id": journeyLead, "programme_id": uuid.NewString()})}
	if err := svc.HandleJourneyEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	message := messages.all(workerTenant)[0]
	gate := funcGate(true)
	withoutConsent := application.NewService(messages, nil, nil,
		application.WithJourneyRepository(journeys), application.WithFeatureGate(gate),
		application.WithLeadResolver(welcomeResolver{recipient: ports.LeadWelcomeRecipient{Email: "ama@example.com"}}),
	)
	if err := withoutConsent.DeliverScheduled(ctx, message); err != nil {
		t.Fatalf("consent cancellation: %v", err)
	}
	if message.Status != "cancelled" || message.Error == nil {
		t.Fatalf("revoked consent did not stop delivery: %+v", message)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
