// Package application implements the fees-service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead   = "fees.read"
	PermManage = "fees.manage"
)

// FeatureFees is the feature flag key for fees management.
const FeatureFees = "fees"

// Service holds the fees use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	fsRepo  ports.FeeStructureRepository
	invRepo ports.InvoiceRepository
	pub     ports.EventPublisher
	gates   flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

type noopPublisher struct{}

func (noopPublisher) PublishFeeStructure(context.Context, string, *domain.FeeStructure, map[string]any) error {
	return nil
}

func (noopPublisher) PublishInvoice(context.Context, string, *domain.Invoice, map[string]any) error {
	return nil
}

// NewService constructs the application service.
func NewService(fsRepo ports.FeeStructureRepository, invRepo ports.InvoiceRepository, opts ...Option) *Service {
	s := &Service{
		fsRepo:  fsRepo,
		invRepo: invRepo,
		pub:     noopPublisher{},
		gates:   flags.NewStaticSnapshot(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- FeeStructure use cases ---

// CreateFeeStructureRequest is the input for creating a fee structure.
type CreateFeeStructureRequest struct {
	Name           string
	AcademicYearID string
	AmountCents    int
	Currency       string
	Recurrence     string
	Target         string
	DueDay         *int
	Description    *string
}

// UpdateFeeStructureRequest is the input for patching a fee structure.
type UpdateFeeStructureRequest struct {
	Name           *string
	AcademicYearID *string
	AmountCents    *int
	Currency       *string
	Recurrence     *string
	Target         *string
	DueDay         *int
	Description    *string
	Status         *string
}

// CreateFeeStructure validates and persists a new FeeStructure.
func (s *Service) CreateFeeStructure(ctx context.Context, actor auth.Actor, req CreateFeeStructureRequest) (*domain.FeeStructure, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	fs, err := domain.NewFeeStructure(
		tenantID, req.Name, req.AcademicYearID, req.Currency, req.Recurrence,
		req.Target, req.AmountCents, req.DueDay, req.Description,
	)
	if err != nil {
		return nil, err
	}
	if err := s.fsRepo.Create(ctx, tenantID, fs); err != nil {
		return nil, err
	}
	return fs, nil
}

// ListFeeStructures returns a tenant-scoped page of fee structures.
func (s *Service) ListFeeStructures(ctx context.Context, actor auth.Actor, filter ports.FeeStructureFilter) ([]*domain.FeeStructure, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.fsRepo.List(ctx, tenantID, filter)
}

// GetFeeStructure returns a single fee structure.
func (s *Service) GetFeeStructure(ctx context.Context, actor auth.Actor, id string) (*domain.FeeStructure, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.fsRepo.GetByID(ctx, tenantID, id)
}

// UpdateFeeStructure patches a fee structure.
func (s *Service) UpdateFeeStructure(ctx context.Context, actor auth.Actor, id string, req UpdateFeeStructureRequest) (*domain.FeeStructure, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	fs, err := s.fsRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := fs.ApplyUpdate(domain.FeeStructurePatch{
		Name:           req.Name,
		AcademicYearID: req.AcademicYearID,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		Recurrence:     req.Recurrence,
		Target:         req.Target,
		DueDay:         req.DueDay,
		Description:    req.Description,
		Status:         req.Status,
	})
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return fs, nil
	}
	if err := fs.Validate(); err != nil {
		return nil, err
	}
	if err := s.fsRepo.Update(ctx, tenantID, fs); err != nil {
		return nil, err
	}
	return fs, nil
}

// DeleteFeeStructure removes a fee structure.
func (s *Service) DeleteFeeStructure(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.fsRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.fsRepo.Delete(ctx, tenantID, id)
}

// --- Invoice use cases ---

// CreateInvoiceRequest is the input for creating an invoice.
type CreateInvoiceRequest struct {
	StudentID      string
	FeeStructureID string
	AmountCents    int
	BalanceCents   *int
	DueDate        string
	Notes          *string
}

// UpdateInvoiceRequest is the input for patching an invoice.
type UpdateInvoiceRequest struct {
	AmountCents  *int
	BalanceCents *int
	Status       *string
	DueDate      *string
	Notes        *string
}

// CreateInvoice validates and persists a new Invoice, optionally deriving the
// amount from a fee structure. Emits fee.assigned.v1 and invoice.created.v1.
func (s *Service) CreateInvoice(ctx context.Context, actor auth.Actor, req CreateInvoiceRequest) (*domain.Invoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}

	fs, err := s.fsRepo.GetByID(ctx, tenantID, req.FeeStructureID)
	if err != nil {
		return nil, fmt.Errorf("%w: fee structure not found", domain.ErrValidation)
	}

	amountCents := req.AmountCents
	if amountCents == 0 && fs != nil {
		amountCents = fs.AmountCents
	}

	balanceCents := amountCents
	if req.BalanceCents != nil {
		balanceCents = *req.BalanceCents
	}

	dueDate, err := domain.NewDate(req.DueDate)
	if err != nil {
		return nil, fmt.Errorf("%w: due_date must be YYYY-MM-DD", domain.ErrValidation)
	}

	inv, err := domain.NewInvoice(tenantID, req.StudentID, req.FeeStructureID, amountCents, balanceCents, dueDate, req.Notes)
	if err != nil {
		return nil, err
	}
	if err := s.invRepo.Create(ctx, tenantID, inv); err != nil {
		return nil, err
	}

	if err := s.pub.PublishFeeStructure(ctx, "fee.assigned.v1", fs, map[string]any{
		"invoice_id":   inv.ID,
		"student_id":   inv.StudentID,
		"amount_cents": inv.AmountCents,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish fee structure event", "event_type", "fee.assigned.v1", "err", err)
	}
	if err := s.pub.PublishInvoice(ctx, "invoice.created.v1", inv, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.created.v1", "err", err)
	}
	return inv, nil
}

// ListInvoices returns a tenant-scoped page of invoices.
func (s *Service) ListInvoices(ctx context.Context, actor auth.Actor, filter ports.InvoiceFilter) ([]*domain.Invoice, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.invRepo.List(ctx, tenantID, filter)
}

// GetInvoice returns a single invoice.
func (s *Service) GetInvoice(ctx context.Context, actor auth.Actor, id string) (*domain.Invoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.invRepo.GetByID(ctx, tenantID, id)
}

// UpdateInvoice patches an invoice.
func (s *Service) UpdateInvoice(ctx context.Context, actor auth.Actor, id string, req UpdateInvoiceRequest) (*domain.Invoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	var dueDate *domain.Date
	if req.DueDate != nil {
		d, err := domain.NewDate(*req.DueDate)
		if err != nil {
			return nil, fmt.Errorf("%w: due_date must be YYYY-MM-DD", domain.ErrValidation)
		}
		dueDate = &d
	}

	wasPaid := inv.Status == string(domain.InvoiceStatusPaid)
	changed, err := inv.ApplyUpdate(domain.InvoicePatch{
		AmountCents:  req.AmountCents,
		BalanceCents: req.BalanceCents,
		Status:       req.Status,
		DueDate:      dueDate,
		Notes:        req.Notes,
	})
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return inv, nil
	}
	if err := inv.Validate(); err != nil {
		return nil, err
	}
	if err := s.invRepo.Update(ctx, tenantID, inv); err != nil {
		return nil, err
	}

	nowPaid := inv.Status == string(domain.InvoiceStatusPaid)
	if !wasPaid && nowPaid {
		if err := s.pub.PublishInvoice(ctx, "invoice.paid.v1", inv, map[string]any{"changed_fields": changed}); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.paid.v1", "err", err)
		}
	}
	if err := s.pub.PublishInvoice(ctx, "invoice.updated.v1", inv, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.updated.v1", "err", err)
	}
	return inv, nil
}

// DeleteInvoice removes an invoice.
func (s *Service) DeleteInvoice(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.invRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.PublishInvoice(ctx, "invoice.deleted.v1", inv, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.deleted.v1", "err", err)
	}
	return nil
}

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) {
		return "", domain.ErrForbidden
	}
	if !actor.Has(perm) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureFees) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureFees)
	}
	return tenantID, nil
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
