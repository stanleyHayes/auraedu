package flags

import (
	"log/slog"
	"strings"

	"github.com/auraedu/platform/config"
)

// NewRuntimeGate selects the safe feature source for a service process.
//
// Development may use the checked-in registry when Tenant Service is absent or
// temporarily unavailable so the local stack remains usable. Production never
// trusts those bootstrap defaults: a missing, failed or malformed Tenant
// Service lookup disables the feature until the live entitlement can be read.
func NewRuntimeGate(baseURL string, registryFallback Gate, log *slog.Logger) Gate {
	if registryFallback == nil {
		registryFallback = NewStaticSnapshot()
	}
	if log == nil {
		log = slog.Default()
	}

	if strings.EqualFold(strings.TrimSpace(config.Getenv("ENVIRONMENT", "development")), "production") {
		return NewTenantServiceClient(baseURL, WarnOnceFailClosed(log))
	}

	if strings.TrimSpace(baseURL) == "" {
		return registryFallback
	}
	return NewTenantServiceClient(baseURL, WarnOnceFallback(registryFallback, log))
}
