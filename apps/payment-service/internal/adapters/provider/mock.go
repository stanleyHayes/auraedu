// Package provider implements deterministic payment-provider adapters for tests and local development.
package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/google/uuid"
)

// MockProvider is a deterministic payment provider for tests and local development.
// It returns success when the reference contains "success".
type MockProvider struct{}

// NewMockProvider creates a new mock provider adapter.
func NewMockProvider() *MockProvider { return &MockProvider{} }

// Initiate returns a synthetic provider reference and checkout URL.
func (m *MockProvider) Initiate(_ context.Context, p domain.Payment) (string, string, error) {
	ref := fmt.Sprintf("mock_%s_%s", p.ID, uuid.Must(uuid.NewV7()).String())
	url := fmt.Sprintf("https://mock.auraedu.test/checkout/%s", p.ID)
	return ref, url, nil
}

// Verify returns "success" when the reference contains "success", otherwise "failed".
func (m *MockProvider) Verify(_ context.Context, reference string) (string, error) {
	if strings.Contains(reference, "success") {
		return string(domain.PaymentStatusSuccess), nil
	}
	return string(domain.PaymentStatusFailed), nil
}
