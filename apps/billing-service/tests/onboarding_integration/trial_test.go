package onboardingintegration

import (
	"context"
	"os"
	"testing"

	"github.com/auraedu/billing-service/internal/adapters/postgres"
	"github.com/auraedu/billing-service/internal/application"
	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
)

func TestRequestedPlanTrialIsIdempotent(t *testing.T) {
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

	planRepo := postgres.NewPlanRepository(database)
	subRepo := postgres.NewSubscriptionRepository(database)
	invRepo := postgres.NewSaaSInvoiceRepository(database)
	service := application.NewService(planRepo, subRepo, invRepo)
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "onboarding-school"})
	plan, err := domain.NewPlan("Growth", "growth", "GHS", string(domain.BillingIntervalMonthly), 0, nil, []string{"growth_crm"})
	if err != nil {
		t.Fatalf("new plan: %v", err)
	}
	if err := planRepo.Create(ctx, plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	first, err := service.CreateSubscriptionForTenant(ctx, "onboarding-school", "growth")
	if err != nil {
		t.Fatalf("first trial: %v", err)
	}
	replay, err := service.CreateSubscriptionForTenant(ctx, "onboarding-school", "growth")
	if err != nil {
		t.Fatalf("replayed trial: %v", err)
	}
	if replay.ID != first.ID {
		t.Fatalf("replay created another trial: first=%s replay=%s", first.ID, replay.ID)
	}
	trials, _, err := subRepo.List(ctx, "onboarding-school", ports.SubscriptionFilter{Limit: 10, Status: string(domain.SubscriptionStatusTrialing)})
	if err != nil || len(trials) != 1 {
		t.Fatalf("trials=%d err=%v", len(trials), err)
	}
}
