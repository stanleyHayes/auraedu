package servercmd

import (
	"strings"
	"testing"
)

func TestValidateProductionRuntimeFailsClosed(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("missing production database must fail first, got %v", err)
	}

	t.Setenv("DATABASE_URL", "postgres://configured")
	if err := validateProductionRuntime(); err == nil || !strings.Contains(err.Error(), "INTERNAL_SERVICE_TOKEN") {
		t.Fatalf("missing internal service credential must fail, got %v", err)
	}

	t.Setenv("INTERNAL_SERVICE_TOKEN", "configured-secret")
	if err := validateProductionRuntime(); err != nil {
		t.Fatalf("complete production runtime rejected: %v", err)
	}
}

func TestValidateProductionRuntimeAllowsDevelopmentRepository(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")
	if err := validateProductionRuntime(); err != nil {
		t.Fatalf("development fallback rejected: %v", err)
	}
}
