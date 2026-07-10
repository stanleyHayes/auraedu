package ports

import (
	"context"

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
	FeeStructureID string
	Status         string
}
