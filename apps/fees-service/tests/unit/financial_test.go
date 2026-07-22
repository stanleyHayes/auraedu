package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

type financialStub struct {
	balance *domain.Balance
	receipt *domain.Receipt
	invoice *domain.Invoice
	created bool
}

func (s *financialStub) GetStudentBalance(context.Context, string, string) (*domain.Balance, error) {
	return s.balance, nil
}
func (s *financialStub) GetReceiptByID(_ context.Context, tenantID, id string) (*domain.Receipt, error) {
	if s.receipt == nil || s.receipt.TenantID != tenantID || s.receipt.ID != id {
		return nil, domain.ErrNotFound
	}
	return s.receipt, nil
}
func (s *financialStub) ApplyPayment(context.Context, string, ports.PaymentApplication) (*domain.Invoice, *domain.Receipt, bool, error) {
	return s.invoice, s.receipt, s.created, nil
}

func TestFinancialViewsEnforceLearnerOwnership(t *testing.T) {
	invoice := newInvoice(t, "student-own")
	receipt, err := domain.NewReceipt(feesTenant, invoice.ID, invoice.StudentID, "payment-1", "GHS", 1000, 1000, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	financial := &financialStub{
		balance: &domain.Balance{StudentID: invoice.StudentID, Totals: []domain.CurrencyBalance{{Currency: "GHS", OutstandingCents: 24000}}},
		receipt: receipt,
		invoice: invoice,
	}
	svc := application.NewService(fakeFeeStructureRepo{}, &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{invoice.ID: invoice}},
		application.WithFeatureGate(feesGate()),
		application.WithLearnerScopeResolver(learnerResolver{ids: []string{invoice.StudentID}}),
		application.WithFinancialRepositories(financial, financial, financial),
	)
	parent := auth.Actor{UserID: "parent-1", TenantID: feesTenant, Role: "parent", Permissions: []string{application.PermRead}}
	if _, err := svc.GetStudentBalance(feesContext(), parent, invoice.StudentID); err != nil {
		t.Fatalf("owned balance: %v", err)
	}
	if _, err := svc.GetStudentBalance(feesContext(), parent, "student-other"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other balance: %v", err)
	}
	if _, err := svc.GetReceipt(feesContext(), parent, receipt.ID); err != nil {
		t.Fatalf("owned receipt: %v", err)
	}
}

func TestPaymentReconciliationRequiresMatchingTenantContext(t *testing.T) {
	invoice := newInvoice(t, "student-own")
	receipt, err := domain.NewReceipt(feesTenant, invoice.ID, invoice.StudentID, "payment-1", "GHS", 1000, 1000, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	financial := &financialStub{invoice: invoice, receipt: receipt, created: true}
	svc := application.NewService(fakeFeeStructureRepo{}, &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{}},
		application.WithFeatureGate(feesGate()),
		application.WithFinancialRepositories(financial, financial, financial),
	)
	input := application.PaymentReceivedInput{TenantID: feesTenant, InvoiceID: invoice.ID, PaymentID: "payment-1", AmountCents: 1000}
	if _, _, _, err := svc.ApplyPaymentReceived(feesContext(), input); err != nil {
		t.Fatalf("apply payment: %v", err)
	}
	input.TenantID = "tenant-other"
	if _, _, _, err := svc.ApplyPaymentReceived(feesContext(), input); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("tenant mismatch: %v", err)
	}
}
