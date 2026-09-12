// Package mongo persists fee structures, invoices and receipts in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Receipts are stored inside the invoice they pay rather than in their own
// collection. PostgreSQL applies a payment by updating the invoice balance,
// inserting the receipt and queueing the events in one transaction; without one,
// those would be separate writes and a crash between them would lose money or
// invent it. As part of the invoice, the whole reconciliation is a single
// atomic document write.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	StructureCollection = "fee_structures"
	InvoiceCollection   = "invoices"
	outboxLease         = 5 * time.Minute
	eventSource         = "fees-service"
)

func live(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	merged["deleted_at"] = bson.M{"$exists": false}
	return merged
}

func notFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	return err
}

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("fees: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// ---- fee structures ------------------------------------------------------

type StructureRepository struct{ structures *pmongo.Collection }

var _ ports.FeeStructureRepository = (*StructureRepository)(nil)

func NewStructureRepository(store *pmongo.Store) *StructureRepository {
	return &StructureRepository{structures: store.Collection(StructureCollection)}
}

type structureDoc struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	Name           string    `bson:"name"`
	AcademicYearID string    `bson:"academic_year_id"`
	AmountCents    int       `bson:"amount_cents"`
	Currency       string    `bson:"currency"`
	Recurrence     string    `bson:"recurrence"`
	Target         string    `bson:"target"`
	DueDay         *int      `bson:"due_day,omitempty"`
	Description    *string   `bson:"description,omitempty"`
	Status         string    `bson:"status"`
	CreatedAt      time.Time `bson:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"`
}

func (d structureDoc) toDomain() *domain.FeeStructure {
	return &domain.FeeStructure{
		ID: d.ID, TenantID: d.TenantID, Name: d.Name, AcademicYearID: d.AcademicYearID,
		AmountCents: d.AmountCents, Currency: d.Currency, Recurrence: d.Recurrence,
		Target: d.Target, DueDay: d.DueDay, Description: d.Description, Status: d.Status,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func structureFields(f *domain.FeeStructure) bson.M {
	return bson.M{
		"name": f.Name, "academic_year_id": f.AcademicYearID, "amount_cents": f.AmountCents,
		"currency": f.Currency, "recurrence": f.Recurrence, "target": f.Target,
		"due_day": f.DueDay, "description": f.Description, "status": f.Status,
		"created_at": f.CreatedAt, "updated_at": f.UpdatedAt,
	}
}

func (r *StructureRepository) Create(ctx context.Context, tenantID string, f *domain.FeeStructure) error {
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := structureFields(f)
	doc["_id"] = f.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("fees: create structure: %w", err)
	}
	return nil
}

func (r *StructureRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.FeeStructure, error) {
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	var doc structureDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		return nil, notFound(err)
	}
	return doc.toDomain(), nil
}

