package integration

import (
	"context"
	"testing"

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
}
