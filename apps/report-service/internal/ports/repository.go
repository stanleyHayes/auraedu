// Package ports defines the report service repository boundary.
package ports

import (
	"context"

	"github.com/auraedu/report-service/internal/domain"
)

// Repository persists ReportTemplate and ReportCard aggregates. Implementations
// MUST scope every query by tenantID (defense-in-depth with Postgres RLS).
type Repository interface {
	// Report templates.
	CreateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error
	GetReportTemplateByID(ctx context.Context, tenantID, id string) (*domain.ReportTemplate, error)
	ListReportTemplates(ctx context.Context, tenantID string, filter ReportTemplateListFilter) ([]*domain.ReportTemplate, string, error)
	UpdateReportTemplate(ctx context.Context, tenantID string, t *domain.ReportTemplate) error
	DeleteReportTemplate(ctx context.Context, tenantID, id string) error

	// Report cards.
	CreateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error
	GetReportCardByID(ctx context.Context, tenantID, id string) (*domain.ReportCard, error)
	ListReportCards(ctx context.Context, tenantID string, filter ReportCardListFilter) ([]*domain.ReportCard, string, error)
	UpdateReportCard(ctx context.Context, tenantID string, c *domain.ReportCard) error
	DeleteReportCard(ctx context.Context, tenantID, id string) error
}

// ReportTemplateListFilter carries cursor pagination and optional equality filters.
type ReportTemplateListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	Status         string
}

// ReportCardListFilter carries cursor pagination and optional equality filters.
type ReportCardListFilter struct {
	Limit          int
	Cursor         string
	AcademicYearID string
	Status         string
	StudentID      string
	TemplateID     string
}
