package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/tenant-service/internal/adapters/postgres"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

type countingPublisher struct{ calls int }

type integrationTXT struct{ records map[string][]string }

func (r *integrationTXT) LookupTXT(_ context.Context, name string) ([]string, error) {
	return r.records[name], nil
}

func (p *countingPublisher) Publish(context.Context, string, string, map[string]any) error {
	p.calls++
	return nil
}

func onboardingInput(email string) application.SubmitOnboardingInput {
	priorities := "Attendance, fees and parent communication"
	return application.SubmitOnboardingInput{
		SchoolName: "Integration Academy", AdministratorName: "Kojo Owusu",
		Email: email, CountryCode: "GH", Plan: "professional", Priorities: &priorities,
		PrivacyNoticeVersion: "2026-07-18", AcceptedTerms: true,
	}
}

func TestOnboardingPersistenceIdempotencyAndAtomicProvisioning(t *testing.T) {
	ctx := context.Background()
	var database *platformdb.DB
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var err error
		database, err = platformdb.Open(ctx, platformdb.Config{DSN: dsn, Migrations: "../../migrations"})
		if err != nil {
			t.Fatalf("open test database: %v", err)
		}
		t.Cleanup(database.Close)
	} else {
		database = testkit.NewPostgres(ctx, t, "../../migrations").DB
	}
	repo := postgres.NewRepository(database)
	svc := application.NewService(repo)

	request, created, err := svc.SubmitOnboarding(ctx, "integration-onboarding-0001", onboardingInput("admin@integration.example"))
	if err != nil || !created {
		t.Fatalf("submit: created=%v err=%v", created, err)
	}
	replay, created, err := svc.SubmitOnboarding(ctx, "integration-onboarding-0001", onboardingInput("admin@integration.example"))
	if err != nil || created || replay.ID != request.ID {
		t.Fatalf("idempotent replay: request=%+v created=%v err=%v", replay, created, err)
	}
	duplicateEmail, created, err := svc.SubmitOnboarding(ctx, "integration-onboarding-0002", onboardingInput("ADMIN@integration.example"))
	if err != nil || created || duplicateEmail.ID != request.ID {
		t.Fatalf("pending email dedupe: request=%+v created=%v err=%v", duplicateEmail, created, err)
	}

	admin := auth.Actor{UserID: "platform-1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	approved, err := svc.ApproveOnboarding(ctx, admin, request.ID, "integration-academy")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != domain.OnboardingApproved {
		t.Fatalf("status = %q, want approved", approved.Status)
	}
	outbox, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim onboarding outbox: %v", err)
	}
	if len(outbox) != 2 {
		t.Fatalf("durable onboarding events = %+v", outbox)
	}
	eventTypes := map[string]bool{}
	for _, event := range outbox {
		eventTypes[event.EventType] = true
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode %s payload: %v", event.EventType, err)
		}
		for _, forbidden := range []string{"email", "phone", "administrator_name"} {
			if _, found := payload[forbidden]; found {
				t.Fatalf("outbox event %s leaked %s", event.EventType, forbidden)
			}
		}
		if err := repo.MarkPublished(ctx, event.ID); err != nil {
			t.Fatalf("mark %s published: %v", event.EventType, err)
		}
	}
	if !eventTypes["tenant.created.v1"] || !eventTypes["tenant.onboarding_approved.v1"] {
		t.Fatalf("durable onboarding event types = %v", eventTypes)
	}
	tenant, err := svc.GetTenant(ctx, admin, "integration-academy")
	if err != nil || tenant.Plan != "professional" || tenant.Status != "onboarding" {
		t.Fatalf("tenant = %+v err=%v", tenant, err)
	}
	if err := svc.ActivateOnboardingTenant(ctx, "integration-academy"); err != nil {
		t.Fatalf("activate tenant: %v", err)
	}
	if err := svc.ActivateOnboardingTenant(ctx, "integration-academy"); err != nil {
		t.Fatalf("idempotent activation: %v", err)
	}
	activationEvents, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim activation outbox: %v", err)
	}
	if len(activationEvents) != 1 || activationEvents[0].EventType != "tenant.activated.v1" {
		t.Fatalf("activation must enqueue exactly once: %+v", activationEvents)
	}
	if err := repo.MarkPublished(ctx, activationEvents[0].ID); err != nil {
		t.Fatalf("mark activation published: %v", err)
	}
	tenant, err = svc.GetTenant(ctx, admin, "integration-academy")
	if err != nil || tenant.Status != "active" {
		t.Fatalf("activated tenant = %+v err=%v", tenant, err)
	}
	settings, err := svc.Settings(ctx, admin, "integration-academy")
	if err != nil || settings.PrimaryContactEmail != "admin@integration.example" {
		t.Fatalf("settings = %+v err=%v", settings, err)
	}
	requests, _, err := svc.ListOnboarding(ctx, admin, 25, "", domain.OnboardingApproved)
	if err != nil || len(requests) != 1 || requests[0].ID != request.ID {
		t.Fatalf("approved queue = %+v err=%v", requests, err)
	}
}

