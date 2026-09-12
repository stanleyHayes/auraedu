package adapters_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraedu/platform/testkit"
	mongoadapter "github.com/auraedu/student-service/internal/adapters/mongo"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
)

func TestMongoStudentTransactionalRegressions(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongoReplicaSet(ctx, t)
	if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
		t.Fatal(err)
	}
	r := mongoadapter.NewRepository(db.Store)
	s := newStudent("a")
	badClass := "not-a-uuid"
	year := uuid.NewString()
	s.ClassID = &badClass
	s.AcademicYearID = &year
	if err := r.CommitStudentLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: s}, "student.created.v1", map[string]any{"student_id": s.ID}); err == nil {
		t.Fatal("invalid enrollment accepted")
	}
	if _, err := r.GetByID(ctx, "a", s.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("student survived enrollment failure", err)
	}
	events, err := r.ClaimPendingStudentEvents(ctx, 100)
	if err != nil || len(events) != 0 {
		t.Fatal("failed creation published an event", err)
	}
	class := uuid.NewString()
	s.ClassID = &class
	if err := r.Create(ctx, "a", s); err != nil {
		t.Fatal(err)
	}
	enrollments, _, err := r.ListEnrollments(ctx, "a", s.ID, 100, "")
	if err != nil || len(enrollments) != 1 {
		t.Fatalf("initial enrollment missing %+v %v", enrollments, err)
	}
	newYear := uuid.NewString()
	newClass := uuid.NewString()
	var wins atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			e, err := domain.NewEnrollment("a", s.ID, newClass, newYear, time.Now())
			if err != nil {
				t.Error(err)
				return
			}
			err = r.CommitStudentLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.MutationEnrollmentCreate, Enrollment: e}, "student.enrolled.v1", map[string]any{"student_id": s.ID})
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, domain.ErrConflict) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("enrollment winners %d", wins.Load())
	}
	got, err := r.GetByID(ctx, "a", s.ID)
	if err != nil || got.ClassID == nil || *got.ClassID != newClass {
		t.Fatal("roster projection not updated", err)
	}
	events, err = r.ClaimPendingStudentEvents(ctx, 100)
	if err != nil || len(events) != 1 || events[0].EventType != "student.enrolled.v1" {
		t.Fatalf("enrollment event absent %+v %v", events, err)
	}
	g := newGuardian("a")
	if err = r.CreateGuardian(ctx, "a", g); err != nil {
		t.Fatal(err)
	}
	link := &domain.StudentGuardian{ID: uuid.NewString(), TenantID: "a", StudentID: s.ID, GuardianID: g.ID, CreatedAt: time.Now()}
	if err = r.CommitStudentLifecycle(ctx, "a", ports.LifecycleMutation{Kind: ports.MutationGuardianLink, Link: link}, "", nil); err == nil {
		t.Fatal("invalid event accepted")
	}
	gs, _, err := r.ListGuardiansByStudent(ctx, "a", s.ID, 100, "")
	if err != nil || len(gs) != 0 {
		t.Fatal("link survived event failure", err)
	}
	if err = r.LinkGuardianToStudent(ctx, "b", link); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-tenant link accepted", err)
	}
	if err = r.Delete(ctx, "a", s.ID); err != nil {
		t.Fatal(err)
	}
	enrollments, _, err = r.ListEnrollments(ctx, "a", s.ID, 100, "")
	if err != nil || len(enrollments) != 0 {
		t.Fatal("deleted student still has visible enrollments", err)
	}
}

func TestMongoStudentIndexesRepeatedConcurrentStartup(t *testing.T) {
	ctx := context.Background()
	db := testkit.NewMongo(ctx, t)
	for range 10 {
		if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 3 {
				if err := mongoadapter.EnsureIndexes(ctx, db.Store); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
}
