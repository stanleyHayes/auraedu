package workercmd

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/tenant-service/internal/adapters/events"
	"github.com/auraedu/tenant-service/internal/adapters/postgres"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

func TestLiveTenantLifecycleOutboxPublishesOnce(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		t.Skip("NATS_URL is required for live outbox proof")
	}
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)

	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		t.Fatalf("ensure stream: %v", err)
	}
	subscription, err := js.SubscribeSync("AURA.tenant.>", nats.DeliverNew(), nats.AckExplicit())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() {
		if err := subscription.Unsubscribe(); err != nil {
			t.Errorf("unsubscribe: %v", err)
		}
	})

	unique := uuid.NewString()[:8]
	tenantCode := "outbox-smoke-" + unique
	input := application.SubmitOnboardingInput{
		SchoolName: "Outbox Smoke Academy", AdministratorName: "Test Operator",
		Email: "outbox-" + unique + "@example.test", CountryCode: "GH", Plan: "growth",
		PrivacyNoticeVersion: "2026-07-19", AcceptedTerms: true,
	}
	service := application.NewService(repo)
	request, created, err := service.SubmitOnboarding(ctx, "outbox-smoke-"+uuid.NewString(), input)
	if err != nil || !created {
		t.Fatalf("submit: created=%v err=%v", created, err)
	}
	admin := auth.Actor{UserID: "platform-smoke", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	if _, err := service.ApproveOnboarding(ctx, admin, request.ID, tenantCode); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := service.ActivateOnboardingTenant(ctx, tenantCode); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := service.ActivateOnboardingTenant(ctx, tenantCode); err != nil {
		t.Fatalf("idempotent activation: %v", err)
	}
	updatedName := "Outbox Smoke Academy International"
	if _, err := service.UpdateTenant(ctx, admin, tenantCode, domain.TenantUpdate{Name: &updatedName}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := service.UpdateSettings(ctx, admin, tenantCode, domain.Settings{
		Locale: "en-GH", Timezone: "Africa/Accra", DateFormat: "DD/MM/YYYY",
		AcademicYearStartMonth: 9, PrimaryContactEmail: input.Email,
	}); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if _, err := service.SetFeature(ctx, admin, tenantCode, "analytics", false); err != nil {
		t.Fatalf("feature: %v", err)
	}
	if err := service.DeleteTenant(ctx, admin, tenantCode); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var retainedStatus string
	var retainedTenantCode *string
	if err := database.Pool().QueryRow(ctx, `
		SELECT status, tenant_code FROM onboarding_requests WHERE id = $1
	`, request.ID).Scan(&retainedStatus, &retainedTenantCode); err != nil {
		t.Fatalf("read retained onboarding decision: %v", err)
	}
	if retainedStatus != "approved" || retainedTenantCode != nil {
		t.Fatalf("deleted tenant must preserve the approved decision without a stale reference: status=%q tenant=%v", retainedStatus, retainedTenantCode)
	}

	publisher := events.NewPublisher(eventbus.NewPublisher(js))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics := observ.NewWorkerMetrics("tenant-service-worker-live-test", "outbox-batch", "outbox-publish")
	if err := dispatch(ctx, repo, publisher, logger, metrics); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	types := map[string]bool{}
	for len(types) < 7 {
		message, err := subscription.NextMsg(5 * time.Second)
		if err != nil {
			t.Fatalf("receive tenant lifecycle events: %v (types=%v)", err, types)
		}
		var event map[string]any
		if err := json.Unmarshal(message.Data, &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if event["tenant_id"] != tenantCode {
			if err := message.Ack(); err != nil {
				t.Fatalf("ack unrelated event: %v", err)
			}
			continue
		}
		typeName, ok := event["type"].(string)
		if !ok || typeName == "" {
			t.Fatalf("event type is invalid: %v", event["type"])
		}
		types[typeName] = true
		data, ok := event["data"].(map[string]any)
		if !ok {
			t.Fatalf("event data is invalid: %v", event["data"])
		}
		for _, forbidden := range []string{"email", "phone", "administrator_name", "primary_contact_email", "domain", "logo_url"} {
			if _, found := data[forbidden]; found {
				t.Fatalf("event %s leaked %s", typeName, forbidden)
			}
		}
		if message.Header.Get("Nats-Msg-Id") == "" || event["id"] != message.Header.Get("Nats-Msg-Id") {
			t.Fatalf("event id and JetStream idempotency key differ: event=%v header=%q", event["id"], message.Header.Get("Nats-Msg-Id"))
		}
		if err := message.Ack(); err != nil {
			t.Fatalf("ack: %v", err)
		}
	}
	for _, eventType := range []string{
		"tenant.created.v1", "tenant.onboarding_approved.v1", "tenant.activated.v1",
		"tenant.updated.v1", "tenant.settings_updated.v1", "tenant.feature_disabled.v1",
		"tenant.deleted.v1",
	} {
		if !types[eventType] {
			t.Fatalf("missing %s in types=%v", eventType, types)
		}
	}

	if err := dispatch(ctx, repo, publisher, logger, metrics); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	pending, err := repo.ClaimPending(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("published outbox must be drained: pending=%+v err=%v", pending, err)
	}
}