func TestTenantLifecycleMutationsAreAtomicAndDurable(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)
	direct := &countingPublisher{}
	svc := application.NewService(repo, application.WithPublisher(direct))
	admin := auth.Actor{UserID: "platform-1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	code := "lifecycle-academy"

	if _, err := svc.CreateTenant(ctx, admin, domain.Tenant{
		Code: code, Name: "Lifecycle Academy", Short: "Lifecycle", Plan: "professional",
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	name := "Lifecycle Academy International"
	if _, err := svc.UpdateTenant(ctx, admin, code, domain.TenantUpdate{Name: &name}); err != nil {
		t.Fatalf("update tenant: %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, admin, code, domain.Settings{
		Locale: "en-GH", Timezone: "Africa/Accra", DateFormat: "DD/MM/YYYY",
		AcademicYearStartMonth: 9, PrimaryContactEmail: "private@lifecycle.example",
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if _, err := svc.SetFeature(ctx, admin, code, "analytics", false); err != nil {
		t.Fatalf("disable feature: %v", err)
	}
	if err := svc.DeleteTenant(ctx, admin, code); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if direct.calls != 0 {
		t.Fatalf("postgres lifecycle must not use best-effort publisher, calls=%d", direct.calls)
	}

	events, err := repo.ClaimPending(ctx, 20)
	if err != nil {
		t.Fatalf("claim lifecycle outbox: %v", err)
	}
	want := map[string]int{
		"tenant.created.v1":          1,
		"tenant.updated.v1":          1,
		"tenant.settings_updated.v1": 1,
		"tenant.feature_disabled.v1": 1,
		"tenant.deleted.v1":          1,
	}
	got := map[string]int{}
	for _, event := range events {
		got[event.EventType]++
		if event.TenantID != code {
			t.Fatalf("event tenant=%q, want %q", event.TenantID, code)
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode %s: %v", event.EventType, err)
		}
		for _, forbidden := range []string{"primary_contact_email", "email", "logo_url", "domain"} {
			if _, found := payload[forbidden]; found {
				t.Fatalf("event %s leaked %s", event.EventType, forbidden)
			}
		}
		if err := repo.MarkPublished(ctx, event.ID); err != nil {
			t.Fatalf("mark %s published: %v", event.EventType, err)
		}
	}
	if len(events) != len(want) {
		t.Fatalf("lifecycle events=%d (%v), want %d (%v)", len(events), got, len(want), want)
	}
	for eventType, count := range want {
		if got[eventType] != count {
			t.Fatalf("event %s count=%d, want %d (all=%v)", eventType, got[eventType], count, got)
		}
	}

	// A missing outbox must fail the write, proving state cannot commit without
	// its integration event. This database is isolated to the test container.
	if _, err := database.Pool().Exec(ctx, `DROP TABLE tenant_outbox`); err != nil {
		t.Fatalf("drop outbox for atomicity probe: %v", err)
	}
	rollbackCode := "rollback-academy"
	if _, err := svc.CreateTenant(ctx, admin, domain.Tenant{Code: rollbackCode, Name: "Rollback Academy", Plan: "growth"}); err == nil {
		t.Fatal("create must fail when its outbox write cannot commit")
	}
	if _, err := svc.GetTenant(ctx, admin, rollbackCode); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tenant mutation committed without event: %v", err)
	}
}

func TestVerifiedCustomDomainLifecycleIsTenantSafeAndDurable(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	repo := postgres.NewRepository(database)
	resolver := &integrationTXT{records: map[string][]string{}}
	svc := application.NewService(repo, application.WithTXTResolver(resolver), application.WithDomainTokenGenerator(func() (string, error) {
		return strings.Repeat("b", 64), nil
	}))
	platform := auth.Actor{UserID: "platform-1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	owner := auth.Actor{UserID: "owner-1", TenantID: "domain-academy", Role: "school_admin", Permissions: []string{"features.manage"}}

	if _, err := svc.CreateTenant(ctx, platform, domain.Tenant{Code: "domain-academy", Name: "Domain Academy", Plan: "professional"}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	registration, err := svc.RequestCustomDomain(ctx, owner, "domain-academy", "WWW.Domain-Academy.edu.gh.")
	if err != nil {
		t.Fatalf("request domain: %v", err)
	}
	if registration.Hostname != "www.domain-academy.edu.gh" || registration.VerificationToken == "" {
		t.Fatalf("registration = %+v", registration)
	}
	var challengeHash string
	if err := database.Pool().QueryRow(ctx, `SELECT challenge_hash FROM tenant_custom_domains WHERE tenant_code = 'domain-academy'`).Scan(&challengeHash); err != nil {
		t.Fatalf("read challenge hash: %v", err)
	}
	if len(challengeHash) != 64 || challengeHash == registration.VerificationToken || strings.Contains(challengeHash, strings.Repeat("b", 16)) {
		t.Fatalf("challenge was not stored as an opaque hash: %q", challengeHash)
	}
	if _, err := svc.ResolveTenant(ctx, registration.Hostname, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("pending domain resolved publicly: %v", err)
	}
	resolver.records[registration.TXTRecordName] = []string{registration.VerificationToken}
	if _, err := svc.VerifyCustomDomain(ctx, owner, "domain-academy"); err != nil {
		t.Fatalf("verify domain: %v", err)
	}
	if _, err := svc.ActivateCustomDomain(ctx, owner, "domain-academy", "render-domain-verified"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("tenant owner activated TLS state: %v", err)
	}
	active, err := svc.ActivateCustomDomain(ctx, platform, "domain-academy", "render-domain-verified")
	if err != nil || active.Status != domain.DomainActive {
		t.Fatalf("activate domain: %+v err=%v", active, err)
	}
	resolved, err := svc.ResolveTenant(ctx, registration.Hostname, "")
	if err != nil || resolved.Code != "domain-academy" {
		t.Fatalf("resolve active domain: %+v err=%v", resolved, err)
	}
	inactive, err := svc.DeactivateCustomDomain(ctx, platform, "domain-academy", "render-domain-removed")
	if err != nil || inactive.Status != domain.DomainInactive || inactive.DeactivatedAt == nil {
		t.Fatalf("deactivate domain: %+v err=%v", inactive, err)
	}
	if _, err := svc.ResolveTenant(ctx, registration.Hostname, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("inactive domain resolved publicly: %v", err)
	}
	events, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim events: %v", err)
	}
	counts := map[string]int{}
	for _, event := range events {
		counts[event.EventType]++
	}
	if counts["tenant.custom_domain_activated.v1"] != 1 {
		t.Fatalf("custom domain activation event counts=%v", counts)
	}
	if counts["tenant.custom_domain_deactivated.v1"] != 1 {
		t.Fatalf("custom domain deactivation event counts=%v", counts)
	}
}

func TestTenantPostgresRLSIsolation(t *testing.T) {
	ctx := context.Background()
	database := testkit.NewPostgres(ctx, t, "../../migrations").DB
	_, err := database.Pool().Exec(ctx, `
		CREATE ROLE tenant_runtime NOLOGIN;
		GRANT SELECT, INSERT ON tenants, tenant_features, tenant_outbox TO tenant_runtime;
		INSERT INTO tenants (code, name, status, plan) VALUES
			('upshs', 'UPSHS', 'active', 'professional'),
			('aboom-ame-zion-c', 'Aboom', 'active', 'growth');
		INSERT INTO tenant_features (tenant_code, feature_key, is_enabled) VALUES
			('upshs', 'student_management', true),
			('aboom-ame-zion-c', 'student_management', true);
		INSERT INTO tenant_outbox (id, tenant_id, event_type, payload) VALUES
			('11111111-1111-4111-8111-111111111111', 'upshs', 'tenant.created.v1', '{}'),
			('22222222-2222-4222-8222-222222222222', 'aboom-ame-zion-c', 'tenant.created.v1', '{}');
	`)
	if err != nil {
		t.Fatalf("seed tenant RLS probe: %v", err)
	}

	for _, table := range []string{"tenants", "tenant_features", "tenant_outbox"} {
		var enabled, forced bool
		if err := database.Pool().QueryRow(ctx, `
			SELECT relrowsecurity, relforcerowsecurity
			FROM pg_class WHERE oid = $1::regclass
		`, table).Scan(&enabled, &forced); err != nil {
			t.Fatalf("inspect %s RLS: %v", table, err)
		}
		if !enabled || !forced {
			t.Fatalf("expected enabled and forced RLS for %s", table)
		}
	}

	service := application.NewService(postgres.NewRepository(database))
	admin := auth.Actor{UserID: "platform", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	all, err := service.ListTenants(ctx, admin)
	if err != nil || len(all) != 2 {
		t.Fatalf("platform admin list through forced RLS: tenants=%+v err=%v", all, err)
	}

	tx, err := database.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin tenant runtime probe: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE tenant_runtime"); err != nil {
		t.Fatalf("set runtime role: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', 'upshs', true)"); err != nil {
		t.Fatalf("set runtime tenant: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.is_platform_admin', 'false', true)"); err != nil {
		t.Fatalf("set runtime privilege: %v", err)
	}
	var tenantCount, featureCount, outboxCount int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM tenants),
		       (SELECT count(*) FROM tenant_features),
		       (SELECT count(*) FROM tenant_outbox)
	`).Scan(&tenantCount, &featureCount, &outboxCount); err != nil {
		t.Fatalf("query runtime tenant rows: %v", err)
	}
	if tenantCount != 1 || featureCount != 1 || outboxCount != 1 {
		t.Fatalf("expected one tenant, feature and outbox row for UPSHS, got %d, %d and %d", tenantCount, featureCount, outboxCount)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT forbidden_tenant_write`); err != nil {
		t.Fatalf("savepoint tenant write: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenants (code, name, status, plan)
		VALUES ('forbidden-school', 'Forbidden', 'active', 'growth')
	`); err == nil {
		t.Fatal("expected cross-tenant root write to be denied by RLS")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT forbidden_tenant_write`); err != nil {
		t.Fatalf("rollback tenant write: %v", err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT forbidden_outbox_write`); err != nil {
		t.Fatalf("savepoint outbox write: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_outbox (id, tenant_id, event_type, payload)
		VALUES ('33333333-3333-4333-8333-333333333333', 'aboom-ame-zion-c', 'tenant.created.v1', '{}')
	`); err == nil {
		t.Fatal("expected cross-tenant outbox write to be denied by RLS")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT forbidden_outbox_write`); err != nil {
		t.Fatalf("rollback outbox write: %v", err)
	}
}
