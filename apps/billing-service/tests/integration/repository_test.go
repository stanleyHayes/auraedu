package integration

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/billing-service/internal/adapters/postgres"
	"github.com/auraedu/billing-service/internal/application"
	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"
)

func newRepos(t *testing.T) (ports.PlanRepository, ports.SubscriptionRepository, ports.SaaSInvoiceRepository) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewPlanRepository(tdb.DB), postgres.NewSubscriptionRepository(tdb.DB), postgres.NewSaaSInvoiceRepository(tdb.DB)
}

func TestLifecycleMutationsAndOutboxAreAtomic(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	planRepo := postgres.NewPlanRepository(tdb.DB)
	subRepo := postgres.NewSubscriptionRepository(tdb.DB)
	invRepo := postgres.NewSaaSInvoiceRepository(tdb.DB)
	plan := mustCreatePlan(ctx, t, planRepo, "Durable", "durable", 1500)
	now := time.Now().UTC()
	trialEnd := now.AddDate(0, 0, 14)
	subscription, err := domain.NewSubscription(tenantA, plan.ID, now, now.AddDate(0, 1, 0), string(domain.SubscriptionStatusTrialing), &trialEnd)
	if err != nil {
		t.Fatal(err)
	}
	events := []ports.LifecycleEvent{
		{EventType: "billing.subscription_changed.v1", TenantID: tenantA, Payload: ports.SubscriptionEventData(subscription, map[string]any{"plan_key": plan.Code})},
		{EventType: "billing.trial_started.v1", TenantID: tenantA, Payload: ports.SubscriptionEventData(subscription, map[string]any{"plan_key": plan.Code, "trial_ends_at": trialEnd.Format(time.RFC3339)})},
	}
	if err := subRepo.CommitSubscriptionLifecycle(ctx, tenantA, ports.BillingMutationSubscriptionCreate, subscription, events); err != nil {
		t.Fatal(err)
	}
	items, err := subRepo.ClaimPendingBillingEvents(context.Background(), 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}

	invoice, err := domain.NewSaaSInvoice(tenantA, subscription.ID, 1500, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE billing_outbox`); err != nil {
		t.Fatal(err)
	}
	invoiceEvents := []ports.LifecycleEvent{{EventType: "billing.invoice_created.v1", TenantID: tenantA, Payload: ports.InvoiceEventData(invoice, nil)}}
	if err := invRepo.CommitInvoiceLifecycle(ctx, tenantA, ports.BillingMutationInvoiceCreate, invoice, invoiceEvents); err == nil {
		t.Fatal("expected outbox failure")
	}
	if _, err := invRepo.GetByID(ctx, tenantA, invoice.ID); err == nil {
		t.Fatal("invoice mutation must roll back when outbox insert fails")
	}
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreatePlan(ctx context.Context, t *testing.T, repo ports.PlanRepository, name, code string, priceCents int) *domain.Plan {
	t.Helper()
	p, err := domain.NewPlan(name, code, "GHS", "monthly", priceCents, nil, []string{"billing"})
	if err != nil {
		t.Fatalf("new plan: %v", err)
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return p
}

func mustCreateSubscription(ctx context.Context, t *testing.T, repo ports.SubscriptionRepository, planID string) *domain.Subscription {
	t.Helper()
	now := time.Now().UTC()
	sub, err := domain.NewSubscription(tenantA, planID, now, now.AddDate(0, 1, 0), string(domain.SubscriptionStatusActive), nil)
	if err != nil {
		t.Fatalf("new subscription: %v", err)
	}
	if err := repo.Create(ctx, tenantA, sub); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

func mustCreateInvoice(ctx context.Context, t *testing.T, repo ports.SaaSInvoiceRepository, subscriptionID string, amountCents int) *domain.SaaSInvoice {
	t.Helper()
	inv, err := domain.NewSaaSInvoice(tenantA, subscriptionID, amountCents, nil)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}
	if err := repo.Create(ctx, tenantA, inv); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return inv
}

func TestPlanRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	p := mustCreatePlan(ctx, t, repo, "Starter", "starter", 1000)

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if got.ID != p.ID || got.Code != "starter" {
		t.Fatalf("plan mismatch: %+v", got)
	}
}

func TestPlanRepository_GetByCode(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	p := mustCreatePlan(ctx, t, repo, "Starter", "STARTER", 1000)

	got, err := repo.GetByCode(ctx, "Starter")
	if err != nil {
		t.Fatalf("get plan by code: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("expected plan %s, got %s", p.ID, got.ID)
	}
}

func TestPlanRepository_UniqueCode(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	mustCreatePlan(ctx, t, repo, "Starter", "starter", 1000)
	_, err := domain.NewPlan("Starter 2", "starter", "GHS", "monthly", 2000, nil, nil)
	if err != nil {
		t.Fatalf("new plan with duplicate code: %v", err)
	}
	p2, err := domain.NewPlan("Starter 2", "starter", "GHS", "monthly", 2000, nil, nil)
	if err != nil {
		t.Fatalf("new plan with duplicate code: %v", err)
	}
	if err := repo.Create(ctx, p2); err == nil {
		t.Fatal("expected error for duplicate plan code")
	}
}

func TestPlanRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	mustCreatePlan(ctx, t, repo, "Starter", "starter", 1000)
	p2 := mustCreatePlan(ctx, t, repo, "Pro", "pro", 5000)

	page, next, err := repo.List(ctx, ports.PlanFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, ports.PlanFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != p2.ID {
		t.Fatalf("expected second plan, got %+v", page2)
	}
}

func TestService_ListPublicPlansReturnsOnlyActiveCatalogue(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subscriptionRepo, invoiceRepo := newRepos(t)
	active := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 125000)
	archived := mustCreatePlan(ctx, t, planRepo, "Legacy", "legacy", 50000)
	status := string(domain.PlanStatusArchived)
	if _, err := archived.ApplyUpdate(domain.PlanPatch{Status: &status}); err != nil {
		t.Fatal(err)
	}
	if err := planRepo.Update(ctx, archived); err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(planRepo, subscriptionRepo, invoiceRepo)
	plans, _, err := svc.ListPublicPlans(ctx, ports.PlanFilter{Limit: 100, Status: string(domain.PlanStatusArchived)})
	if err != nil {
		t.Fatalf("list public plans: %v", err)
	}
	if len(plans) != 1 || plans[0].ID != active.ID {
		t.Fatalf("public catalogue = %+v", plans)
	}
}

func TestPlanRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	p := mustCreatePlan(ctx, t, repo, "Starter", "starter", 1000)
	name := "Updated Starter"
	if _, err := p.ApplyUpdate(domain.PlanPatch{Name: &name}); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != name {
		t.Fatalf("plan not updated: %+v", got)
	}
}

func TestPlanRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _ := newRepos(t)

	p := mustCreatePlan(ctx, t, repo, "Starter", "starter", 1000)
	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, p.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSubscriptionRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, _ := newRepos(t)

	p := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	sub := mustCreateSubscription(ctx, t, subRepo, p.ID)

	got, err := subRepo.GetByID(ctx, tenantA, sub.ID)
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if got.ID != sub.ID || got.PlanID != p.ID {
		t.Fatalf("subscription mismatch: %+v", got)
	}
}

func TestSubscriptionRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, _ := newRepos(t)

	p1 := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	p2 := mustCreatePlan(ctx, t, planRepo, "Pro", "pro", 5000)
	mustCreateSubscription(ctx, t, subRepo, p1.ID)
	s2 := mustCreateSubscription(ctx, t, subRepo, p2.ID)
	s2.Status = string(domain.SubscriptionStatusPastDue)
	if err := subRepo.Update(ctx, tenantA, s2); err != nil {
		t.Fatalf("update subscription status: %v", err)
	}

	cases := []struct {
		name   string
		filter ports.SubscriptionFilter
		want   int
	}{
		{"by plan_id", ports.SubscriptionFilter{Limit: 10, PlanID: p1.ID}, 1},
		{"by status", ports.SubscriptionFilter{Limit: 10, Status: string(domain.SubscriptionStatusPastDue)}, 1},
		{"all", ports.SubscriptionFilter{Limit: 10}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := subRepo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestInvoiceRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	p := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	sub := mustCreateSubscription(ctx, t, subRepo, p.ID)
	inv := mustCreateInvoice(ctx, t, invRepo, sub.ID, 1000)

	got, err := invRepo.GetByID(ctx, tenantA, inv.ID)
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if got.ID != inv.ID || got.AmountCents != 1000 {
		t.Fatalf("invoice mismatch: %+v", got)
	}
}

func TestInvoiceRepository_StatusTransitions(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	p := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	sub := mustCreateSubscription(ctx, t, subRepo, p.ID)
	inv := mustCreateInvoice(ctx, t, invRepo, sub.ID, 1000)

	if err := inv.MarkPaid(); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if err := invRepo.Update(ctx, tenantA, inv); err != nil {
		t.Fatalf("update paid invoice: %v", err)
	}

	got, err := invRepo.GetByID(ctx, tenantA, inv.ID)
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if got.Status != string(domain.SaaSInvoiceStatusPaid) || got.PaidAt == nil {
		t.Fatalf("invoice not paid: %+v", got)
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	planRepo, subRepo, invRepo := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	p := mustCreatePlan(aCtx, t, planRepo, "Starter", "starter", 1000)
	sub := mustCreateSubscription(aCtx, t, subRepo, p.ID)
	inv := mustCreateInvoice(aCtx, t, invRepo, sub.ID, 1000)

	bCtx := withTenant(ctx, tenantB)
	if _, err := subRepo.GetByID(bCtx, tenantB, sub.ID); err == nil {
		t.Fatal("tenant B should not see tenant A subscription")
	}
	if _, err := invRepo.GetByID(bCtx, tenantB, inv.ID); err == nil {
		t.Fatal("tenant B should not see tenant A invoice")
	}

	subList, _, err := subRepo.List(bCtx, tenantB, ports.SubscriptionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B subscriptions: %v", err)
	}
	if len(subList) != 0 {
		t.Fatalf("tenant B should see 0 subscriptions, got %d", len(subList))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	planRepo, subRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureBilling, false)

	svc := application.NewService(planRepo, subRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermManage)

	_, err := svc.CreateSubscription(ctx, actor, application.CreateSubscriptionRequest{PlanID: "plan-1"})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureBilling, true)

	svc := application.NewService(planRepo, subRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	p := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	sub, err := svc.CreateSubscription(ctx, actor, application.CreateSubscriptionRequest{PlanID: p.ID})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("expected subscription id")
	}
}

func TestService_CreateSubscriptionForTenant(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	svc := application.NewService(planRepo, subRepo, invRepo)
	mustCreatePlan(ctx, t, planRepo, "Free", "free", 0)

	sub, err := svc.CreateSubscriptionForTenant(ctx, tenantA, "free")
	if err != nil {
		t.Fatalf("create subscription for tenant: %v", err)
	}
	if sub.Status != string(domain.SubscriptionStatusTrialing) {
		t.Fatalf("expected trialing status, got %q", sub.Status)
	}
	if sub.TrialEndsAt == nil {
		t.Fatal("expected trial_ends_at to be set")
	}
}

func TestService_ChangeSubscriptionPlan_Upgrades(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureBilling, true)
	svc := application.NewService(planRepo, subRepo, invRepo, application.WithFeatureGate(gates))
	starter := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	pro := mustCreatePlan(ctx, t, planRepo, "Pro", "pro", 5000)
	sub := mustCreateSubscription(ctx, t, subRepo, starter.ID)

	actor := actorWithPerms(tenantA, application.PermManage)
	updated, err := svc.ChangeSubscriptionPlan(ctx, actor, sub.ID, pro.ID)
	if err != nil {
		t.Fatalf("change plan: %v", err)
	}
	if updated.PlanID != pro.ID {
		t.Fatalf("expected plan_id %s, got %s", pro.ID, updated.PlanID)
	}
}

func TestService_CreateInvoice(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	planRepo, subRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureBilling, true)
	svc := application.NewService(planRepo, subRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	p := mustCreatePlan(ctx, t, planRepo, "Starter", "starter", 1000)
	sub, err := svc.CreateSubscription(ctx, actor, application.CreateSubscriptionRequest{PlanID: p.ID})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	inv, err := svc.CreateInvoice(ctx, actor, application.CreateInvoiceRequest{
		SubscriptionID: sub.ID,
		AmountCents:    1000,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if inv.Status != string(domain.SaaSInvoiceStatusDraft) {
		t.Fatalf("expected draft status, got %q", inv.Status)
	}

	paid, err := svc.MarkInvoicePaid(ctx, actor, inv.ID)
	if err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if paid.Status != string(domain.SaaSInvoiceStatusPaid) {
		t.Fatalf("expected paid status, got %q", paid.Status)
	}
}
