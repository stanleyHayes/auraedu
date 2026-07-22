package servercmd

import (
	"strings"
	"testing"

	provideradapter "github.com/auraedu/payment-service/internal/adapters/provider"
)

func liveTestKey() string {
	// Assemble the shape without committing a credential-looking literal.
	return strings.Join([]string{"sk", "live", "test-only-not-a-secret"}, "_")
}

func TestPaymentProviderAllowsMockOnlyOutsideProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("PAYMENTS_PROVIDER", "mock")

	provider, err := paymentProvider()
	if err != nil {
		t.Fatalf("development mock: %v", err)
	}
	if _, ok := provider.(*provideradapter.MockProvider); !ok {
		t.Fatalf("expected mock provider, got %T", provider)
	}

	t.Setenv("ENVIRONMENT", "production")
	if _, err := paymentProvider(); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("production mock must fail closed, got %v", err)
	}
}

func TestPaymentProviderRejectsNonLiveProductionKey(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PAYMENTS_PROVIDER", "paystack")
	t.Setenv("PAYSTACK_SECRET_KEY", "test-key")
	t.Setenv("PAYSTACK_BASE_URL", provideradapter.DefaultPaystackBaseURL)

	if _, err := paymentProvider(); err == nil || !strings.Contains(err.Error(), "live secret key") {
		t.Fatalf("production test key must fail closed, got %v", err)
	}
}

func TestPaymentProviderRejectsProductionOriginOverride(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PAYMENTS_PROVIDER", "paystack")
	t.Setenv("PAYSTACK_SECRET_KEY", liveTestKey())
	t.Setenv("PAYSTACK_BASE_URL", "https://payments.example.test")

	if _, err := paymentProvider(); err == nil || !strings.Contains(err.Error(), "API origin") {
		t.Fatalf("production origin override must fail closed, got %v", err)
	}
}

func TestPaymentProviderAcceptsCanonicalProductionPaystack(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PAYMENTS_PROVIDER", "paystack")
	t.Setenv("PAYSTACK_SECRET_KEY", liveTestKey())
	t.Setenv("PAYSTACK_BASE_URL", provideradapter.DefaultPaystackBaseURL+"/")

	provider, err := paymentProvider()
	if err != nil {
		t.Fatalf("canonical production Paystack: %v", err)
	}
	if _, ok := provider.(*provideradapter.PaystackProvider); !ok {
		t.Fatalf("expected Paystack provider, got %T", provider)
	}
}
