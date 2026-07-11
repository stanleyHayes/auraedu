package integration

import (
	"context"
	"testing"

	"github.com/auraedu/attendance-service/internal/adapters/postgres"
	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const tenantB = "22222222-2222-2222-2222-222222222222"

const studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
const studentB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
const ay1 = "cccccccc-cccc-cccc-cccc-cccccccccccc"
const ay2 = "dddddddd-dddd-dddd-dddd-dddddddddddd"
const staff1 = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

func newRepo(t *testing.T) ports.Repository {
	t.Helper()
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "../../migrations")
	return postgres.NewRepository(tdb.DB)
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func actorWithPerms(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func mustCreateRecord(ctx context.Context, t *testing.T, repo ports.Repository, studentID, academicYearID, date, status string) *domain.AttendanceRecord {
	t.Helper()
	rec, err := domain.NewAttendanceRecord(tenantA, studentID, academicYearID, date, status, staff1, nil)
	if err != nil {
		t.Fatalf("new attendance record: %v", err)
	}
	if err := repo.Create(ctx, tenantA, rec); err != nil {
		t.Fatalf("create attendance record: %v", err)
	}
	return rec
}

func TestRepository_CreateAndGet(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	rec := mustCreateRecord(ctx, t, repo, studentA, ay1, "2025-09-01", "present")

	got, err := repo.GetByID(ctx, tenantA, rec.ID)
	if err != nil {
		t.Fatalf("get attendance record: %v", err)
	}
	if got.ID != rec.ID || got.Status != "present" {
		t.Fatalf("attendance record mismatch: %+v", got)
	}
}

func TestRepository_ListPagination(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	mustCreateRecord(ctx, t, repo, studentA, ay1, "2025-09-01", "present")
	rec2 := mustCreateRecord(ctx, t, repo, studentB, ay1, "2025-09-02", "absent")

	page, next, err := repo.List(ctx, tenantA, ports.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page))
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	page2, _, err := repo.List(ctx, tenantA, ports.ListFilter{Limit: 1, Cursor: next})
	if err != nil {
		t.Fatalf("list cursor: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != rec2.ID {
		t.Fatalf("expected second attendance record, got %+v", page2)
	}
}

func TestRepository_ListFilters(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	mustCreateRecord(ctx, t, repo, studentA, ay1, "2025-09-01", "present")
	mustCreateRecord(ctx, t, repo, studentB, ay1, "2025-09-01", "absent")
	mustCreateRecord(ctx, t, repo, studentA, ay2, "2025-09-02", "late")

	cases := []struct {
		name   string
		filter ports.ListFilter
		want   int
	}{
		{"by student_id", ports.ListFilter{Limit: 10, StudentID: studentA}, 2},
		{"by academic_year_id", ports.ListFilter{Limit: 10, AcademicYearID: ay1}, 2},
		{"by date", ports.ListFilter{Limit: 10, Date: "2025-09-01"}, 2},
		{"by status", ports.ListFilter{Limit: 10, Status: "late"}, 1},
		{"combined", ports.ListFilter{Limit: 10, StudentID: studentA, AcademicYearID: ay1}, 1},
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

func TestRepository_Update(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	rec := mustCreateRecord(ctx, t, repo, studentA, ay1, "2025-09-01", "absent")
	status := "excused"
	reason := "medical"
	if _, err := rec.ApplyUpdate(&status, &reason, nil); err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if err := repo.Update(ctx, tenantA, rec); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, rec.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Status != status || got.Reason == nil || *got.Reason != reason {
		t.Fatalf("record not updated: %+v", got)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	rec := mustCreateRecord(ctx, t, repo, studentA, ay1, "2025-09-01", "present")
	if err := repo.Delete(ctx, tenantA, rec.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenantA, rec.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	rec := mustCreateRecord(aCtx, t, repo, studentA, ay1, "2025-09-01", "present")

	bCtx := withTenant(ctx, tenantB)
	if _, err := repo.GetByID(bCtx, tenantB, rec.ID); err == nil {
		t.Fatal("tenant B should not see tenant A attendance record")
	}

	list, _, err := repo.List(bCtx, tenantB, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("tenant B should see 0 attendance records, got %d", len(list))
	}
}

func TestService_FeatureFlagGatesAccess(t *testing.T) {
	ctx := withTenant(context.Background(), tenantB)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantB, application.FeatureAttendance, false)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantB, application.PermMark)

	_, err := svc.Create(ctx, actor, application.CreateAttendanceRequest{
		StudentID:      studentA,
		AcademicYearID: ay1,
		Date:           "2025-09-01",
		Status:         "present",
		MarkedBy:       staff1,
	})
	if err == nil {
		t.Fatal("expected feature-disabled error")
	}
}

func TestService_FeatureFlagAllowsAccessWhenEnabled(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAttendance, true)

	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := actorWithPerms(tenantA, application.PermMark)

	rec, err := svc.Create(ctx, actor, application.CreateAttendanceRequest{
		StudentID:      studentA,
		AcademicYearID: ay1,
		Date:           "2025-09-01",
		Status:         "present",
		MarkedBy:       staff1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected record id")
	}
}
