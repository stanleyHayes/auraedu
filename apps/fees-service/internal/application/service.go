// Package application implements the fees-service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
	fsRepo             ports.FeeStructureRepository
	invRepo            ports.InvoiceRepository
	balanceRepo        ports.BalanceRepository
	receiptRepo        ports.ReceiptRepository
	reconciliationRepo ports.PaymentReconciliationRepository
	pub                ports.EventPublisher
	gates              flags.Gate
	scope              ports.LearnerScopeResolver
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithLearnerScopeResolver configures identity-to-student ownership resolution.
func WithLearnerScopeResolver(r ports.LearnerScopeResolver) Option {
	return func(s *Service) { s.scope = r }
}

// WithFinancialRepositories configures balance projections, receipt reads and
// event-driven provider reconciliation.
func WithFinancialRepositories(balance ports.BalanceRepository, receipts ports.ReceiptRepository, reconciliation ports.PaymentReconciliationRepository) Option {
	return func(s *Service) {
		s.balanceRepo = balance
		s.receiptRepo = receipts
		s.reconciliationRepo = reconciliation
	}
}

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
	assignmentMeta := map[string]any{
		"invoice_id":   inv.ID,
		"student_id":   inv.StudentID,
		"amount_cents": inv.AmountCents,
	}
	if durable, ok := s.invRepo.(ports.InvoiceLifecycleRepository); ok {
		events := []ports.LifecycleEvent{
			{EventType: "fee.assigned.v1", Payload: ports.FeeStructureEventData(fs, assignmentMeta)},
			{EventType: "invoice.created.v1", Payload: ports.InvoiceEventData("invoice.created.v1", inv, nil)},
		}
		if err := durable.CommitInvoiceLifecycle(ctx, tenantID, inv, ports.InvoiceMutationCreate, events); err != nil {
			return nil, err
		}
		return inv, nil
	}
	if err := s.invRepo.Create(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	if err := s.pub.PublishFeeStructure(ctx, "fee.assigned.v1", fs, assignmentMeta); err != nil {
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
	filter, err = s.applyInvoiceScope(ctx, actor, filter)
	if err != nil {
		return nil, "", err
	}
	return s.invRepo.List(ctx, tenantID, filter)
}

// GetInvoice returns a single invoice.
func (s *Service) GetInvoice(ctx context.Context, actor auth.Actor, id string) (*domain.Invoice, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	inv, err := s.invRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if scopedLearnerRole(actor.Role) {
		filter, scopeErr := s.applyInvoiceScope(ctx, actor, ports.InvoiceFilter{StudentID: inv.StudentID})
		if scopeErr != nil {
			return nil, scopeErr
		}
		if len(filter.StudentIDs) == 0 {
			return nil, domain.ErrNotFound
		}
	}
	return inv, nil
}

// GetStudentBalance returns currency-separated invoice ledger totals.
func (s *Service) GetStudentBalance(ctx context.Context, actor auth.Actor, studentID string) (*domain.Balance, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.balanceRepo == nil {
		return nil, domain.ErrUnavailable
	}
	if err := s.authorizeStudentID(ctx, actor, studentID); err != nil {
		return nil, err
	}
	return s.balanceRepo.GetStudentBalance(ctx, tenantID, studentID)
}

// GetReceipt returns immutable payment evidence after enforcing learner ownership.
func (s *Service) GetReceipt(ctx context.Context, actor auth.Actor, id string) (*domain.Receipt, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.receiptRepo == nil {
		return nil, domain.ErrUnavailable
	}
	receipt, err := s.receiptRepo.GetReceiptByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeStudentID(ctx, actor, receipt.StudentID); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (s *Service) authorizeStudentID(ctx context.Context, actor auth.Actor, studentID string) error {
	if !scopedLearnerRole(actor.Role) {
		return nil
	}
	if s.scope == nil {
		return domain.ErrUnavailable
	}
	ids, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == studentID {
			return nil
		}
	}
	return domain.ErrNotFound
}

// PaymentReceivedInput is the trusted payment.received.v1 payload.
type PaymentReceivedInput struct {
	TenantID          string
	InvoiceID         string
	PaymentID         string
	AmountCents       int
	ProviderReference *string
	ReceivedAt        time.Time
}

// ApplyPaymentReceived idempotently reconciles a provider payment into the
// invoice ledger. Tenant context remains mandatory for database RLS.
func (s *Service) ApplyPaymentReceived(ctx context.Context, input PaymentReceivedInput) (*domain.Invoice, *domain.Receipt, bool, error) {
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" || input.TenantID == "" {
		return nil, nil, false, domain.ErrMissingTenant
	}
	if tenantID != input.TenantID {
		return nil, nil, false, domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureFees) {
		return nil, nil, false, fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureFees)
	}
	if s.reconciliationRepo == nil {
		return nil, nil, false, domain.ErrUnavailable
	}
	invoice, receipt, created, err := s.reconciliationRepo.ApplyPayment(ctx, tenantID, ports.PaymentApplication{
		InvoiceID: input.InvoiceID, PaymentID: input.PaymentID, AmountCents: input.AmountCents,
		ProviderReference: input.ProviderReference, ReceivedAt: input.ReceivedAt,
	})
	if err != nil || !created {
		return invoice, receipt, created, err
	}
	if durable, ok := s.reconciliationRepo.(ports.DurablePaymentReconciliation); ok && durable.PaymentReconciliationEventsDurable() {
		return invoice, receipt, true, nil
	}
	meta := map[string]any{"payment_id": input.PaymentID, "receipt_id": receipt.ID, "applied_cents": receipt.AppliedCents}
	if err := s.pub.PublishInvoice(ctx, "invoice.updated.v1", invoice, meta); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish reconciled invoice update", "err", err)
	}
	if invoice.Status == string(domain.InvoiceStatusPaid) {
		if err := s.pub.PublishInvoice(ctx, "invoice.paid.v1", invoice, meta); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish reconciled invoice paid event", "err", err)
		}
	}
	return invoice, receipt, true, nil
}

func scopedLearnerRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "parent" || role == "student"
}

func (s *Service) applyInvoiceScope(ctx context.Context, actor auth.Actor, filter ports.InvoiceFilter) (ports.InvoiceFilter, error) {
	if !scopedLearnerRole(actor.Role) {
		return filter, nil
	}
	if s.scope == nil {
		return filter, domain.ErrUnavailable
	}
	ids, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
	if err != nil {
		return filter, err
	}
	if filter.StudentID != "" {
		for _, id := range ids {
			if id == filter.StudentID {
				filter.StudentIDs = []string{id}
				filter.StudentID = ""
				return filter, nil
			}
		}
		return filter, domain.ErrNotFound
	}
	filter.StudentIDs = ids
	return filter, nil
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
	nowPaid := inv.Status == string(domain.InvoiceStatusPaid)
	meta := map[string]any{"changed_fields": changed}
	events := []ports.LifecycleEvent{{EventType: "invoice.updated.v1", Payload: ports.InvoiceEventData("invoice.updated.v1", inv, meta)}}
	if !wasPaid && nowPaid {
		events = append(events, ports.LifecycleEvent{
			EventType: "invoice.paid.v1",
			Payload:   ports.InvoiceEventData("invoice.paid.v1", inv, meta),
		})
	}
	if durable, ok := s.invRepo.(ports.InvoiceLifecycleRepository); ok {
		if err := durable.CommitInvoiceLifecycle(ctx, tenantID, inv, ports.InvoiceMutationUpdate, events); err != nil {
			return nil, err
		}
		return inv, nil
	}
	if err := s.invRepo.Update(ctx, tenantID, inv); err != nil {
		return nil, err
	}
	if !wasPaid && nowPaid {
		if err := s.pub.PublishInvoice(ctx, "invoice.paid.v1", inv, meta); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.paid.v1", "err", err)
		}
	}
	if err := s.pub.PublishInvoice(ctx, "invoice.updated.v1", inv, meta); err != nil {
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
	if durable, ok := s.invRepo.(ports.InvoiceLifecycleRepository); ok {
		events := []ports.LifecycleEvent{{
			EventType: "invoice.deleted.v1",
			Payload:   ports.InvoiceEventData("invoice.deleted.v1", inv, nil),
		}}
		return durable.CommitInvoiceLifecycle(ctx, tenantID, inv, ports.InvoiceMutationDelete, events)
	}
	if err := s.invRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.PublishInvoice(ctx, "invoice.deleted.v1", inv, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish invoice event", "event_type", "invoice.deleted.v1", "err", err)
	}
	return nil
}

// ResolveInvoiceAccess returns only requested invoices owned by the learner identity.
// It is used by trusted services and deliberately bypasses end-user RBAC while retaining
// tenant and learner ownership checks.
func (s *Service) ResolveInvoiceAccess(ctx context.Context, tenantID, userID, role string, invoiceIDs []string) ([]string, error) {
	tenantID, userID, role = strings.TrimSpace(tenantID), strings.TrimSpace(userID), strings.ToLower(strings.TrimSpace(role))
	if tenantID == "" {
		return nil, domain.ErrMissingTenant
	}
	if userID == "" || (role != "parent" && role != "student") || len(invoiceIDs) > 100 {
		return nil, domain.ErrValidation
	}
	if len(invoiceIDs) == 0 {
		return []string{}, nil
	}
	if s.scope == nil {
		return nil, domain.ErrUnavailable
	}
	studentIDs, err := s.scope.Resolve(ctx, tenantID, userID, role)
	if err != nil {
		return nil, err
	}
	records, _, err := s.invRepo.List(ctx, tenantID, ports.InvoiceFilter{Limit: len(invoiceIDs), StudentIDs: studentIDs, InvoiceIDs: invoiceIDs})
	if err != nil {
		return nil, err
	}
	allowed := make([]string, 0, len(records))
	for _, invoice := range records {
		allowed = append(allowed, invoice.ID)
	}
	return allowed, nil
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
