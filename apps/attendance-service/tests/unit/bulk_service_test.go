package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	bulkTenant  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	bulkAY      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	bulkClass   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	bulkSubject = "abababab-abab-abab-abab-abababababab"
	bulkStu1    = "11111111-1111-1111-1111-111111111111"
	bulkStu2    = "22222222-2222-2222-2222-222222222222"
	bulkStu3    = "33333333-3333-3333-3333-333333333333"
)

type fakeRepo struct {
	upsertTenantID string
	upserted       []*domain.AttendanceRecord
	upsertErr      error
}

func (f *fakeRepo) Create(context.Context, string, *domain.AttendanceRecord) error {
	return errors.New("not implemented")
}

func (f *fakeRepo) UpsertMany(_ context.Context, tenantID string, records []*domain.AttendanceRecord) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upsertTenantID = tenantID
	f.upserted = append(f.upserted, records...)
	return nil
}

func (f *fakeRepo) GetByID(context.Context, string, string) (*domain.AttendanceRecord, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) List(context.Context, string, ports.ListFilter) ([]*domain.AttendanceRecord, string, error) {
	return nil, "", nil
}

func (f *fakeRepo) Update(context.Context, string, *domain.AttendanceRecord) error {
	return errors.New("not implemented")
}

func (f *fakeRepo) Delete(context.Context, string, string) error {
	return errors.New("not implemented")
}

type publishedEvent struct {
	eventType string
	record    *domain.AttendanceRecord
}

type fakePublisher struct{ events []publishedEvent }

func (f *fakePublisher) Publish(_ context.Context, eventType string, record *domain.AttendanceRecord, _ map[string]any) error {
	f.events = append(f.events, publishedEvent{eventType: eventType, record: record})
	return nil
}

func bulkCtx() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: bulkTenant})
}

func bulkActor(perms ...string) auth.Actor {
	return auth.Actor{UserID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", TenantID: bulkTenant, Permissions: perms}
}

func enabledGates() *flags.StaticSnapshot {
	g := flags.NewStaticSnapshot()
	g.Set(bulkTenant, application.FeatureAttendance, true)
	return g
}

func validBulkRequest() application.BulkMarkRequest {
	remark := "sick note"
	return application.BulkMarkRequest{
		AcademicYearID: bulkAY,
		Date:           "2025-09-01",
		ClassID:        strPtr(bulkClass),
		SubjectID:      strPtr(bulkSubject),
		Records: []application.BulkMarkRow{
			{StudentID: bulkStu1, Status: "present"},
			{StudentID: bulkStu2, Status: "absent", Remark: &remark},
			{StudentID: bulkStu3, Status: "late"},
		},
	}
}

func strPtr(v string) *string { return &v }

func newBulkService() (*application.Service, *fakeRepo, *fakePublisher) {
	repo := &fakeRepo{}
	pub := &fakePublisher{}
	svc := application.NewService(repo, application.WithPublisher(pub), application.WithFeatureGate(enabledGates()))
	return svc, repo, pub
}

func TestBulkMark_Success(t *testing.T) {
	svc, repo, pub := newBulkService()

	records, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermMark), validBulkRequest())
	if err != nil {
		t.Fatalf("bulk mark: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if repo.upsertTenantID != bulkTenant {
		t.Fatalf("repository not tenant-scoped: got tenant %q", repo.upsertTenantID)
	}
	if len(repo.upserted) != 3 {
		t.Fatalf("expected 3 upserted records, got %d", len(repo.upserted))
	}
	actor := bulkActor(application.PermMark)
	for i, rec := range repo.upserted {
		if rec.TenantID != bulkTenant {
			t.Fatalf("record %d tenant mismatch: %q", i, rec.TenantID)
		}
		if rec.MarkedBy != actor.UserID {
			t.Fatalf("record %d marked_by should be the actor: got %q", i, rec.MarkedBy)
		}
		if rec.AcademicYearID != bulkAY || rec.Date.String() != "2025-09-01" {
			t.Fatalf("record %d scope mismatch: %+v", i, rec)
		}
		if rec.ClassID == nil || *rec.ClassID != bulkClass {
			t.Fatalf("record %d class_id not set: %+v", i, rec.ClassID)
		}
		if rec.SubjectID == nil || *rec.SubjectID != bulkSubject {
			t.Fatalf("record %d subject_id not set: %+v", i, rec.SubjectID)
		}
	}
	if repo.upserted[1].Reason == nil || *repo.upserted[1].Reason != "sick note" {
		t.Fatalf("remark should map to reason: %+v", repo.upserted[1].Reason)
	}
	if len(pub.events) != 3 {
		t.Fatalf("expected 3 events (one per student), got %d", len(pub.events))
	}
	for i, ev := range pub.events {
		if ev.eventType != "attendance.marked.v1" {
			t.Fatalf("event %d type mismatch: %q", i, ev.eventType)
		}
	}
	if pub.events[0].record.StudentID != bulkStu1 || pub.events[1].record.StudentID != bulkStu2 || pub.events[2].record.StudentID != bulkStu3 {
		t.Fatalf("events should be per student in request order: %+v", pub.events)
	}
}

