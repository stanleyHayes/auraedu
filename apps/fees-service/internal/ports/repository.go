package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
)

// FeeStructureRepository persists FeeStructure aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type FeeStructureRepository interface {
	Create(ctx context.Context, tenantID string, f *domain.FeeStructure) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.FeeStructure, error)
	List(ctx context.Context, tenantID string, filter FeeStructureFilter) ([]*domain.FeeStructure, string, error)
	Update(ctx context.Context, tenantID string, f *domain.FeeStructure) error
	Delete(ctx context.Context, tenantID, id string) error
}

// InvoiceRepository persists Invoice aggregates. Implementations MUST scope every
// query by tenantID (defense-in-depth with Postgres RLS).
type InvoiceRepository interface {
	Create(ctx context.Context, tenantID string, i *domain.Invoice) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Invoice, error)
	List(ctx context.Context, tenantID string, filter InvoiceFilter) ([]*domain.Invoice, string, error)
	Update(ctx context.Context, tenantID string, i *domain.Invoice) error
	Delete(ctx context.Context, tenantID, id string) error
}

// BalanceRepository produces currency-safe ledger projections from invoices.
type BalanceRepository interface {
	GetStudentBalance(context.Context, string, string) (*domain.Balance, error)
}

// ReceiptRepository reads immutable provider reconciliation evidence.
type ReceiptRepository interface {
	GetReceiptByID(context.Context, string, string) (*domain.Receipt, error)
}

// PaymentApplication carries a confirmed provider payment into the fees ledger.
type PaymentApplication struct {
	InvoiceID         string
	PaymentID         string
	AmountCents       int
	ProviderReference *string
	ReceivedAt        time.Time
}

// PaymentReconciliationRepository applies a payment and creates its receipt in
// one transaction. created=false means the payment was already reconciled.
type PaymentReconciliationRepository interface {
	ApplyPayment(context.Context, string, PaymentApplication) (*domain.Invoice, *domain.Receipt, bool, error)
}

// DurablePaymentReconciliation identifies repositories that write invoice
// lifecycle events in the same transaction as the receipt and balance change.
type DurablePaymentReconciliation interface {
	PaymentReconciliationEventsDurable() bool
}

const (
	InvoiceMutationCreate = "create"
	InvoiceMutationUpdate = "update"
	InvoiceMutationDelete = "delete"
)

type LifecycleEvent struct {
	EventType string
	Payload   map[string]any
}

type InvoiceLifecycleRepository interface {
	CommitInvoiceLifecycle(context.Context, string, *domain.Invoice, string, []LifecycleEvent) error
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

type OutboxRepository interface {
	ClaimPendingFeeEvents(context.Context, int) ([]OutboxEvent, error)
	MarkFeeEventPublished(context.Context, string) error
	MarkFeeEventFailed(context.Context, string, string) error
}

type LearnerScopeResolver interface {
	Resolve(context.Context, string, string, string) ([]string, error)
}

// FeeStructureFilter carries cursor pagination and optional equality filters.
type FeeStructureFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	Status         string
}

// InvoiceFilter carries cursor pagination and optional equality filters.
type InvoiceFilter struct {
	Limit          int
	Cursor         string
	StudentID      string
	StudentIDs     []string
	InvoiceIDs     []string
	FeeStructureID string
	Status         string
}
