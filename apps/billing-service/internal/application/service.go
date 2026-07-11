// Package application implements the billing use cases and RBAC policy.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead   = "billing.read"
	PermManage = "billing.manage"
)

// FeatureBilling is the feature flag key for billing management.
const FeatureBilling = "billing"

// Service holds the billing use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	planRepo ports.PlanRepository
	subRepo  ports.SubscriptionRepository
	invRepo  ports.SaaSInvoiceRepository
	pub      ports.EventPublisher
	gates    flags.Gate
	now      func() time.Time
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithClock sets the clock function (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

type noopPublisher struct{}

func (noopPublisher) PublishSubscription(context.Context, string, *domain.Subscription, map[string]any) error {
	return nil
}

func (noopPublisher) PublishPlan(context.Context, string, *domain.Plan, map[string]any) error {
	return nil
}

func (noopPublisher) PublishInvoice(context.Context, string, *domain.SaaSInvoice, map[string]any) error {
	return nil
}

// NewService constructs the application service.
func NewService(planRepo ports.PlanRepository, subRepo ports.SubscriptionRepository, invRepo ports.SaaSInvoiceRepository, opts ...Option) *Service {
	s := &Service{
		planRepo: planRepo,
		subRepo:  subRepo,
		invRepo:  invRepo,
		pub:      noopPublisher{},
		gates:    flags.NewStaticSnapshot(),
		now:      time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- Plan use cases ---

// CreatePlanRequest is the input for creating a plan.
type CreatePlanRequest struct {
	Name            string
	Code            string
	Description     *string
	PriceCents      int
	Currency        string
	BillingInterval string
	Features        []string
}

// UpdatePlanRequest is the input for patching a plan.
type UpdatePlanRequest struct {
	Name            *string
	Code            *string
	Description     *string
	PriceCents      *int
	Currency        *string
	BillingInterval *string
	Features        *[]string
	Status          *string
}

const platformTenantID = "00000000-0000-0000-0000-000000000000"

// CreatePlan validates and persists a new Plan.
func (s *Service) CreatePlan(ctx context.Context, actor auth.Actor, req CreatePlanRequest) (*domain.Plan, error) {
	if err := s.requirePlanManage(actor); err != nil {
		return nil, err
	}
	p, err := domain.NewPlan(req.Name, req.Code, req.Currency, req.BillingInterval, req.PriceCents, req.Description, req.Features)
	if err != nil {
		return nil, err
	}
	if err := s.planRepo.Create(withPlatformTenant(ctx), p); err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, err
	}
	return p, nil
}

// ListPlans returns a page of plans.
func (s *Service) ListPlans(ctx context.Context, actor auth.Actor, filter ports.PlanFilter) ([]*domain.Plan, string, error) {
	if !actor.Authenticated() || !actor.Has(PermRead) {
		return nil, "", domain.ErrForbidden
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.planRepo.List(withPlatformTenant(ctx), filter)
}

// GetPlan returns a single plan.
func (s *Service) GetPlan(ctx context.Context, actor auth.Actor, id string) (*domain.Plan, error) {
	if !actor.Authenticated() || !actor.Has(PermRead) {
		return nil, domain.ErrForbidden
	}
	return s.planRepo.GetByID(withPlatformTenant(ctx), id)
}

// UpdatePlan patches a plan.
func (s *Service) UpdatePlan(ctx context.Context, actor auth.Actor, id string, req UpdatePlanRequest) (*domain.Plan, error) {
	if err := s.requirePlanManage(actor); err != nil {
		return nil, err
	}
	p, err := s.planRepo.GetByID(withPlatformTenant(ctx), id)
	if err != nil {
		return nil, err
	}
	_, err = p.ApplyUpdate(domain.PlanPatch{
		Name:            req.Name,
		Code:            req.Code,
		Description:     req.Description,
		PriceCents:      req.PriceCents,
		Currency:        req.Currency,
		BillingInterval: req.BillingInterval,
		Features:        req.Features,
		Status:          req.Status,
	})
	if err != nil {
		return nil, err
	}
	if err := s.planRepo.Update(withPlatformTenant(ctx), p); err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrConflict
		}
		return nil, err
	}
	return p, nil
}

// DeletePlan removes a plan.
func (s *Service) DeletePlan(ctx context.Context, actor auth.Actor, id string) error {
	if err := s.requirePlanManage(actor); err != nil {
		return err
	}
	if _, err := s.planRepo.GetByID(withPlatformTenant(ctx), id); err != nil {
		return err
	}
	return s.planRepo.Delete(withPlatformTenant(ctx), id)
}

// --- Subscription use cases ---

// CreateSubscriptionRequest is the input for creating a subscription.
type CreateSubscriptionRequest struct {
	PlanID             string
	Status             string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	TrialEndsAt        *time.Time
}

// UpdateSubscriptionRequest is the input for patching a subscription.
type UpdateSubscriptionRequest struct {
	Status             *string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	TrialEndsAt        *time.Time
	CancelledAt        *time.Time
}

// CreateSubscription validates and persists a new Subscription.
func (s *Service) CreateSubscription(ctx context.Context, actor auth.Actor, req CreateSubscriptionRequest) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	periodStart := now
	if req.CurrentPeriodStart != nil {
		periodStart = *req.CurrentPeriodStart
	}
	periodEnd := periodStart.AddDate(0, 1, 0)
	if req.CurrentPeriodEnd != nil {
		periodEnd = *req.CurrentPeriodEnd
	}
	status := req.Status
	if status == "" {
		status = string(domain.SubscriptionStatusActive)
	}
	sub, err := domain.NewSubscription(tenantID, req.PlanID, periodStart, periodEnd, status, req.TrialEndsAt)
	if err != nil {
		return nil, err
	}
	if err := s.subRepo.Create(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// CreateSubscriptionForTenant creates a trial subscription for a tenant from a plan code.
func (s *Service) CreateSubscriptionForTenant(ctx context.Context, tenantID, planCode string) (*domain.Subscription, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, domain.ErrMissingTenant
	}
	plan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	trialEndsAt := now.AddDate(0, 0, 14)
	var periodEnd time.Time
	if plan.BillingInterval == string(domain.BillingIntervalYearly) {
		periodEnd = now.AddDate(1, 0, 0)
	} else {
		periodEnd = now.AddDate(0, 1, 0)
	}
	if trialEndsAt.After(periodEnd) {
		periodEnd = trialEndsAt
	}
	sub, err := domain.NewSubscription(tenantID, plan.ID, now, periodEnd, string(domain.SubscriptionStatusTrialing), &trialEndsAt)
	if err != nil {
		return nil, err
	}
	if err := s.subRepo.Create(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	if err := s.pub.PublishSubscription(ctx, "billing.subscription_changed.v1", sub, map[string]any{
		"tenant_id": tenantID,
		"plan_key":  plan.Code,
		"status":    sub.Status,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish subscription changed event", "err", err)
	}
	if err := s.pub.PublishSubscription(ctx, "billing.trial_started.v1", sub, map[string]any{
		"tenant_id":     tenantID,
		"plan_key":      plan.Code,
		"trial_ends_at": trialEndsAt.Format(time.RFC3339),
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish trial started event", "err", err)
	}
	return sub, nil
}

// ListSubscriptions returns a tenant-scoped page of subscriptions.
func (s *Service) ListSubscriptions(ctx context.Context, actor auth.Actor, filter ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.subRepo.List(ctx, tenantID, filter)
}

// GetSubscription returns a single subscription.
func (s *Service) GetSubscription(ctx context.Context, actor auth.Actor, id string) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.subRepo.GetByID(ctx, tenantID, id)
}

// UpdateSubscription patches a subscription.
func (s *Service) UpdateSubscription(ctx context.Context, actor auth.Actor, id string, req UpdateSubscriptionRequest) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	sub, err := s.subRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	_, err = sub.ApplyUpdate(domain.SubscriptionPatch{
		Status:             req.Status,
		CurrentPeriodStart: req.CurrentPeriodStart,
		CurrentPeriodEnd:   req.CurrentPeriodEnd,
		TrialEndsAt:        req.TrialEndsAt,
		CancelledAt:        req.CancelledAt,
	})
	if err != nil {
		return nil, err
	}
	if err := s.subRepo.Update(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// ChangeSubscriptionPlan updates the plan on a subscription and emits domain events.
func (s *Service) ChangeSubscriptionPlan(ctx context.Context, actor auth.Actor, subscriptionID, newPlanID string) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	sub, err := s.subRepo.GetByID(ctx, tenantID, subscriptionID)
	if err != nil {
		return nil, err
	}
	currentPlan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}
	newPlan, err := s.planRepo.GetByID(ctx, newPlanID)
	if err != nil {
		return nil, err
	}
	previousPlanCode := currentPlan.Code
	if err := sub.ChangePlan(newPlanID); err != nil {
		return nil, err
	}
	if err := s.subRepo.Update(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	if err := s.pub.PublishSubscription(ctx, "billing.subscription_changed.v1", sub, map[string]any{
		"tenant_id": tenantID,
		"plan_key":  newPlan.Code,
		"status":    sub.Status,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish subscription changed event", "err", err)
	}
	if newPlan.PriceCents > currentPlan.PriceCents {
		if err := s.pub.PublishPlan(ctx, "billing.plan_upgraded.v1", newPlan, map[string]any{
			"tenant_id":     tenantID,
			"previous_plan": previousPlanCode,
		}); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish plan upgraded event", "err", err)
		}
	}
	return sub, nil
}

// DeleteSubscription removes a subscription.
func (s *Service) DeleteSubscription(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.subRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.subRepo.Delete(ctx, tenantID, id)
}

// --- SaaSInvoice use cases ---

// CreateInvoiceRequest is the input for creating an invoice.
type CreateInvoiceRequest struct {
	SubscriptionID string
	AmountCents    int
	DueDate        *time.Time
}

// UpdateInvoiceRequest is the input for patching an invoice.
type UpdateInvoiceRequest struct {
	AmountCents *int
	Status      *string
	DueDate     *time.Time
	PaidAt      *time.Time
}

// CreateInvoice validates and persists a new SaaSInvoice.
func (s *Service) CreateInvoice(ctx context.Context, actor auth.Actor, req CreateInvoiceRequest) (*domain.SaaSInvoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	sub, err := s.subRepo.GetByID(ctx, tenantID, req.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("%w: subscription not found", domain.ErrValidation)
	}
	inv, err := domain.NewSaaSInvoice(tenantID, sub.ID, req.AmountCents, req.DueDate)
	if err != nil {
		return nil, err
	}
	if err := s.invRepo.Create(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	if err := s.pub.PublishInvoice(ctx, "billing.invoice_created.v1", inv, map[string]any{
		"tenant_id": tenantID,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish invoice created event", "err", err)
	}
	return inv, nil
}

// ListInvoices returns a tenant-scoped page of invoices.
func (s *Service) ListInvoices(ctx context.Context, actor auth.Actor, filter ports.SaaSInvoiceFilter) ([]*domain.SaaSInvoice, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.invRepo.List(ctx, tenantID, filter)
}

// GetInvoice returns a single invoice.
func (s *Service) GetInvoice(ctx context.Context, actor auth.Actor, id string) (*domain.SaaSInvoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.invRepo.GetByID(ctx, tenantID, id)
}

// UpdateInvoice patches an invoice.
func (s *Service) UpdateInvoice(ctx context.Context, actor auth.Actor, id string, req UpdateInvoiceRequest) (*domain.SaaSInvoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	_, err = inv.ApplyUpdate(domain.SaaSInvoicePatch{
		AmountCents: req.AmountCents,
		Status:      req.Status,
		DueDate:     req.DueDate,
		PaidAt:      req.PaidAt,
	})
	if err != nil {
		return nil, err
	}
	if err := s.invRepo.Update(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// MarkInvoicePaid transitions an invoice to paid.
func (s *Service) MarkInvoicePaid(ctx context.Context, actor auth.Actor, id string) (*domain.SaaSInvoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := inv.MarkPaid(); err != nil {
		return nil, err
	}
	if err := s.invRepo.Update(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// MarkInvoiceVoid transitions an invoice to void.
func (s *Service) MarkInvoiceVoid(ctx context.Context, actor auth.Actor, id string) (*domain.SaaSInvoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := inv.MarkVoid(); err != nil {
		return nil, err
	}
	if err := s.invRepo.Update(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// DeleteInvoice removes an invoice.
func (s *Service) DeleteInvoice(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.invRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.invRepo.Delete(ctx, tenantID, id)
}

// --- Worker helpers ---

// CreateTrialSubscriptionForTenant finds the default active plan and creates a
// trial subscription for the tenant. It is used by the worker on tenant.created.v1.
func (s *Service) CreateTrialSubscriptionForTenant(ctx context.Context, tenantID string) (*domain.Subscription, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, domain.ErrMissingTenant
	}
	defaultPlan, err := s.findDefaultPlan(ctx)
	if err != nil {
		return nil, err
	}
	return s.CreateSubscriptionForTenant(ctx, tenantID, defaultPlan.Code)
}

func (s *Service) findDefaultPlan(ctx context.Context) (*domain.Plan, error) {
	plans, _, err := s.planRepo.List(ctx, ports.PlanFilter{Limit: 1000})
	if err != nil {
		return nil, err
	}
	var active []*domain.Plan
	for _, p := range plans {
		if p.Status == string(domain.PlanStatusActive) {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return nil, domain.ErrNoDefaultPlan
	}
	for _, p := range active {
		if p.Code == "free" {
			return p, nil
		}
	}
	cheapest := active[0]
	for _, p := range active {
		if p.PriceCents < cheapest.PriceCents {
			cheapest = p
		}
	}
	return cheapest, nil
}

// --- access helpers ---

func (s *Service) requirePlanManage(actor auth.Actor) error {
	if !actor.Authenticated() {
		return domain.ErrForbidden
	}
	if !actor.Has(PermManage) {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) {
		return "", domain.ErrForbidden
	}
	if !actor.Has(perm) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureBilling) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureBilling)
	}
	return tenantID, nil
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, domain.ErrConflict) || contains(err.Error(), "unique") || contains(err.Error(), "duplicate")
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func withPlatformTenant(ctx context.Context) context.Context {
	if tenancy.TenantID(ctx) == "" {
		return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: platformTenantID})
	}
	return ctx
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