func TestBulkMark_ValidationCollectsEveryRow(t *testing.T) {
	svc, repo, pub := newBulkService()

	req := validBulkRequest()
	req.Records = []application.BulkMarkRow{
		{StudentID: bulkStu1, Status: "present"},
		{StudentID: "", Status: "present"},           // missing student
		{StudentID: "not-a-uuid", Status: "present"}, // invalid uuid
		{StudentID: bulkStu2, Status: "unknown"},     // invalid status
		{StudentID: bulkStu1, Status: "absent"},      // duplicate of row 0
	}

	_, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermMark), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	var rowErr *application.RowValidationError
	if !errors.As(err, &rowErr) {
		t.Fatalf("expected RowValidationError, got %T", err)
	}
	for _, key := range []string{"records[1].student_id", "records[2].student_id", "records[3].status", "records[4].student_id"} {
		if _, ok := rowErr.Rows[key]; !ok {
			t.Fatalf("expected row error %q, got %v", key, rowErr.Rows)
		}
	}
	if len(repo.upserted) != 0 {
		t.Fatalf("all-or-nothing: repository must not be called on validation failure, got %d records", len(repo.upserted))
	}
	if len(pub.events) != 0 {
		t.Fatalf("no events may be emitted on validation failure, got %d", len(pub.events))
	}
}

func TestBulkMark_TopLevelValidation(t *testing.T) {
	svc, repo, _ := newBulkService()

	req := validBulkRequest()
	req.AcademicYearID = ""
	req.Date = "01/09/2025"
	req.ClassID = strPtr("not-a-uuid")

	_, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermMark), req)
	var rowErr *application.RowValidationError
	if !errors.As(err, &rowErr) {
		t.Fatalf("expected RowValidationError, got %v", err)
	}
	for _, key := range []string{"academic_year_id", "date", "class_id"} {
		if _, ok := rowErr.Rows[key]; !ok {
			t.Fatalf("expected error for %q, got %v", key, rowErr.Rows)
		}
	}
	if len(repo.upserted) != 0 {
		t.Fatal("repository must not be called on validation failure")
	}
}

func TestBulkMark_EmptyRecords(t *testing.T) {
	svc, repo, _ := newBulkService()

	req := validBulkRequest()
	req.Records = nil

	_, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermMark), req)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for empty records, got %v", err)
	}
	if len(repo.upserted) != 0 {
		t.Fatal("repository must not be called for empty records")
	}
}

func TestBulkMark_PermissionDenied(t *testing.T) {
	svc, repo, pub := newBulkService()

	_, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermRead), validBulkRequest())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden without attendance.mark, got %v", err)
	}
	if len(repo.upserted) != 0 || len(pub.events) != 0 {
		t.Fatal("nothing may be persisted or published on permission denial")
	}
}

func TestBulkMark_Unauthenticated(t *testing.T) {
	svc, _, _ := newBulkService()

	_, err := svc.BulkMark(bulkCtx(), auth.Actor{}, validBulkRequest())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for unauthenticated actor, got %v", err)
	}
}

func TestBulkMark_CrossTenantRejected(t *testing.T) {
	svc, repo, _ := newBulkService()

	actor := bulkActor(application.PermMark)
	actor.TenantID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	_, err := svc.BulkMark(bulkCtx(), actor, validBulkRequest())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for cross-tenant actor, got %v", err)
	}
	if len(repo.upserted) != 0 {
		t.Fatal("repository must not be called for cross-tenant actor")
	}
}

func TestBulkMark_MissingTenantContext(t *testing.T) {
	svc, _, _ := newBulkService()

	_, err := svc.BulkMark(context.Background(), bulkActor(application.PermMark), validBulkRequest())
	if !errors.Is(err, domain.ErrMissingTenant) {
		t.Fatalf("expected ErrMissingTenant, got %v", err)
	}
}

func TestBulkMark_FeatureDisabled(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{}
	gates := flags.NewStaticSnapshot()
	gates.Set(bulkTenant, application.FeatureAttendance, false)
	svc := application.NewService(repo, application.WithPublisher(pub), application.WithFeatureGate(gates))

	_, err := svc.BulkMark(bulkCtx(), bulkActor(application.PermMark), validBulkRequest())
	if !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
	if len(repo.upserted) != 0 || len(pub.events) != 0 {
		t.Fatal("nothing may be persisted or published when the feature is disabled")
	}
}