func (r *StructureRepository) List(ctx context.Context, tenantID string, filter ports.FeeStructureFilter) ([]*domain.FeeStructure, string, error) {
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := bson.M{}
	if filter.AcademicYearID != "" {
		query["academic_year_id"] = filter.AcademicYearID
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Cursor != "" {
		query["_id"] = bson.M{"$gt": filter.Cursor}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("fees: list structures: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.FeeStructure
	for cur.Next(ctx) {
		var doc structureDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("fees: decode structure: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *StructureRepository) Update(ctx context.Context, tenantID string, f *domain.FeeStructure) error {
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": f.ID}), bson.M{"$set": structureFields(f)})
	if err != nil {
		return fmt.Errorf("fees: update structure: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *StructureRepository) Delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("fees: delete structure: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// ---- invoices and receipts -----------------------------------------------

type InvoiceRepository struct {
	invoices   *pmongo.Collection
	structures *pmongo.Collection
	outbox     *pmongo.ClaimableOutbox
}

var (
	_ ports.InvoiceRepository               = (*InvoiceRepository)(nil)
	_ ports.BalanceRepository               = (*InvoiceRepository)(nil)
	_ ports.ReceiptRepository               = (*InvoiceRepository)(nil)
	_ ports.PaymentReconciliationRepository = (*InvoiceRepository)(nil)
	_ ports.DurablePaymentReconciliation    = (*InvoiceRepository)(nil)
	_ ports.InvoiceLifecycleRepository      = (*InvoiceRepository)(nil)
	_ ports.OutboxRepository                = (*InvoiceRepository)(nil)
)

func NewInvoiceRepository(store *pmongo.Store) *InvoiceRepository {
	invoices := store.Collection(InvoiceCollection)
	return &InvoiceRepository{
		invoices: invoices, structures: store.Collection(StructureCollection),
		outbox: pmongo.NewClaimableOutbox(invoices, outboxLease),
	}
}

func (*InvoiceRepository) PaymentReconciliationEventsDurable() bool { return true }

type receiptDoc struct {
	ID                string    `bson:"id"`
	InvoiceID         string    `bson:"invoice_id"`
	StudentID         string    `bson:"student_id"`
	PaymentID         string    `bson:"payment_id"`
	AmountCents       int       `bson:"amount_cents"`
	AppliedCents      int       `bson:"applied_cents"`
	OverpaymentCents  int       `bson:"overpayment_cents"`
	Currency          string    `bson:"currency"`
	ProviderReference *string   `bson:"provider_reference,omitempty"`
	IssuedAt          time.Time `bson:"issued_at"`
}

func (d receiptDoc) toDomain(tenantID string) *domain.Receipt {
	return &domain.Receipt{
		ID: d.ID, TenantID: tenantID, InvoiceID: d.InvoiceID, StudentID: d.StudentID,
		PaymentID: d.PaymentID, AmountCents: d.AmountCents, AppliedCents: d.AppliedCents,
		OverpaymentCents: d.OverpaymentCents, Currency: d.Currency,
		ProviderReference: d.ProviderReference, IssuedAt: d.IssuedAt,
	}
}

type invoiceDoc struct {
	ID             string       `bson:"_id"`
	TenantID       string       `bson:"tenant_id"`
	StudentID      string       `bson:"student_id"`
	FeeStructureID string       `bson:"fee_structure_id"`
	AmountCents    int          `bson:"amount_cents"`
	BalanceCents   int          `bson:"balance_cents"`
	Status         string       `bson:"status"`
	DueDate        string       `bson:"due_date,omitempty"`
	IssuedAt       time.Time    `bson:"issued_at"`
	Notes          *string      `bson:"notes,omitempty"`
	Receipts       []receiptDoc `bson:"receipts,omitempty"`
	CreatedAt      time.Time    `bson:"created_at"`
	UpdatedAt      time.Time    `bson:"updated_at"`
}

func (d invoiceDoc) toDomain() *domain.Invoice {
	inv := &domain.Invoice{
		ID: d.ID, TenantID: d.TenantID, StudentID: d.StudentID,
		FeeStructureID: d.FeeStructureID, AmountCents: d.AmountCents,
		BalanceCents: d.BalanceCents, Status: d.Status, IssuedAt: d.IssuedAt,
		Notes: d.Notes, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.DueDate != "" {
		if parsed, err := domain.NewDate(d.DueDate); err == nil {
			inv.DueDate = parsed
		}
	}
	return inv
}

func invoiceFields(i *domain.Invoice) bson.M {
	return bson.M{
		"student_id": i.StudentID, "fee_structure_id": i.FeeStructureID,
		"amount_cents": i.AmountCents, "balance_cents": i.BalanceCents,
		"status": i.Status, "due_date": i.DueDate.String(), "issued_at": i.IssuedAt,
		"notes": i.Notes, "created_at": i.CreatedAt, "updated_at": i.UpdatedAt,
	}
}

func (r *InvoiceRepository) Create(ctx context.Context, tenantID string, i *domain.Invoice) error {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	doc := invoiceFields(i)
	doc["_id"] = i.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("fees: create invoice: %w", err)
	}
	return nil
}

func (r *InvoiceRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Invoice, error) {
	doc, err := r.invoiceDoc(ctx, tenantID, bson.M{"_id": id})
	if err != nil {
		return nil, err
	}
	return doc.toDomain(), nil
}

func (r *InvoiceRepository) invoiceDoc(ctx context.Context, tenantID string, query bson.M) (invoiceDoc, error) {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return invoiceDoc{}, err
	}
	var doc invoiceDoc
	if err := scope.FindOne(ctx, live(query)).Decode(&doc); err != nil {
		return invoiceDoc{}, notFound(err)
	}
	return doc, nil
}

func (r *InvoiceRepository) List(ctx context.Context, tenantID string, filter ports.InvoiceFilter) ([]*domain.Invoice, string, error) {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return nil, "", err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	query := bson.M{}
	if filter.StudentID != "" {
		query["student_id"] = filter.StudentID
	}
	if len(filter.StudentIDs) > 0 {
		query["student_id"] = bson.M{"$in": filter.StudentIDs}
	}
	if len(filter.InvoiceIDs) > 0 {
		query["_id"] = bson.M{"$in": filter.InvoiceIDs}
	}
	if filter.FeeStructureID != "" {
		query["fee_structure_id"] = filter.FeeStructureID
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Cursor != "" {
		if existing, ok := query["_id"].(bson.M); ok {
			existing["$gt"] = filter.Cursor
		} else {
			query["_id"] = bson.M{"$gt": filter.Cursor}
		}
	}
	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("fees: list invoices: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*domain.Invoice
	for cur.Next(ctx) {
		var doc invoiceDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("fees: decode invoice: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *InvoiceRepository) Update(ctx context.Context, tenantID string, i *domain.Invoice) error {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": i.ID}), bson.M{"$set": invoiceFields(i)})
	if err != nil {
		return fmt.Errorf("fees: update invoice: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *InvoiceRepository) Delete(ctx context.Context, tenantID, id string) error {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	res, err := scope.DeleteOne(ctx, live(bson.M{"_id": id}))
	if err != nil {
		return fmt.Errorf("fees: delete invoice: %w", err)
	}
	if res.DeletedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// GetStudentBalance totals a learner's invoices per currency, which it reads
// from the fee structure each invoice was issued against. Unlike monies are
// never summed together.
func (r *InvoiceRepository) GetStudentBalance(ctx context.Context, tenantID, studentID string) (*domain.Balance, error) {
	invoices, _, err := r.List(ctx, tenantID, ports.InvoiceFilter{StudentID: studentID, Limit: 100})
	if err != nil {
		return nil, err
	}
	scope, err := r.structures.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}

	currencies := map[string]*domain.CurrencyBalance{}
	order := []string{}
	for _, inv := range invoices {
		var structure structureDoc
		if err := scope.FindOne(ctx, bson.M{"_id": inv.FeeStructureID}).Decode(&structure); err != nil {
			return nil, fmt.Errorf("fees: read structure currency: %w", notFound(err))
		}
		entry, ok := currencies[structure.Currency]
		if !ok {
			entry = &domain.CurrencyBalance{Currency: structure.Currency}
			currencies[structure.Currency] = entry
			order = append(order, structure.Currency)
		}
		entry.TotalInvoicedCents += inv.AmountCents
		entry.TotalPaidCents += inv.AmountCents - inv.BalanceCents
		entry.OutstandingCents += inv.BalanceCents
	}

	balance := &domain.Balance{StudentID: studentID}
	for _, currency := range order {
		balance.Totals = append(balance.Totals, *currencies[currency])
	}
	return balance, nil
}

func (r *InvoiceRepository) GetReceiptByID(ctx context.Context, tenantID, id string) (*domain.Receipt, error) {
	doc, err := r.invoiceDoc(ctx, tenantID, bson.M{"receipts.id": id})
	if err != nil {
		return nil, err
	}
	for _, receipt := range doc.Receipts {
		if receipt.ID == id {
			return receipt.toDomain(tenantID), nil
		}
	}
	return nil, domain.ErrNotFound
}

// ApplyPayment reconciles a provider payment against an invoice.
//
// The balance change, the receipt and both events are one atomic write on the
// invoice, which is why receipts live inside it. The filter refuses an already
// reconciled payment, so a redelivery returns the stored result rather than
// applying the money twice, and it also pins the balance that was read so a
// concurrent payment cannot be lost — a changed balance simply retries.
func (r *InvoiceRepository) ApplyPayment(ctx context.Context, tenantID string, input ports.PaymentApplication) (*domain.Invoice, *domain.Receipt, bool, error) {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return nil, nil, false, err
	}

	for range 5 {
		doc, err := r.invoiceDoc(ctx, tenantID, bson.M{"_id": input.InvoiceID})
		if err != nil {
			return nil, nil, false, err
		}
		for _, existing := range doc.Receipts {
			if existing.PaymentID == input.PaymentID {
				return doc.toDomain(), existing.toDomain(tenantID), false, nil
			}
		}

		var structure structureDoc
		structScope, err := r.structures.ScopeTo(tenantID)
		if err != nil {
			return nil, nil, false, err
		}
		if err := structScope.FindOne(ctx, bson.M{"_id": doc.FeeStructureID}).Decode(&structure); err != nil {
			return nil, nil, false, fmt.Errorf("fees: read structure currency: %w", notFound(err))
		}

		applied := input.AmountCents
		if applied > doc.BalanceCents {
			applied = doc.BalanceCents
		}
		receipt, err := domain.NewReceipt(
			tenantID, doc.ID, doc.StudentID, input.PaymentID, structure.Currency,
			input.AmountCents, applied, input.ProviderReference, input.ReceivedAt,
		)
		if err != nil {
			return nil, nil, false, err
		}

		balance := doc.BalanceCents - applied
		status := string(domain.InvoiceStatusPartial)
		if balance == 0 {
			status = string(domain.InvoiceStatusPaid)
		}
		updatedAt := time.Now().UTC()

		next := doc.toDomain()
		next.BalanceCents = balance
		next.Status = status
		next.UpdatedAt = updatedAt

		meta := map[string]any{
			"payment_id": input.PaymentID, "receipt_id": receipt.ID,
			"applied_cents": receipt.AppliedCents,
		}
		events := []tenancy.CloudEvent{}
		updated, err := lifecycleEvent(tenantID, "invoice.updated.v1", invoiceEventData(next, meta))
		if err != nil {
			return nil, nil, false, err
		}
		events = append(events, updated)
		if status == string(domain.InvoiceStatusPaid) {
			paid, err := lifecycleEvent(tenantID, "invoice.paid.v1", invoiceEventData(next, meta))
			if err != nil {
				return nil, nil, false, err
			}
			events = append(events, paid)
		}

		res, err := scope.UpdateWithEvents(ctx, live(bson.M{
			"_id":                 input.InvoiceID,
			"balance_cents":       doc.BalanceCents,
			"receipts.payment_id": bson.M{"$ne": input.PaymentID},
		}), bson.M{
			"$set": bson.M{"balance_cents": balance, "status": status, "updated_at": updatedAt},
			"$push": bson.M{"receipts": bson.M{
				"id": receipt.ID, "invoice_id": receipt.InvoiceID, "student_id": receipt.StudentID,
				"payment_id": receipt.PaymentID, "amount_cents": receipt.AmountCents,
				"applied_cents": receipt.AppliedCents, "overpayment_cents": receipt.OverpaymentCents,
				"currency": receipt.Currency, "provider_reference": receipt.ProviderReference,
				"issued_at": receipt.IssuedAt,
			}},
		}, events...)
		if err != nil {
			return nil, nil, false, fmt.Errorf("fees: apply payment: %w", err)
		}
		if res.MatchedCount == 1 {
			return next, receipt, true, nil
		}
		// The invoice changed underneath: read it again and decide afresh.
	}
	return nil, nil, false, domain.ErrConflict
}

func invoiceEventData(i *domain.Invoice, meta map[string]any) map[string]any {
	data := map[string]any{
		"invoice_id": i.ID, "student_id": i.StudentID, "status": i.Status,
		"amount_cents": i.AmountCents, "balance_cents": i.BalanceCents,
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}

// CommitInvoiceLifecycle applies the mutation and queues its events in one
// atomic write. A delete tombstones the invoice so the event survives.
func (r *InvoiceRepository) CommitInvoiceLifecycle(ctx context.Context, tenantID string, invoice *domain.Invoice, mutation string, lifecycle []ports.LifecycleEvent) error {
	scope, err := r.invoices.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	events := make([]tenancy.CloudEvent, 0, len(lifecycle))
	for _, e := range lifecycle {
		event, err := lifecycleEvent(tenantID, e.EventType, e.Payload)
		if err != nil {
			return err
		}
		events = append(events, event)
	}

	switch mutation {
	case ports.InvoiceMutationCreate:
		doc := invoiceFields(invoice)
		doc["_id"] = invoice.ID
		if _, err := scope.InsertWithEvents(ctx, doc, events...); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return domain.ErrConflict
			}
			return fmt.Errorf("fees: create invoice: %w", err)
		}
		return nil
	case ports.InvoiceMutationUpdate:
		res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": invoice.ID}),
			bson.M{"$set": invoiceFields(invoice)}, events...)
		if err != nil {
			return fmt.Errorf("fees: update invoice: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		return nil
	case ports.InvoiceMutationDelete:
		res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": invoice.ID}),
			bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, events...)
		if err != nil {
			return fmt.Errorf("fees: delete invoice: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		return nil
	default:
		return fmt.Errorf("fees: unsupported lifecycle mutation %q", mutation)
	}
}

func (r *InvoiceRepository) ClaimPendingFeeEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	claimed, err := r.outbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("fees: claim outbox: %w", err)
	}
	out := make([]ports.OutboxEvent, 0, len(claimed))
	for _, c := range claimed {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID, EventType: c.Event.Type,
			Payload: json.RawMessage(c.Event.Data), CreatedAt: time.Now().UTC(),
		})
	}
	return out, nil
}

func (r *InvoiceRepository) MarkFeeEventPublished(ctx context.Context, id string) error {
	if err := r.outbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("fees: mark published: %w", err)
	}
	if _, err := r.outbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}}); err != nil {
		return err
	}
	return nil
}

func (r *InvoiceRepository) MarkFeeEventFailed(ctx context.Context, id, reason string) error {
	if err := r.outbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("fees: mark failed: %w", err)
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX statements in the PostgreSQL
// migrations. Receipts are indexed inside their invoice.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]mongo.IndexModel{
		StructureCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "academic_year_id", Value: 1}}},
		},
		InvoiceCollection: {
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "receipts.id", Value: 1}}},
			{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "receipts.payment_id", Value: 1}}},
		},
	}
	for name, models := range specs {
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("fees: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}
