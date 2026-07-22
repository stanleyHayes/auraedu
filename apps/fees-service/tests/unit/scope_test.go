package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const feesTenant = "11111111-1111-1111-1111-111111111111"

type scopedInvoiceRepo struct{ invoices map[string]*domain.Invoice }

func (r *scopedInvoiceRepo) Create(_ context.Context, _ string, invoice *domain.Invoice) error {
	r.invoices[invoice.ID] = invoice
	return nil
}
func (r *scopedInvoiceRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Invoice, error) {
	invoice, ok := r.invoices[id]
	if !ok || invoice.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	return invoice, nil
}
func (r *scopedInvoiceRepo) List(_ context.Context, tenantID string, filter ports.InvoiceFilter) ([]*domain.Invoice, string, error) {
	var result []*domain.Invoice
	for _, invoice := range r.invoices {
		if invoice.TenantID != tenantID || !containsID(filter.StudentIDs, invoice.StudentID) || (filter.InvoiceIDs != nil && !containsID(filter.InvoiceIDs, invoice.ID)) {
			continue
		}
		result = append(result, invoice)
	}
	return result, "", nil
}

func TestResolveInvoiceAccessReturnsOwnedRequestedSubset(t *testing.T) {
	own := newInvoice(t, "student-own")
	other := newInvoice(t, "student-other")
	repo := &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{own.ID: own, other.ID: other}}
	svc := application.NewService(fakeFeeStructureRepo{}, repo, application.WithLearnerScopeResolver(learnerResolver{ids: []string{own.StudentID}}))
	ids, err := svc.ResolveInvoiceAccess(feesContext(), feesTenant, "parent-1", "parent", []string{own.ID, other.ID})
	if err != nil {
		t.Fatalf("resolve invoice access: %v", err)
	}
	if len(ids) != 1 || ids[0] != own.ID {
		t.Fatalf("expected only owned requested invoice, got %v", ids)
	}
}
func (r *scopedInvoiceRepo) Update(context.Context, string, *domain.Invoice) error { return nil }
func (r *scopedInvoiceRepo) Delete(context.Context, string, string) error          { return nil }

type learnerResolver struct {
	ids []string
	err error
}

func (r learnerResolver) Resolve(context.Context, string, string, string) ([]string, error) {
	return r.ids, r.err
}

func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func feesContext() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: feesTenant})
}

func feesGate() *flags.StaticSnapshot {
	gate := flags.NewStaticSnapshot()
	gate.Set(feesTenant, application.FeatureFees, true)
	return gate
}

func newInvoice(t *testing.T, studentID string) *domain.Invoice {
	t.Helper()
	invoice, err := domain.NewInvoice(feesTenant, studentID, "fee-1", 25000, 25000, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}
	return invoice
}

func TestParentInvoicesAreLearnerScoped(t *testing.T) {
	own := newInvoice(t, "student-own")
	other := newInvoice(t, "student-other")
	repo := &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{own.ID: own, other.ID: other}}
	svc := application.NewService(fakeFeeStructureRepo{}, repo,
		application.WithFeatureGate(feesGate()),
		application.WithLearnerScopeResolver(learnerResolver{ids: []string{own.StudentID}}),
	)
	actor := auth.Actor{UserID: "parent-1", TenantID: feesTenant, Role: "parent", Permissions: []string{application.PermRead}}

	invoices, _, err := svc.ListInvoices(feesContext(), actor, ports.InvoiceFilter{})
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}
	if len(invoices) != 1 || invoices[0].ID != own.ID {
		t.Fatalf("expected only owned invoice, got %+v", invoices)
	}
	if _, err := svc.GetInvoice(feesContext(), actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected other learner invoice to be hidden, got %v", err)
	}
}

func TestParentInvoiceScopeFailsClosedWithoutResolver(t *testing.T) {
	svc := application.NewService(fakeFeeStructureRepo{}, &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{}}, application.WithFeatureGate(feesGate()))
	actor := auth.Actor{UserID: "parent-1", TenantID: feesTenant, Role: "parent", Permissions: []string{application.PermRead}}
	if _, _, err := svc.ListInvoices(feesContext(), actor, ports.InvoiceFilter{}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected unavailable without learner resolver, got %v", err)
	}
}
