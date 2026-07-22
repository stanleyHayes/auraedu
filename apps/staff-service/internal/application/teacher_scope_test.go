package application

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/staff-service/internal/domain"
)

type teacherScopeRepo struct{ staff *domain.Staff }

func (r teacherScopeRepo) Create(context.Context, string, *domain.Staff) error { return nil }
func (r teacherScopeRepo) GetByID(context.Context, string, string) (*domain.Staff, error) {
	return r.staff, nil
}
func (r teacherScopeRepo) GetByUserID(_ context.Context, tenantID, userID string) (*domain.Staff, error) {
	if r.staff == nil || r.staff.TenantID != tenantID || r.staff.UserID == nil || *r.staff.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return r.staff, nil
}
func (r teacherScopeRepo) List(context.Context, string, int, string) ([]*domain.Staff, string, error) {
	return nil, "", nil
}
func (r teacherScopeRepo) Update(context.Context, string, *domain.Staff) error { return nil }
func (r teacherScopeRepo) Delete(context.Context, string, string) error        { return nil }

func TestResolveTeacherScopeRequiresActiveTeacher(t *testing.T) {
	userID := "33333333-3333-4333-8333-333333333333"
	staff, err := domain.NewStaff("school-one", "Ama", "Teacher", string(domain.StaffTypeTeacher))
	if err != nil {
		t.Fatal(err)
	}
	staff.UserID = &userID
	svc := NewService(teacherScopeRepo{staff: staff})
	staffID, err := svc.ResolveTeacherScope(context.Background(), "school-one", userID)
	if err != nil || staffID != staff.ID {
		t.Fatalf("staff_id=%q err=%v", staffID, err)
	}
	staff.Deactivate()
	if _, err := svc.ResolveTeacherScope(context.Background(), "school-one", userID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("inactive teacher=%v", err)
	}
}
