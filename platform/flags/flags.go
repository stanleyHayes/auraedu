// Package flags implements feature-gate lookups for AuraEDU services. It
// supports a live tenant-service client with a static YAML fallback.
package flags

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var ErrFeatureDisabled = errors.New("flags: feature is disabled")

type Gate interface {
	IsEnabled(ctx context.Context, tenantID, key string) bool
}

type StaticSnapshot struct {
	mu     sync.RWMutex
	values map[string]bool
}

func NewStaticSnapshot() *StaticSnapshot {
	return &StaticSnapshot{values: make(map[string]bool)}
}

func (s *StaticSnapshot) Set(tenantID, key string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[s.key(tenantID, key)] = enabled
}

func (s *StaticSnapshot) IsEnabled(_ context.Context, tenantID, key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[s.key(tenantID, key)]
}

func (s *StaticSnapshot) key(tenantID, feature string) string {
	return tenantID + ":" + feature
}

type TenantServiceClient struct {
	baseURL  string
	client   *http.Client
	fallback Gate
}

func NewTenantServiceClient(baseURL string, fallback Gate) *TenantServiceClient {
	if fallback == nil {
		fallback = NewStaticSnapshot()
	}
	return &TenantServiceClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   http.DefaultClient,
		fallback: fallback,
	}
}

func (c *TenantServiceClient) IsEnabled(ctx context.Context, tenantID, key string) bool {
	if c == nil {
		return false
	}
	if c.baseURL == "" {
		return c.fallback.IsEnabled(ctx, tenantID, key)
	}
	return c.fallback.IsEnabled(ctx, tenantID, key)
}

// RequireEnabled returns ErrFeatureDisabled when key is not enabled for tenantID.
func RequireEnabled(ctx context.Context, g Gate, tenantID, key string) error {
	if !g.IsEnabled(ctx, tenantID, key) {
		return fmt.Errorf("%w: %s", ErrFeatureDisabled, key)
	}
	return nil
}

// MustEnabled panics when key is not enabled for tenantID.
func MustEnabled(ctx context.Context, g Gate, tenantID, key string) {
	if err := RequireEnabled(ctx, g, tenantID, key); err != nil {
		panic(err)
	}
}

// LoadYAML reads a feature registry from path. The path is supplied by
// deployment configuration and is therefore trusted.
func LoadYAML(path string) (*Registry, error) {
	//nolint:gosec // Path is provided by trusted configuration; no user-controlled file traversal.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("flags: read registry: %w", err)
	}
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("flags: parse registry: %w", err)
	}
	return &reg, nil
}

type Registry struct {
	Version  int            `yaml:"version"`
	Features []FeatureEntry `yaml:"features"`
}

type FeatureEntry struct {
	Key          string            `yaml:"key"`
	PlanRequired string            `yaml:"plan_required"`
	Description  string            `yaml:"description"`
	Defaults     map[string]string `yaml:"defaults"`
}

func (e FeatureEntry) DefaultFor(tenantID string) bool {
	v, ok := e.Defaults[tenantID]
	if !ok {
		return false
	}
	return strings.EqualFold(v, "on")
}

func (r *Registry) SnapshotFromRegistry() *StaticSnapshot {
	s := NewStaticSnapshot()
	for _, f := range r.Features {
		for tenant, v := range f.Defaults {
			s.Set(tenant, f.Key, strings.EqualFold(v, "on"))
		}
	}
	return s
}
