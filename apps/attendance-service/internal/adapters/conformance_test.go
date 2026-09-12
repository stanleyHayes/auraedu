// Package adapters_test runs one contract against every attendance repository driver.
package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mongoadapter "github.com/auraedu/attendance-service/internal/adapters/mongo"
	pgadapter "github.com/auraedu/attendance-service/internal/adapters/postgres"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

type attendanceRepo interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
}

func tenantCtx(tenantID string) context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantID})
}

func newRecord(t *testing.T, tenantID, studentID, yearID, day string) *domain.AttendanceRecord {
	t.Helper()
	date, err := domain.NewDate(day)
	if err != nil {
		t.Fatalf("new date: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.AttendanceRecord{
		ID: uuid.NewString(), TenantID: tenantID, StudentID: studentID,
		AcademicYearID: yearID, Date: date, Status: "present",
		MarkedBy: uuid.NewString(), CreatedAt: now, UpdatedAt: now,
	}
}

func runAttendanceContract(t *testing.T, repo attendanceRepo) {
	const tenant = "upshs"
	const other = "aboom"
	ctx := tenantCtx(tenant)

	t.Run("a record round trips and stays inside its tenant", func(t *testing.T) {
		rec := newRecord(t, tenant, uuid.NewString(), uuid.NewString(), "2025-09-01")
		if err := repo.Create(ctx, tenant, rec); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetByID(ctx, tenant, rec.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status != "present" || got.Date.String() != "2025-09-01" {
			t.Fatalf("round trip changed the record: %+v", got)
		}
		if _, err := repo.GetByID(tenantCtx(other), other, rec.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read returned %v; want ErrNotFound", err)
		}
		if err := repo.Delete(tenantCtx(other), other, rec.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant delete returned %v; want ErrNotFound", err)
		}
	})

	t.Run("marking a register twice updates rather than duplicates", func(t *testing.T) {
		studentID, yearID := uuid.NewString(), uuid.NewString()
		first := newRecord(t, tenant, studentID, yearID, "2025-09-02")
		if err := repo.UpsertMany(ctx, tenant, []*domain.AttendanceRecord{first}); err != nil {
			t.Fatalf("first register: %v", err)
		}

		second := newRecord(t, tenant, studentID, yearID, "2025-09-02")
		second.Status = "absent"
		if err := repo.UpsertMany(ctx, tenant, []*domain.AttendanceRecord{second}); err != nil {
			t.Fatalf("second register: %v", err)
		}

		got, _, err := repo.List(ctx, tenant, ports.ListFilter{
			StudentID: studentID, AcademicYearID: yearID, Date: "2025-09-02", Limit: 50,
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("re-marking a register produced %d records; want 1", len(got))
		}
		if got[0].Status != "absent" {
			t.Fatalf("re-marking did not update the status: %q", got[0].Status)
		}
	})

	t.Run("list filters and pages without repeating", func(t *testing.T) {
		yearID := uuid.NewString()
		for range 3 {
			rec := newRecord(t, tenant, uuid.NewString(), yearID, "2025-09-03")
			if err := repo.Create(ctx, tenant, rec); err != nil {
				t.Fatalf("create: %v", err)
			}
		}
		seen := map[string]bool{}
		cursor := ""
		for range 20 {
			page, next, err := repo.List(ctx, tenant, ports.ListFilter{
				AcademicYearID: yearID, Limit: 2, Cursor: cursor,
			})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			for _, rec := range page {
				if rec.TenantID != tenant {
					t.Fatalf("list leaked a record owned by %q", rec.TenantID)
				}
				if seen[rec.ID] {
					t.Fatalf("the cursor repeated record %s", rec.ID)
				}
				seen[rec.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		if len(seen) != 3 {
			t.Fatalf("paging saw %d records; want the 3 created", len(seen))
		}
	})

	t.Run("a lifecycle register persists records and queues an event each", func(t *testing.T) {
		drainAttendance(t, repo)
		yearID := uuid.NewString()
		records := []*domain.AttendanceRecord{
			newRecord(t, tenant, uuid.NewString(), yearID, "2025-09-04"),
			newRecord(t, tenant, uuid.NewString(), yearID, "2025-09-04"),
		}
		payloads := []map[string]any{
			ports.AttendanceEventData(records[0], nil),
			ports.AttendanceEventData(records[1], nil),
		}
		if err := repo.CommitAttendanceLifecycle(ctx, tenant, ports.AttendanceMutationBulkUpsert,
			records, "attendance.marked.v1", payloads); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}

		got, _, err := repo.List(ctx, tenant, ports.ListFilter{AcademicYearID: yearID, Limit: 50})
		if err != nil || len(got) != 2 {
			t.Fatalf("lifecycle register persisted %d records err=%v", len(got), err)
		}

		claimed, err := repo.ClaimPendingAttendanceEvents(ctx, 100)
		if err != nil || len(claimed) != 2 {
			t.Fatalf("claimed %d events; want 2 err=%v", len(claimed), err)
		}
		for _, e := range claimed {
			if e.EventType != "attendance.marked.v1" {
				t.Fatalf("unexpected event type %q", e.EventType)
			}
			if err := repo.MarkAttendanceEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("mark published: %v", err)
			}
		}
		again, err := repo.ClaimPendingAttendanceEvents(ctx, 100)
		if err != nil || len(again) != 0 {
			t.Fatalf("published events were claimed again: %d err=%v", len(again), err)
		}
	})

	t.Run("a failed event is retried rather than dropped", func(t *testing.T) {
		drainAttendance(t, repo)
		rec := newRecord(t, tenant, uuid.NewString(), uuid.NewString(), "2025-09-05")
		if err := repo.CommitAttendanceLifecycle(ctx, tenant, ports.AttendanceMutationCreate,
			[]*domain.AttendanceRecord{rec}, "attendance.marked.v1",
			[]map[string]any{ports.AttendanceEventData(rec, nil)}); err != nil {
			t.Fatalf("commit lifecycle: %v", err)
		}
		claimed, err := repo.ClaimPendingAttendanceEvents(ctx, 100)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("claim: %d err=%v", len(claimed), err)
		}
		if err := repo.MarkAttendanceEventFailed(ctx, claimed[0].ID, "bus unavailable"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		if err := repo.MarkAttendanceEventPublished(ctx, claimed[0].ID); err != nil {
			t.Fatalf("a failed event could not be acknowledged later: %v", err)
		}
	})
}

func drainAttendance(t *testing.T, repo attendanceRepo) {
	t.Helper()
	ctx := tenantCtx("upshs")
	for range 50 {
		claimed, err := repo.ClaimPendingAttendanceEvents(ctx, 100)
		if err != nil {
			t.Fatalf("drain: %v", err)
		}
		if len(claimed) == 0 {
			return
		}
		for _, e := range claimed {
			if err := repo.MarkAttendanceEventPublished(ctx, e.ID); err != nil {
				t.Fatalf("drain acknowledge: %v", err)
			}
		}
	}
}

func TestPostgresAttendanceRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	pg := testkit.NewPostgres(ctx, t, "../../migrations")
	runAttendanceContract(t, pgadapter.NewRepository(pg.DB))
}

func TestMongoAttendanceRepositorySatisfiesTheContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB adapter conformance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	mg := testkit.NewMongo(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, mg.Store); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	runAttendanceContract(t, mongoadapter.NewRepository(mg.Store))
}
