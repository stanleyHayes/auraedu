package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func regressionStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func validation(t *testing.T, s *pmongo.Store, name string, rule bson.M) {
	t.Helper()
	if err := s.Database().RunCommand(context.Background(), bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: rule}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Fatal(err)
	}
}
func TestMongoRegisterRollback(t *testing.T) {
	ctx := context.Background()
	s := regressionStore(t)
	r := NewRepository(s)
	day, err := domain.NewDate("2026-09-12")
	if err != nil {
		t.Fatal(err)
	}
	makeRecord := func(student string) *domain.AttendanceRecord {
		return &domain.AttendanceRecord{ID: uuid.NewString(), TenantID: "tenant", StudentID: student, AcademicYearID: "year", Date: day, Status: "present", MarkedBy: "teacher", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	good, bad := makeRecord("good"), makeRecord("reject")
	validation(t, s, RecordCollection, bson.M{"student_id": bson.M{"$ne": "reject"}})
	if err := r.CommitAttendanceLifecycle(ctx, "tenant", ports.AttendanceMutationBulkUpsert, []*domain.AttendanceRecord{good, bad}, "attendance.marked.v1", []map[string]any{{}, {}}); err == nil {
		t.Fatal("fault did not fail")
	}
	scope, err := r.records.ScopeTo("tenant")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := scope.CountDocuments(ctx, bson.M{}); err != nil || n != 0 {
		t.Fatalf("partial register %d %v", n, err)
	}
	validation(t, s, RecordCollection, bson.M{})
	if err := r.CommitAttendanceLifecycle(ctx, "tenant", ports.AttendanceMutationBulkUpsert, []*domain.AttendanceRecord{good, bad}, "attendance.marked.v1", []map[string]any{{}, {}}); err != nil {
		t.Fatal(err)
	}
	replacement := makeRecord("good")
	replacement.Status = "absent"
	if err := r.CommitAttendanceLifecycle(ctx, "tenant", ports.AttendanceMutationBulkUpsert, []*domain.AttendanceRecord{replacement}, "attendance.marked.v1", []map[string]any{{}}); err != nil {
		t.Fatal(err)
	}
	events, err := r.ClaimPendingAttendanceEvents(ctx, 100)
	if err != nil || len(events) != 3 {
		t.Fatalf("natural-key event lost %d %v", len(events), err)
	}
	updated, err := r.GetByID(ctx, "tenant", good.ID)
	if err != nil || updated.Status != "absent" {
		t.Fatalf("retry update %+v %v", updated, err)
	}
}
