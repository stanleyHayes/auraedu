package ports

import (
	"context"

	"github.com/auraedu/payment-service/internal/domain"
)

// PaymentProvider abstracts a payment-gateway integration.
type PaymentProvider interface {
	// Initiate starts a payment at the provider and returns the provider reference and a checkout URL.
	Initiate(ctx context.Context, p domain.Payment) (providerReference, checkoutURL string, err error)
	// Verify checks the final status of a payment using its provider reference.
	Verify(ctx context.Context, reference string) (status string, err error)
}
