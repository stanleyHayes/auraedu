// Package adapters_test runs one contract against every tenant repository driver.
//
// Swapping persistence is only safe if both adapters behave identically, so the
// assertions live here once and each driver runs them.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/platform/testkit"
	mongoadapter "github.com/auraedu/tenant-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/tenant-service/internal/adapters/postgres"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/google/uuid"
)

type tenantRepo interface {
	ports.Repository
	ports.OutboxRepository
	ports.DurableTenantLifecycleRepository
}

// Both adapters take the tenant as an argument and scope themselves from it —
// the Postgres one sets app.tenant_id, the Mongo one binds a Scope — so the
// contract calls them with a plain context, exactly as the application does.

func newTenant(code, status string) domain.Tenant {
	t := domain.Tenant{
		Code: code, Name: "Test School " + code, Short: "TS",
		Status: status, Plan: "starter", Domain: code + ".example.test",
	}
	t.Branding.Brand.Primary = "#123456"
	return t
}

func code(prefix string) string { return prefix + uuid.NewString()[:8] }

func runTenantContract(t *testing.T, repo tenantRepo) {
	ctx := context.Background()

	t.Run("create then read back", func(t *testing.T) {
		c := code("t")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create tenant: %v", err)
		}
		got, err := repo.GetTenant(ctx, c)
		if err != nil {
			t.Fatalf("get tenant: %v", err)
		}
		if got.Code != c || got.Plan != "starter" || got.Branding.Brand.Primary != "#123456" {
			t.Fatalf("round trip changed the tenant: %+v", got)
		}
	})

	t.Run("a missing tenant is not found", func(t *testing.T) {
		if _, err := repo.GetTenant(ctx, code("missing")); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing tenant returned %v; want ErrNotFound", err)
		}
	})

	t.Run("update applies only the named fields", func(t *testing.T) {
		c := code("u")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		name := "Renamed School"
		got, err := repo.UpdateTenant(ctx, c, domain.TenantUpdate{Name: &name})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if got.Name != name || got.Plan != "starter" {
			t.Fatalf("update changed the wrong fields: %+v", got)
		}
	})

	t.Run("list spans tenants", func(t *testing.T) {
		c := code("l")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		all, err := repo.ListTenants(ctx)
		if err != nil {
			t.Fatalf("list tenants: %v", err)
		}
		found := false
		for _, tn := range all {
			if tn.Code == c {
				found = true
			}
		}
		if !found {
			t.Fatalf("the platform listing omitted %s", c)
		}
	})

	t.Run("resolve finds an active tenant by subdomain and host", func(t *testing.T) {
		c := code("r")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.ResolveTenant(ctx, "", c)
		if err != nil || got.Code != c {
			t.Fatalf("resolve by subdomain: got %+v err=%v", got, err)
		}
		got, err = repo.ResolveTenant(ctx, c+".example.test", "")
		if err != nil || got.Code != c {
			t.Fatalf("resolve by host: got %+v err=%v", got, err)
		}
	})

	t.Run("resolve ignores a tenant that is not active", func(t *testing.T) {
		c := code("p")
		if err := repo.CreateTenant(ctx, newTenant(c, "onboarding")); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.ResolveTenant(ctx, "", c); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("an inactive tenant was resolved: %v", err)
		}
	})

	t.Run("settings round trip", func(t *testing.T) {
		c := code("s")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		want := domain.Settings{Locale: "en-GH", Timezone: "Africa/Accra", AcademicYearStartMonth: 9}
		if err := repo.UpdateSettings(ctx, c, want); err != nil {
			t.Fatalf("update settings: %v", err)
		}
		got, err := repo.Settings(ctx, c)
		if err != nil {
			t.Fatalf("settings: %v", err)
		}
		if got.Locale != want.Locale || got.Timezone != want.Timezone || got.AcademicYearStartMonth != 9 {
			t.Fatalf("settings changed in storage: %+v", got)
		}
	})

	t.Run("features report the catalogue with overrides applied", func(t *testing.T) {
		c := code("f")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		before, err := repo.Features(ctx, c)
		if err != nil || len(before) == 0 {
			t.Fatalf("features: %d entries err=%v", len(before), err)
		}
		key := ""
		for _, f := range before {
			if !f.Enabled {
				key = f.Key
				break
			}
		}
		if key == "" {
			t.Skip("the starter plan enables every catalogue feature")
		}

		if _, err := repo.SetFeature(ctx, c, key, true, "enabled by contract"); err != nil {
			t.Fatalf("set feature: %v", err)
		}
		after, err := repo.Features(ctx, c)
		if err != nil {
			t.Fatalf("features: %v", err)
		}
		if len(after) != len(before) {
			t.Fatalf("the catalogue changed size: %d then %d", len(before), len(after))
		}
		for _, f := range after {
			if f.Key == key && !f.Enabled {
				t.Fatalf("the override was not applied to %s", key)
			}
		}
	})

	t.Run("an unknown feature key is refused", func(t *testing.T) {
		c := code("x")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.SetFeature(ctx, c, "not_a_real_feature", true, ""); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("an unknown feature key was accepted: %v", err)
		}
	})

	t.Run("another tenant cannot read or change a tenant's features", func(t *testing.T) {
		owner, other := code("o"), code("n")
		for _, c := range []string{owner, other} {
			if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
				t.Fatalf("create %s: %v", c, err)
			}
		}
		catalog, err := repo.Features(ctx, owner)
		if err != nil || len(catalog) == 0 {
			t.Fatalf("features: %v", err)
		}
		// Pick a flag the plan leaves off, so enabling it for one tenant is a
		// visible change rather than the default both tenants already share.
		key := ""
		for _, f := range catalog {
			if !f.Enabled {
				key = f.Key
				break
			}
		}
		if key == "" {
			t.Skip("the starter plan enables every catalogue feature")
		}
		if _, err := repo.SetFeature(ctx, owner, key, true, ""); err != nil {
			t.Fatalf("set feature: %v", err)
		}

		// The other tenant must see its own default, not the owner's override.
		theirs, err := repo.Features(ctx, other)
		if err != nil {
			t.Fatalf("features: %v", err)
		}
		for _, f := range theirs {
			if f.Key == key && f.Enabled {
				t.Fatalf("%s leaked an enabled flag to another tenant", key)
			}
		}
	})

	t.Run("a lifecycle change queues a claimable event", func(t *testing.T) {
		drainOutbox(t, repo)
		c := code("e")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}

		claimed := claimFor(t, repo, c)
		if len(claimed) != 1 || claimed[0].EventType != "tenant.created.v1" {
			t.Fatalf("claimed %d events for %s: %+v", len(claimed), c, claimed)
		}
		if err := repo.MarkPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("mark published: %v", err)
		}
		if again := claimFor(t, repo, c); len(again) != 0 {
			t.Fatalf("a published event was claimed again: %+v", again)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		drainOutbox(t, repo)
		c := code("y")
		if err := repo.CreateTenant(ctx, newTenant(c, "active")); err != nil {
			t.Fatalf("create: %v", err)
		}
		claimed := claimFor(t, repo, c)
		if len(claimed) != 1 {
			t.Fatalf("claimed %d events; want 1", len(claimed))
		}
		if err := repo.MarkFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		if err := repo.MarkPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})

	t.Run("activation happens once and is a no-op afterwards", func(t *testing.T) {
		c := code("a")
		if err := repo.CreateTenant(ctx, newTenant(c, "onboarding")); err != nil {
			t.Fatalf("create: %v", err)
		}
		changed, err := repo.ActivateOnboardingTenant(ctx, c)
		if err != nil || !changed {
			t.Fatalf("first activation: changed=%v err=%v", changed, err)
		}
		changed, err = repo.ActivateOnboardingTenant(ctx, c)
		if err != nil || changed {
			t.Fatalf("a repeat activation was not a no-op: changed=%v err=%v", changed, err)
		}
		got, err := repo.GetTenant(ctx, c)
		if err != nil || got.Status != "active" {
			t.Fatalf("tenant not active after activation: %+v err=%v", got, err)
		}
	})

	t.Run("onboarding submission is idempotent and conflicts on a changed payload", func(t *testing.T) {
		request := newOnboarding()
		hash := uuid.NewString()
		first, created, err := repo.SubmitOnboarding(ctx, request, hash, "payload-a", "fingerprint")
		if err != nil || !created {
			t.Fatalf("first submit: created=%v err=%v", created, err)
		}

		replay := newOnboarding()
		again, created, err := repo.SubmitOnboarding(ctx, replay, hash, "payload-a", "fingerprint")
		if err != nil || created {
			t.Fatalf("replay should return the stored request: created=%v err=%v", created, err)
		}
		if again.ID != first.ID {
			t.Fatalf("replay returned a different request: %s vs %s", again.ID, first.ID)
		}

		// The same key must not describe two different submissions.
		if _, _, err := repo.SubmitOnboarding(ctx, newOnboarding(), hash, "payload-b", "fingerprint"); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("a changed payload under the same key returned %v; want ErrConflict", err)
		}
	})

	t.Run("onboarding can be rejected once", func(t *testing.T) {
		request := newOnboarding()
		if _, _, err := repo.SubmitOnboarding(ctx, request, uuid.NewString(), "p", "f"); err != nil {
			t.Fatalf("submit: %v", err)
		}
		got, err := repo.RejectOnboarding(ctx, request.ID, "not eligible", "admin@auraedu.dev")
		if err != nil || got.Status != "rejected" {
			t.Fatalf("reject: %+v err=%v", got, err)
		}
		if _, err := repo.RejectOnboarding(ctx, request.ID, "again", "admin@auraedu.dev"); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("a second decision returned %v; want ErrConflict", err)
		}
	})
}

func newOnboarding() *domain.OnboardingRequest {
	return &domain.OnboardingRequest{
		ID: uuid.NewString(), SchoolName: "Test School", AdministratorName: "Ama Owusu",
		Email: uuid.NewString() + "@example.test", CountryCode: "GH", Plan: "starter",
		PrivacyNoticeVersion: "2026-01", Status: "pending_review",
		SubmittedAt: time.Now().UTC().Truncate(time.Second),
	}
}

// claimFor drains claimable events and keeps only the named tenant's, because
// the outbox is shared across tenants within the service.
func claimFor(t *testing.T, repo tenantRepo, tenantCode string) []ports.OutboxEvent {
	t.Helper()
	all, err := repo.ClaimPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	var mine []ports.OutboxEvent
	for _, e := range all {
		if e.TenantID == tenantCode {
			mine = append(mine, e)
		}
	}
	return mine
}

func drainOutbox(t *testing.T, repo tenantRepo) {
	t.Helper()
	ctx := context.Background()
	for range 50 {
		claimed, err := repo.ClaimPending(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresTenantRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runTenantContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoTenantRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runTenantContract(t, mongoadapter.NewRepository(mg.Store))
}
