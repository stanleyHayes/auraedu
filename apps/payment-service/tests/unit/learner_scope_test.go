package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type invoiceAccessStub struct{ allowed map[string]bool }

func (s invoiceAccessStub) Resolve(_ context.Context, _, _, _ string, requested []string) ([]string, error) {
	var result []string
	for _, id := range requested {
		if s.allowed[id] {
			result = append(result, id)
		}
	}
	return result, nil
}

type invalidCheckoutProvider struct{}

func (invalidCheckoutProvider) Initiate(context.Context, domain.Payment) (string, string, error) {
	return "unsafe-ref", "http://checkout.invalid", nil
}
func (invalidCheckoutProvider) Verify(context.Context, string) (string, error) { return "", nil }

func TestLearnerPaymentAccessFailsClosedWithoutOwnershipResolver(t *testing.T) {
	gate := flags.NewStaticSnapshot()
	gate.Set(unitTenantA, application.FeaturePayments, true)
	svc := application.NewService(newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}, application.WithFeatureGate(gate))
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: unitTenantA})
	for _, role := range []string{"parent", "student"} {
		actor := auth.Actor{UserID: role + "-1", TenantID: unitTenantA, Role: role, Permissions: []string{application.PermRead, application.PermInitiate}}
		if _, _, err := svc.ListPayments(ctx, actor, ports.PaymentFilter{}); !errors.Is(err, domain.ErrUnavailable) {
			t.Fatalf("%s list must fail closed, got %v", role, err)
		}
		if _, err := svc.InitiatePayment(ctx, actor, "payment-1"); !errors.Is(err, domain.ErrUnavailable) {
			t.Fatalf("%s initiation must fail closed, got %v", role, err)
		}
	}
}

func TestInitiationRejectsInsecureCheckoutAndRollsBackPending(t *testing.T) {
	pRepo := newFakePaymentRepo()
	payment, err := domain.NewPayment(unitTenantA, unitInvoice, "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pRepo.Create(context.Background(), unitTenantA, payment); err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(pRepo, &fakeTxRepo{}, &fakeWebhookRepo{},
		application.WithFeatureGate(enabledGates()),
		application.WithPaymentProvider(invalidCheckoutProvider{}),
		application.WithInvoiceAccessResolver(invoiceAccessStub{allowed: map[string]bool{unitInvoice: true}}),
	)
	actor := auth.Actor{UserID: "parent-1", TenantID: unitTenantA, Role: "parent", Permissions: []string{application.PermInitiate}}
	if _, err := svc.InitiatePayment(tenantCtx(unitTenantA), actor, payment.ID); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected invalid checkout rejection, got %v", err)
	}
	stored, err := pRepo.GetByID(context.Background(), unitTenantA, payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(domain.PaymentStatusPending) {
		t.Fatalf("expected rollback to pending, got %q", stored.Status)
	}
}

func TestLearnerPaymentsAreFilteredAndUnauthorizedIDsAreHidden(t *testing.T) {
	pRepo, txRepo, wRepo := newFakePaymentRepo(), &fakeTxRepo{}, &fakeWebhookRepo{}
	own, err := domain.NewPayment(unitTenantA, unitInvoice, "mock", "GHS", 10000, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := domain.NewPayment(unitTenantA, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "mock", "GHS", 5000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pRepo.Create(context.Background(), unitTenantA, own); err != nil {
		t.Fatal(err)
	}
	if err := pRepo.Create(context.Background(), unitTenantA, other); err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithFeatureGate(enabledGates()),
		application.WithPaymentProvider(&stubProvider{ref: "learner-ref"}),
		application.WithInvoiceAccessResolver(invoiceAccessStub{allowed: map[string]bool{unitInvoice: true}}),
	)
	actor := auth.Actor{UserID: "parent-1", TenantID: unitTenantA, Role: "parent", Permissions: []string{application.PermRead, application.PermInitiate}}
	ctx := tenantCtx(unitTenantA)
	records, _, err := svc.ListPayments(ctx, actor, ports.PaymentFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].ID != own.ID {
		t.Fatalf("expected owned payment only, got %+v", records)
	}
	if _, err := svc.GetPayment(ctx, actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected unauthorized payment hidden, got %v", err)
	}
	initiated, err := svc.InitiatePayment(ctx, actor, own.ID)
	if err != nil {
		t.Fatalf("initiate owned payment: %v", err)
	}
	if initiated.ProviderReference == nil || *initiated.ProviderReference != "learner-ref" {
		t.Fatalf("expected provider initiation, got %+v", initiated)
	}
	if initiated.CheckoutURL == nil || *initiated.CheckoutURL == "" {
		t.Fatalf("expected secure checkout URL, got %+v", initiated)
	}
}
