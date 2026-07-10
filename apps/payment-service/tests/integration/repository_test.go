package integration

import (
	"context"
	"testing"

	"github.com/auraedu/payment-service/internal/adapters/postgres"
	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	tenantA = "11111111-1111-1111-1111-111111111111"
	tenantB = "22222222-2222-2222-2222-222222222222"

	invoiceA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	invoiceB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func newRepos(t *testing.T) (ports.PaymentRepository, ports.TransactionRepository, ports.WebhookEventRepository, *testkit.PostgresTestDB) {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewPaymentRepository(tdb.DB), postgres.NewTransactionRepository(tdb.DB), postgres.NewWebhookEventRepository(tdb.DB), tdb
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreatePayment(t *testing.T, ctx context.Context, repo ports.PaymentRepository, tenantID, invoiceID, provider string, amountCents int) *domain.Payment {
	t.Helper()
	p, err := domain.NewPayment(tenantID, invoiceID, provider, "GHS", amountCents, nil)
	if err != nil {
		t.Fatalf("new payment: %v", err)
	}
	if err := repo.Create(ctx, tenantID, p); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return p
}

func mustCreateTransaction(t *testing.T, ctx context.Context, repo ports.TransactionRepository, tenantID, paymentID, reference string, amountCents int) *domain.Transaction {
	t.Helper()
	tx, err := domain.NewTransaction(tenantID, paymentID, "credit", "success", reference, amountCents)
	if err != nil {
		t.Fatalf("new transaction: %v", err)
	}
	if err := repo.Create(ctx, tenantID, tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	return tx
}

func TestPaymentRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, repo, tenantA, invoiceA, "mock", 10000)

	got, err := repo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if got.ID != p.ID || got.InvoiceID != invoiceA {
		t.Fatalf("payment mismatch: %+v", got)
	}
}

func TestPaymentRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	mustCreatePayment(t, ctx, repo, tenantA, invoiceA, "mock", 10000)
	p2 := mustCreatePayment(t, ctx, repo, tenantA, invoiceB, "mock", 5000)

	page, next, err := repo.List(ctx, tenantA, ports.PaymentFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, ports.PaymentFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != p2.ID {
		t.Fatalf("expected second payment, got %+v", page2)
	}
}

func TestPaymentRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	mustCreatePayment(t, ctx, repo, tenantA, invoiceA, "mock", 10000)
	mustCreatePayment(t, ctx, repo, tenantA, invoiceB, "paystack", 5000)

	cases := []struct {
		name   string
		filter ports.PaymentFilter
		want   int
	}{
		{"by invoice_id", ports.PaymentFilter{Limit: 10, InvoiceID: invoiceA}, 1},
		{"by provider", ports.PaymentFilter{Limit: 10, Provider: "paystack"}, 1},
		{"by status", ports.PaymentFilter{Limit: 10, Status: string(domain.PaymentStatusPending)}, 2},
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

func TestPaymentRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, repo, tenantA, invoiceA, "mock", 10000)
	status := string(domain.PaymentStatusProcessing)
	ref := "mock_ref_123"
	if _, err := p.ApplyUpdate(domain.PaymentPatch{Status: &status, ProviderReference: &ref}); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Status != status || got.ProviderReference == nil || *got.ProviderReference != ref {
		t.Fatalf("payment not updated: %+v", got)
	}
}

func TestPaymentRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo, _, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, repo, tenantA, invoiceA, "mock", 10000)
	if err := repo.Delete(ctx, tenantA, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, p.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTransactionRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, pRepo, tenantA, invoiceA, "mock", 10000)
	tx := mustCreateTransaction(t, ctx, txRepo, tenantA, p.ID, "ref-1", 10000)

	got, err := txRepo.GetByID(ctx, tenantA, tx.ID)
	if err != nil {
		t.Fatalf("get transaction: %v", err)
	}
	if got.ID != tx.ID || got.PaymentID != p.ID {
		t.Fatalf("transaction mismatch: %+v", got)
	}
}

func TestTransactionRepository_ListByPayment(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, pRepo, tenantA, invoiceA, "mock", 10000)
	mustCreateTransaction(t, ctx, txRepo, tenantA, p.ID, "ref-1", 10000)
	mustCreateTransaction(t, ctx, txRepo, tenantA, p.ID, "ref-2", 5000)

	page, _, err := txRepo.ListByPayment(ctx, tenantA, p.ID, ports.TransactionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(page))
	}
}

func TestTransactionRepository_FKEnforcesSameTenant(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, _, _ := newRepos(t)

	p := mustCreatePayment(t, ctx, pRepo, tenantA, invoiceA, "mock", 10000)

	bCtx := withTenant(context.Background(), tenantB)
	tx, _ := domain.NewTransaction(tenantB, p.ID, "credit", "success", "ref-1", 10000)
	if err := txRepo.Create(bCtx, tenantB, tx); err == nil {
		t.Fatal("expected FK violation when payment belongs to another tenant")
	}
}

func TestWebhookEventRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	_, _, wRepo, _ := newRepos(t)

	w, err := domain.NewWebhookEvent("mock", "charge.success", []byte(`{"reference":"ref-1"}`), nil)
	if err != nil {
		t.Fatalf("new webhook event: %v", err)
	}
	w.SetTenant(tenantA)
	if err := wRepo.Create(ctx, tenantA, w); err != nil {
		t.Fatalf("create webhook event: %v", err)
	}

	got, err := wRepo.GetByID(ctx, tenantA, w.ID)
	if err != nil {
		t.Fatalf("get webhook event: %v", err)
	}
	if got.ID != w.ID {
		t.Fatalf("webhook event mismatch: %+v", got)
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	pRepo, txRepo, wRepo, _ := newRepos(t)

	aCtx := withTenant(ctx, tenantA)
	p := mustCreatePayment(t, aCtx, pRepo, tenantA, invoiceA, "mock", 10000)
	tx := mustCreateTransaction(t, aCtx, txRepo, tenantA, p.ID, "ref-1", 10000)
	w, _ := domain.NewWebhookEvent("mock", "charge.success", []byte(`{"reference":"ref-1"}`), nil)
	w.SetTenant(tenantA)
	if err := wRepo.Create(aCtx, tenantA, w); err != nil {
		t.Fatalf("create webhook event: %v", err)
	}

	bCtx := withTenant(ctx, tenantB)
	if _, err := pRepo.GetByID(bCtx, tenantB, p.ID); err == nil {
		t.Fatal("tenant B should not see tenant A payment")
	}
	if _, err := txRepo.GetByID(bCtx, tenantB, tx.ID); err == nil {
		t.Fatal("tenant B should not see tenant A transaction")
	}
	if _, err := wRepo.GetByID(bCtx, tenantB, w.ID); err == nil {
		t.Fatal("tenant B should not see tenant A webhook event")
	}

	pList, _, err := pRepo.List(bCtx, tenantB, ports.PaymentFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B payments: %v", err)
	}
	if len(pList) != 0 {
		t.Fatalf("tenant B should see 0 payments, got %d", len(pList))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	pRepo, txRepo, wRepo, _ := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeaturePayments, false)

	svc := application.NewService(pRepo, txRepo, wRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermManage)

	_, err := svc.CreatePayment(ctx, actor, application.CreatePaymentRequest{
		InvoiceID:   invoiceA,
		AmountCents: 10000,
		Currency:    "GHS",
		Provider:    "mock",
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, wRepo, _ := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeaturePayments, true)

	svc := application.NewService(pRepo, txRepo, wRepo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermManage)

	p, err := svc.CreatePayment(ctx, actor, application.CreatePaymentRequest{
		InvoiceID:   invoiceA,
		AmountCents: 10000,
		Currency:    "GHS",
		Provider:    "mock",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected payment id")
	}
}

func TestService_InitiatePaymentFlow(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, wRepo, _ := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeaturePayments, true)
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithFeatureGate(gates),
		application.WithPaymentProvider(&mockProvider{}),
	)

	actor := actorWithPerms(tenantA, application.PermInitiate)
	p, err := svc.CreatePayment(ctx, actorWithPerms(tenantA, application.PermManage), application.CreatePaymentRequest{
		InvoiceID:   invoiceA,
		AmountCents: 10000,
		Currency:    "GHS",
		Provider:    "mock",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	initiated, err := svc.InitiatePayment(ctx, actor, p.ID)
	if err != nil {
		t.Fatalf("initiate payment: %v", err)
	}
	if initiated.Status != string(domain.PaymentStatusProcessing) {
		t.Fatalf("expected processing status, got %q", initiated.Status)
	}
	if initiated.ProviderReference == nil || *initiated.ProviderReference == "" {
		t.Fatal("expected provider reference set")
	}
}

func TestService_ProcessWebhookIdempotency(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	pRepo, txRepo, wRepo, _ := newRepos(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeaturePayments, true)
	svc := application.NewService(pRepo, txRepo, wRepo,
		application.WithFeatureGate(gates),
		application.WithPaymentProvider(&mockProvider{}),
	)

	p, err := svc.CreatePayment(ctx, actorWithPerms(tenantA, application.PermManage), application.CreatePaymentRequest{
		InvoiceID:   invoiceA,
		AmountCents: 10000,
		Currency:    "GHS",
		Provider:    "mock",
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	_, err = svc.InitiatePayment(ctx, actorWithPerms(tenantA, application.PermInitiate), p.ID)
	if err != nil {
		t.Fatalf("initiate payment: %v", err)
	}

	// Refresh payment to get provider reference.
	p, err = pRepo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}

	payload := []byte(`{"reference":"success","tenant_id":"` + tenantA + `"}`)
	_, err = svc.ProcessWebhook(ctx, application.ProcessWebhookRequest{
		Provider:  "mock",
		Payload:   payload,
		Signature: "",
	})
	if err != nil {
		t.Fatalf("process webhook: %v", err)
	}

	p, err = pRepo.GetByID(ctx, tenantA, p.ID)
	if err != nil {
		t.Fatalf("get payment after webhook: %v", err)
	}
	if p.Status != string(domain.PaymentStatusSuccess) {
		t.Fatalf("expected success status, got %q", p.Status)
	}

	// Second identical webhook should be idempotent: no error, payment stays success.
	_, err = svc.ProcessWebhook(ctx, application.ProcessWebhookRequest{
		Provider:  "mock",
		Payload:   payload,
		Signature: "",
	})
	if err != nil {
		t.Fatalf("process webhook second time: %v", err)
	}
}

type mockProvider struct{}

func (m *mockProvider) Initiate(ctx context.Context, p domain.Payment) (string, string, error) {
	return "success", "https://mock.auraedu.test/checkout/" + p.ID, nil
}

func (m *mockProvider) Verify(ctx context.Context, reference string) (string, error) {
	if reference == "success" {
		return string(domain.PaymentStatusSuccess), nil
	}
	return string(domain.PaymentStatusFailed), nil
}
