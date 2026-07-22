package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/fees-service/internal/adapters/postgres"
	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"

	studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	studentB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	ay1      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	ay2      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
)

func newRepos(t *testing.T) (ports.FeeStructureRepository, ports.InvoiceRepository) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewFeeStructureRepository(tdb.DB), postgres.NewInvoiceRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateFeeStructure(ctx context.Context, t *testing.T, repo ports.FeeStructureRepository, name, academicYearID string, amountCents int) *domain.FeeStructure {
	t.Helper()
	fs, err := domain.NewFeeStructure(tenantA, name, academicYearID, "GHS", "termly", "all_students", amountCents, nil, nil)
	if err != nil {
		t.Fatalf("new fee structure: %v", err)
	}
	if err := repo.Create(ctx, tenantA, fs); err != nil {
		t.Fatalf("create fee structure: %v", err)
	}
	return fs
}

func mustCreateInvoice(ctx context.Context, t *testing.T, repo ports.InvoiceRepository, studentID, feeStructureID string, amountCents int) *domain.Invoice {
	t.Helper()
	inv, err := domain.NewInvoice(tenantA, studentID, feeStructureID, amountCents, amountCents, domain.Date{}, nil)
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}
	if err := repo.Create(ctx, tenantA, inv); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return inv
}

func TestFeeStructureRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, repo, "Tuition", ay1, 10000)

	got, err := repo.GetByID(ctx, tenantA, fs.ID)
	if err != nil {
		t.Fatalf("get fee structure: %v", err)
	}
	if got.ID != fs.ID || got.Name != "Tuition" {
		t.Fatalf("fee structure mismatch: %+v", got)
	}
}

func TestPaymentReconciliationIsAtomicIdempotentAndCurrencySafe(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	feeRepo, invoicePort := newRepos(t)
	invoiceRepo, ok := invoicePort.(*postgres.InvoiceRepository)
	if !ok {
		t.Fatalf("unexpected invoice repository type %T", invoicePort)
	}
	fee := mustCreateFeeStructure(ctx, t, feeRepo, "Tuition", ay1, 10000)
	invoice := mustCreateInvoice(ctx, t, invoicePort, studentA, fee.ID, 10000)

	initial, err := invoiceRepo.GetStudentBalance(ctx, tenantA, studentA)
	if err != nil || len(initial.Totals) != 1 || initial.Totals[0].OutstandingCents != 10000 {
		t.Fatalf("initial balance: %+v err=%v", initial, err)
	}
	first := ports.PaymentApplication{InvoiceID: invoice.ID, PaymentID: "11111111-aaaa-4aaa-8aaa-111111111111", AmountCents: 4000, ReceivedAt: time.Now()}
	updated, receipt, created, err := invoiceRepo.ApplyPayment(ctx, tenantA, first)
	if err != nil || !created || updated.Status != string(domain.InvoiceStatusPartial) || updated.BalanceCents != 6000 {
		t.Fatalf("partial reconciliation: invoice=%+v receipt=%+v created=%v err=%v", updated, receipt, created, err)
	}
	replayed, sameReceipt, created, err := invoiceRepo.ApplyPayment(ctx, tenantA, first)
	if err != nil || created || replayed.BalanceCents != 6000 || sameReceipt.ID != receipt.ID {
		t.Fatalf("replay: invoice=%+v receipt=%+v created=%v err=%v", replayed, sameReceipt, created, err)
	}
	second := ports.PaymentApplication{InvoiceID: invoice.ID, PaymentID: "22222222-bbbb-4bbb-8bbb-222222222222", AmountCents: 7000, ReceivedAt: time.Now()}
	paid, overpayment, created, err := invoiceRepo.ApplyPayment(ctx, tenantA, second)
	if err != nil || !created || paid.Status != string(domain.InvoiceStatusPaid) || paid.BalanceCents != 0 {
		t.Fatalf("paid reconciliation: invoice=%+v receipt=%+v created=%v err=%v", paid, overpayment, created, err)
	}
	if overpayment.AppliedCents != 6000 || overpayment.OverpaymentCents != 1000 {
		t.Fatalf("overpayment evidence: %+v", overpayment)
	}
	outbox, err := invoiceRepo.ClaimPendingFeeEvents(context.Background(), 10)
	if err != nil || len(outbox) != 3 {
		t.Fatalf("reconciliation outbox=%+v err=%v", outbox, err)
	}
	counts := map[string]int{}
	for _, item := range outbox {
		counts[item.EventType]++
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			t.Fatalf("decode fees outbox: %v", err)
		}
		if payload["invoice_id"] != invoice.ID || payload["student_id"] != studentA {
			t.Fatalf("unexpected fees outbox payload: %+v", payload)
		}
		if err := invoiceRepo.MarkFeeEventPublished(context.Background(), item.ID); err != nil {
			t.Fatalf("mark fees event published: %v", err)
		}
	}
	if counts["invoice.updated.v1"] != 2 || counts["invoice.paid.v1"] != 1 {
		t.Fatalf("unexpected reconciliation events: %+v", counts)
	}
	if pending, err := invoiceRepo.ClaimPendingFeeEvents(context.Background(), 10); err != nil || len(pending) != 0 {
		t.Fatalf("published fees outbox must drain: pending=%+v err=%v", pending, err)
	}
	final, err := invoiceRepo.GetStudentBalance(ctx, tenantA, studentA)
	if err != nil || final.Totals[0].TotalPaidCents != 10000 || final.Totals[0].OutstandingCents != 0 {
		t.Fatalf("final balance: %+v err=%v", final, err)
	}
	if _, err := invoiceRepo.GetReceiptByID(withTenant(context.Background(), tenantB), tenantB, receipt.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant receipt: %v", err)
	}
}

