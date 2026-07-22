package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
)

type scopeRepo struct {
	records map[string]*domain.AttendanceRecord
	last    ports.ListFilter
}

func (r *scopeRepo) Create(context.Context, string, *domain.AttendanceRecord) error       { return nil }
func (r *scopeRepo) UpsertMany(context.Context, string, []*domain.AttendanceRecord) error { return nil }
func (r *scopeRepo) GetByID(_ context.Context, _ string, id string) (*domain.AttendanceRecord, error) {
	if item := r.records[id]; item != nil {
		return item, nil
	}
	return nil, domain.ErrNotFound
}
func (r *scopeRepo) List(_ context.Context, _ string, f ports.ListFilter) ([]*domain.AttendanceRecord, string, error) {
	r.last = f
	var out []*domain.AttendanceRecord
	for _, item := range r.records {
		for _, id := range f.StudentIDs {
			if item.StudentID == id {
				out = append(out, item)
			}
		}
	}
	return out, "", nil
}
func (r *scopeRepo) Update(context.Context, string, *domain.AttendanceRecord) error { return nil }
func (r *scopeRepo) Delete(context.Context, string, string) error                   { return nil }

type scopeResolver struct {
	ids      []string
	classIDs []string
}

func (s scopeResolver) Resolve(context.Context, string, string, string) (ports.LearnerScope, error) {
	return ports.LearnerScope{StudentIDs: s.ids, ClassIDs: s.classIDs}, nil
}

func TestTeacherAttendanceWritesAreRestrictedToAssignedRoster(t *testing.T) {
	repo := &scopeRepo{records: map[string]*domain.AttendanceRecord{}}
	svc := application.NewService(repo, application.WithFeatureGate(enabledGate{}), application.WithLearnerScopeResolver(scopeResolver{ids: []string{"33333333-3333-4333-8333-333333333333"}, classIDs: []string{"44444444-4444-4444-8444-444444444444"}}))
	teacher := auth.Actor{UserID: "teacher-user", TenantID: "school-one", Role: "teacher", Permissions: []string{application.PermMark}}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-one"})
	classOwn := "44444444-4444-4444-8444-444444444444"
	_, err := svc.BulkMark(ctx, teacher, application.BulkMarkRequest{AcademicYearID: "11111111-1111-4111-8111-111111111111", Date: "2026-07-18", ClassID: &classOwn, Records: []application.BulkMarkRow{{StudentID: "22222222-2222-4222-8222-222222222222", Status: "present"}}})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned roster write=%v", err)
	}
}

type enabledGate struct{}

func (enabledGate) IsEnabled(context.Context, string, string) bool { return true }

func TestParentAttendanceIsRestrictedToLinkedStudents(t *testing.T) {
	own, err := domain.NewAttendanceRecord(
		"school-one", "student-own", "year-1", "2026-07-18", "present", "teacher-1", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	other, err := domain.NewAttendanceRecord(
		"school-one", "student-other", "year-1", "2026-07-18", "absent", "teacher-1", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	repo := &scopeRepo{records: map[string]*domain.AttendanceRecord{own.ID: own, other.ID: other}}
	svc := application.NewService(repo, application.WithFeatureGate(enabledGate{}), application.WithLearnerScopeResolver(scopeResolver{ids: []string{"student-own"}}))
	actor := auth.Actor{UserID: "parent-1", TenantID: "school-one", Role: "parent", Permissions: []string{application.PermRead}}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-one"})
	items, _, err := svc.List(ctx, actor, ports.ListFilter{Limit: 20})
	if err != nil || len(items) != 1 || items[0].StudentID != "student-own" {
		t.Fatalf("list=%+v err=%v filter=%+v", items, err, repo.last)
	}
	if _, err = svc.Get(ctx, actor, other.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unlinked get=%v", err)
	}
}

func TestLearnerAttendanceFailsClosedWithoutResolver(t *testing.T) {
	repo := &scopeRepo{records: map[string]*domain.AttendanceRecord{}}
	svc := application.NewService(repo, application.WithFeatureGate(enabledGate{}))
	actor := auth.Actor{UserID: "student-user", TenantID: "school-one", Role: "student", Permissions: []string{application.PermRead}}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: "school-one"})
	if _, _, err := svc.List(ctx, actor, ports.ListFilter{Limit: 20}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured scope=%v", err)
	}
}
