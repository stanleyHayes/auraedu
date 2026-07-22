package servercmd

import (
	"strings"
	"testing"
)

func TestValidateProductionRuntimeFailsClosed(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "NATS_URL"} {
		t.Setenv(key, "")
	}
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("missing production database must fail first, got %v", err)
	}

	t.Setenv("DATABASE_URL", "postgres://configured")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("missing production redis must fail, got %v", err)
	}

	t.Setenv("REDIS_URL", "redis://configured")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("missing production nats must fail, got %v", err)
	}

	t.Setenv("NATS_URL", "nats://configured")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "INTERNAL_SERVICE_TOKEN") {
		t.Fatalf("missing internal service credential must fail, got %v", err)
	}
	t.Setenv("INTERNAL_SERVICE_TOKEN", "configured-secret")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "MFA_ENCRYPTION_KEY") {
		t.Fatalf("missing MFA encryption key must fail, got %v", err)
	}
	t.Setenv("MFA_ENCRYPTION_KEY", "too-short")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("short MFA encryption key must fail, got %v", err)
	}
	t.Setenv("MFA_ENCRYPTION_KEY", "production-mfa-key-with-32-characters")
	if err := validateProductionRuntime(); err != nil {
		t.Fatalf("complete production runtime rejected: %v", err)
	}
}

func TestValidateProductionRuntimeAllowsDevelopmentFallbacks(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("NATS_URL", "")
	if err := validateProductionRuntime(); err != nil {
		t.Fatalf("development fallback rejected: %v", err)
	}
}