func TestPaymentReconciliationRollsBackWithoutOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	feeRepo := postgres.NewFeeStructureRepository(tdb.DB)
	invoiceRepo := postgres.NewInvoiceRepository(tdb.DB)
	fee := mustCreateFeeStructure(ctx, t, feeRepo, "Atomic tuition", ay1, 10000)
	invoice := mustCreateInvoice(ctx, t, invoiceRepo, studentA, fee.ID, 10000)
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE fees_outbox`); err != nil {
		t.Fatal(err)
	}
	input := ports.PaymentApplication{InvoiceID: invoice.ID, PaymentID: "33333333-cccc-4ccc-8ccc-333333333333", AmountCents: 4000, ReceivedAt: time.Now()}
	if _, _, _, err := invoiceRepo.ApplyPayment(ctx, tenantA, input); err == nil {
		t.Fatal("payment reconciliation must fail when its durable events cannot be written")
	}
	stored, err := invoiceRepo.GetByID(ctx, tenantA, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BalanceCents != 10000 || stored.Status != string(domain.InvoiceStatusPending) {
		t.Fatalf("invoice mutation escaped rollback: %+v", stored)
	}
}

func TestInvoiceLifecycleRollsBackWithoutOutbox(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	tdb := testkit.NewPostgres(context.Background(), t, "../../migrations")
	feeRepo := postgres.NewFeeStructureRepository(tdb.DB)
	invoiceRepo := postgres.NewInvoiceRepository(tdb.DB)
	fee := mustCreateFeeStructure(ctx, t, feeRepo, "Atomic invoice", ay1, 10000)
	invoice, err := domain.NewInvoice(tenantA, studentA, fee.ID, 10000, 10000, domain.Date{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.DB.Pool().Exec(context.Background(), `DROP TABLE fees_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := invoiceRepo.CommitInvoiceLifecycle(ctx, tenantA, invoice, ports.InvoiceMutationCreate, []ports.LifecycleEvent{{
		EventType: "invoice.created.v1",
		Payload:   ports.InvoiceEventData("invoice.created.v1", invoice, nil),
	}}); err == nil {
		t.Fatal("invoice create must fail when its durable event cannot be written")
	}
	if _, err := invoiceRepo.GetByID(ctx, tenantA, invoice.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invoice mutation escaped rollback: %v", err)
	}
}

func TestFeeStructureRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepos(t)

	mustCreateFeeStructure(ctx, t, repo, "Tuition", ay1, 10000)
	fs2 := mustCreateFeeStructure(ctx, t, repo, "PTA", ay1, 5000)

	page, next, err := repo.List(ctx, tenantA, ports.FeeStructureFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, ports.FeeStructureFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != fs2.ID {
		t.Fatalf("expected second fee structure, got %+v", page2)
	}
}

func TestFeeStructureRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepos(t)

	mustCreateFeeStructure(ctx, t, repo, "Tuition", ay1, 10000)
	mustCreateFeeStructure(ctx, t, repo, "PTA", ay2, 5000)
	archived, err := domain.NewFeeStructure(tenantA, "Old", ay1, "GHS", "one_time", "all_students", 1000, nil, nil)
	if err != nil {
		t.Fatalf("new fee structure: %v", err)
	}
	archived.Status = string(domain.StatusArchived)
	if err := repo.Create(ctx, tenantA, archived); err != nil {
		t.Fatalf("create archived fee structure: %v", err)
	}

	cases := []struct {
		name   string
		filter ports.FeeStructureFilter
		want   int
	}{
		{"by academic_year_id", ports.FeeStructureFilter{Limit: 10, AcademicYearID: ay1}, 2},
		{"by status", ports.FeeStructureFilter{Limit: 10, Status: string(domain.StatusArchived)}, 1},
		{"combined", ports.FeeStructureFilter{Limit: 10, AcademicYearID: ay2}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := repo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestFeeStructureRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, repo, "Tuition", ay1, 10000)
	name := "Updated Tuition"
	if _, err := fs.ApplyUpdate(domain.FeeStructurePatch{Name: &name}); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, fs); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, fs.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Name != name {
		t.Fatalf("fee structure not updated: %+v", got)
	}
}

func TestFeeStructureRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _ := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, repo, "Tuition", ay1, 10000)
	if err := repo.Delete(ctx, tenantA, fs.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, fs.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInvoiceRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, fsRepo, "Tuition", ay1, 10000)
	inv := mustCreateInvoice(ctx, t, invRepo, studentA, fs.ID, 10000)

	got, err := invRepo.GetByID(ctx, tenantA, inv.ID)
	if err != nil {
		t.Fatalf("get invoice: %v", err)
	}
	if got.ID != inv.ID || got.BalanceCents != 10000 {
		t.Fatalf("invoice mismatch: %+v", got)
	}
}

func TestInvoiceRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	fs1 := mustCreateFeeStructure(ctx, t, fsRepo, "Tuition", ay1, 10000)
	fs2 := mustCreateFeeStructure(ctx, t, fsRepo, "PTA", ay1, 5000)
	mustCreateInvoice(ctx, t, invRepo, studentA, fs1.ID, 10000)
	mustCreateInvoice(ctx, t, invRepo, studentB, fs2.ID, 5000)

	cases := []struct {
		name   string
		filter ports.InvoiceFilter
		want   int
	}{
		{"by student_id", ports.InvoiceFilter{Limit: 10, StudentID: studentA}, 1},
		{"by learner scope", ports.InvoiceFilter{Limit: 10, StudentIDs: []string{studentA}}, 1},
		{"by invoice IDs", ports.InvoiceFilter{Limit: 10, InvoiceIDs: []string{"00000000-0000-0000-0000-000000000000"}}, 0},
		{"empty learner scope fails closed", ports.InvoiceFilter{Limit: 10, StudentIDs: []string{}}, 0},
		{"by fee_structure_id", ports.InvoiceFilter{Limit: 10, FeeStructureID: fs2.ID}, 1},
		{"by status", ports.InvoiceFilter{Limit: 10, Status: string(domain.InvoiceStatusPending)}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _, err := invRepo.List(ctx, tenantA, tc.filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(page) != tc.want {
				t.Fatalf("expected %d records, got %d", tc.want, len(page))
			}
		})
	}
}

func TestInvoiceRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, fsRepo, "Tuition", ay1, 10000)
	inv := mustCreateInvoice(ctx, t, invRepo, studentA, fs.ID, 10000)
	status := string(domain.InvoiceStatusPaid)
	if _, err := inv.ApplyUpdate(domain.InvoicePatch{Status: &status}); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := invRepo.Update(ctx, tenantA, inv); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := invRepo.GetByID(ctx, tenantA, inv.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Status != status || got.BalanceCents != 0 {
		t.Fatalf("invoice not updated: %+v", got)
	}
}

func TestInvoiceRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	fs := mustCreateFeeStructure(ctx, t, fsRepo, "Tuition", ay1, 10000)
	inv := mustCreateInvoice(ctx, t, invRepo, studentA, fs.ID, 10000)
	if err := invRepo.Delete(ctx, tenantA, inv.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := invRepo.GetByID(ctx, tenantA, inv.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	fsRepo, invRepo := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	fs := mustCreateFeeStructure(aCtx, t, fsRepo, "Tuition", ay1, 10000)
	inv := mustCreateInvoice(aCtx, t, invRepo, studentA, fs.ID, 10000)

	bCtx := withTenant(ctx, tenantB)
	if _, err := fsRepo.GetByID(bCtx, tenantB, fs.ID); err == nil {
		t.Fatal("tenant B should not see tenant A fee structure")
	}
	if _, err := invRepo.GetByID(bCtx, tenantB, inv.ID); err == nil {
		t.Fatal("tenant B should not see tenant A invoice")
	}

	fsList, _, err := fsRepo.List(bCtx, tenantB, ports.FeeStructureFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B fee structures: %v", err)
	}
	if len(fsList) != 0 {
		t.Fatalf("tenant B should see 0 fee structures, got %d", len(fsList))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	fsRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureFees, false)

	svc := application.NewService(fsRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermManage)

	_, err := svc.CreateFeeStructure(ctx, actor, application.CreateFeeStructureRequest{
		Name:           "Tuition",
		AcademicYearID: ay1,
		AmountCents:    10000,
		Currency:       "GHS",
		Recurrence:     "termly",
		Target:         "all_students",
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureFees, true)

	svc := application.NewService(fsRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	fs, err := svc.CreateFeeStructure(ctx, actor, application.CreateFeeStructureRequest{
		Name:           "Tuition",
		AcademicYearID: ay1,
		AmountCents:    10000,
		Currency:       "GHS",
		Recurrence:     "termly",
		Target:         "all_students",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if fs.ID == "" {
		t.Fatal("expected fee structure id")
	}
}

func TestService_CreateInvoiceFromFeeStructure(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	fsRepo, invRepo := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureFees, true)
	svc := application.NewService(fsRepo, invRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	fs, err := svc.CreateFeeStructure(ctx, actor, application.CreateFeeStructureRequest{
		Name:           "Tuition",
		AcademicYearID: ay1,
		AmountCents:    10000,
		Currency:       "GHS",
		Recurrence:     "termly",
		Target:         "all_students",
	})
	if err != nil {
		t.Fatalf("create fee structure: %v", err)
	}

	inv, err := svc.CreateInvoice(ctx, actor, application.CreateInvoiceRequest{
		StudentID:      studentA,
		FeeStructureID: fs.ID,
		DueDate:        "2025-09-01",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	if inv.AmountCents != 10000 || inv.BalanceCents != 10000 {
		t.Fatalf("expected amount/balance from fee structure, got %+v", inv)
	}
	postgresRepo, ok := invRepo.(*postgres.InvoiceRepository)
	if !ok {
		t.Fatalf("unexpected invoice repository type %T", invRepo)
	}
	outbox, err := postgresRepo.ClaimPendingFeeEvents(context.Background(), 10)
	if err != nil || len(outbox) != 2 {
		t.Fatalf("invoice creation outbox=%+v err=%v", outbox, err)
	}
	counts := map[string]int{}
	for _, event := range outbox {
		counts[event.EventType]++
	}
	if counts["fee.assigned.v1"] != 1 || counts["invoice.created.v1"] != 1 {
		t.Fatalf("invoice creation events=%+v", counts)
	}
}
