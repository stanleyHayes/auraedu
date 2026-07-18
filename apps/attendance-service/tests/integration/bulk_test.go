package integration

import (
	"context"
	"testing"

	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

const bulkClassID = "99999999-9999-9999-9999-999999999999"
const bulkSubjectID = "88888888-8888-8888-8888-888888888888"

func bulkRecord(t *testing.T, tenantID, studentID, status string) *domain.AttendanceRecord {
	t.Helper()
	rec, err := domain.NewAttendanceRecord(tenantID, studentID, ay1, "2025-09-01", status, staff1, nil)
	if err != nil {
		t.Fatalf("new attendance record: %v", err)
	}
	classID := bulkClassID
	subjectID := bulkSubjectID
	rec.ClassID = &classID
	rec.SubjectID = &subjectID
	return rec
}

func TestRepository_UpsertMany_InsertsWithClassAndSubject(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	recs := []*domain.AttendanceRecord{
		bulkRecord(t, tenantA, studentA, "present"),
		bulkRecord(t, tenantA, studentB, "absent"),
	}
	if err := repo.UpsertMany(ctx, tenantA, recs); err != nil {
		t.Fatalf("upsert many: %v", err)
	}

	got, err := repo.GetByID(ctx, tenantA, recs[0].ID)
	if err != nil {
		t.Fatalf("get upserted record: %v", err)
	}
	if got.ClassID == nil || *got.ClassID != bulkClassID {
		t.Fatalf("class_id not persisted: %+v", got.ClassID)
	}
	if got.SubjectID == nil || *got.SubjectID != bulkSubjectID {
		t.Fatalf("subject_id not persisted: %+v", got.SubjectID)
	}
	if got.Status != "present" {
		t.Fatalf("status mismatch: %q", got.Status)
	}
}

func TestRepository_UpsertMany_IdempotentOnUniqueKey(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	first := []*domain.AttendanceRecord{
		bulkRecord(t, tenantA, studentA, "present"),
		bulkRecord(t, tenantA, studentB, "present"),
	}
	if err := repo.UpsertMany(ctx, tenantA, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	originalIDs := []string{first[0].ID, first[1].ID}

	// Re-mark the same (tenant, student, academic year, date): fresh domain ids, changed
	// statuses. The upsert must update the existing rows, not duplicate them.
	reason := "field trip"
	second := []*domain.AttendanceRecord{
		bulkRecord(t, tenantA, studentA, "excused"),
		bulkRecord(t, tenantA, studentB, "absent"),
	}
	second[0].Reason = &reason
	if err := repo.UpsertMany(ctx, tenantA, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second[0].ID != originalIDs[0] || second[1].ID != originalIDs[1] {
		t.Fatalf("upsert should keep pre-existing row ids: got %q, %q want %q, %q",
			second[0].ID, second[1].ID, originalIDs[0], originalIDs[1])
	}

	page, _, err := repo.List(ctx, tenantA, ports.ListFilter{Limit: 10, Date: "2025-09-01"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 rows after idempotent re-mark, got %d", len(page))
	}

	got, err := repo.GetByID(ctx, tenantA, originalIDs[0])
	if err != nil {
		t.Fatalf("get after re-mark: %v", err)
	}
	if got.Status != "excused" || got.Reason == nil || *got.Reason != reason {
		t.Fatalf("re-mark did not update row: %+v", got)
	}
}

func TestRepository_UpsertMany_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t)

	aCtx := withTenant(ctx, tenantA)
	if err := repo.UpsertMany(aCtx, tenantA, []*domain.AttendanceRecord{
		bulkRecord(t, tenantA, studentA, "present"),
	}); err != nil {
		t.Fatalf("tenant A upsert: %v", err)
	}

	// Same natural key under another tenant is a different row (unique key is tenant-scoped).
	bCtx := withTenant(ctx, tenantB)
	if err := repo.UpsertMany(bCtx, tenantB, []*domain.AttendanceRecord{
		bulkRecord(t, tenantB, studentA, "absent"),
	}); err != nil {
		t.Fatalf("tenant B upsert: %v", err)
	}

	aPage, _, err := repo.List(aCtx, tenantA, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant A: %v", err)
	}
	if len(aPage) != 1 || aPage[0].Status != "present" || aPage[0].TenantID != tenantA {
		t.Fatalf("tenant A row wrong or polluted: %+v", aPage)
	}

	bPage, _, err := repo.List(bCtx, tenantB, ports.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(bPage) != 1 || bPage[0].Status != "absent" || bPage[0].TenantID != tenantB {
		t.Fatalf("tenant B should see only its own row: %+v", bPage)
	}
}

func TestService_BulkMark_EndToEndIdempotent(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	repo := newRepo(t)

	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureAttendance, true)
	svc := application.NewService(repo, application.WithFeatureGate(gates))
	actor := auth.Actor{UserID: staff1, TenantID: tenantA, Permissions: []string{application.PermMark}}

	classID := bulkClassID
	req := application.BulkMarkRequest{
		AcademicYearID: ay1,
		Date:           "2025-09-01",
		ClassID:        &classID,
		Records: []application.BulkMarkRow{
			{StudentID: studentA, Status: "present"},
			{StudentID: studentB, Status: "absent"},
		},
	}
	records, err := svc.BulkMark(ctx, actor, req)
	if err != nil {
		t.Fatalf("bulk mark: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Retrying the same class+date mark updates in place instead of duplicating.
	req.Records[0].Status = "late"
	if _, err := svc.BulkMark(ctx, actor, req); err != nil {
		t.Fatalf("retry bulk mark: %v", err)
	}

	page, _, err := repo.List(ctx, tenantA, ports.ListFilter{Limit: 10, Date: "2025-09-01"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 rows after retry, got %d", len(page))
	}
	if page[0].MarkedBy != actor.UserID {
		t.Fatalf("marked_by should be the actor, got %q", page[0].MarkedBy)
	}
}
