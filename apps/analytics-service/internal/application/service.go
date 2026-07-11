// Package application holds the analytics-service use cases.
package application

import (
	"context"
	"fmt"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// PermRead is the analytics view permission key.
const PermRead = "analytics.view"

// FeatureAnalytics is the feature flag key for analytics dashboards.
const FeatureAnalytics = "analytics"

// Service holds the analytics query use cases. Tenant scope + RBAC + feature-flag
// checks belong here, never in HTTP handlers.
type Service struct {
	repo  ports.Repository
	gates flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// NewService constructs the application service.
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, gates: flags.NewStaticSnapshot()}
	for _, o := range opts {
		o(s)
	}
	return s
}

// List returns a tenant-scoped page of metrics, optionally filtered.
func (s *Service) List(ctx context.Context, actor auth.Actor, filter ports.ListFilter) ([]*domain.Metric, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListMetrics(ctx, tenantID, filter)
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureAnalytics) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureAnalytics)
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
