package unit

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/application"
	"github.com/auraedu/student-service/internal/domain"
)

// capturedEvent records one EventPublisher call.
type capturedEvent struct {
	eventType string
	student   *domain.Student
	meta      map[string]any
}

// capturePublisher is a ports.EventPublisher that records emitted events.
type capturePublisher struct {
	events []capturedEvent
}

func (p *capturePublisher) Publish(_ context.Context, eventType string, student *domain.Student, meta map[string]any) error {
	p.events = append(p.events, capturedEvent{eventType: eventType, student: student, meta: meta})
	return nil
}

func newEventService() (*application.Service, *capturePublisher) {
	pub := &capturePublisher{}
	gates := flags.NewStaticSnapshot()
	gates.Set(tenantA, application.FeatureStudentManagement, true)
	svc := application.NewService(newFakeRepo(),
		application.WithFeatureGate(gates),
		application.WithPublisher(pub),
	)
	return svc, pub
}

func eventCtx() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})
}

func eventActor(perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: perms}
}

func TestService_Create_PublishesStudentEnrolled(t *testing.T) {
	svc, pub := newEventService()
	classID := "22222222-2222-2222-2222-222222222222"
	academicYearID := "33333333-3333-3333-3333-333333333333"

	created, err := svc.Create(eventCtx(), eventActor(application.PermCreate), application.CreateStudentRequest{
		FirstName:      "Kwame",
		LastName:       "Nkrumah",
		ClassID:        &classID,
		AcademicYearID: &academicYearID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "student.enrolled.v1" {
		t.Fatalf("expected student.enrolled.v1 (contracts/events/student.enrolled.v1.json), got %q", got.eventType)
	}
	if got.student == nil || got.student.ID != created.ID {
		t.Fatalf("event not bound to created student: %+v", got.student)
	}
	wantDate := created.CreatedAt.UTC().Format(time.DateOnly)
	if got.meta["enrollment_date"] != wantDate {
		t.Errorf("enrollment_date: expected %q, got %v", wantDate, got.meta["enrollment_date"])
	}
	if got.meta["class_id"] != classID {
		t.Errorf("class_id: expected %q, got %v", classID, got.meta["class_id"])
	}
	if got.meta["academic_year_id"] != academicYearID {
		t.Errorf("academic_year_id: expected %q, got %v", academicYearID, got.meta["academic_year_id"])
	}
}

func TestService_Create_WithoutClass_OmitsEnrollmentIDs(t *testing.T) {
	svc, pub := newEventService()

	created, err := svc.Create(eventCtx(), eventActor(application.PermCreate), application.CreateStudentRequest{
		FirstName: "Ama",
		LastName:  "Serwaa",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "student.enrolled.v1" {
		t.Fatalf("expected student.enrolled.v1, got %q", got.eventType)
	}
	if _, ok := got.meta["class_id"]; ok {
		t.Errorf("class_id should be omitted when not supplied, got %v", got.meta["class_id"])
	}
	if _, ok := got.meta["academic_year_id"]; ok {
		t.Errorf("academic_year_id should be omitted when not supplied, got %v", got.meta["academic_year_id"])
	}
	wantDate := created.CreatedAt.UTC().Format(time.DateOnly)
	if got.meta["enrollment_date"] != wantDate {
		t.Errorf("enrollment_date: expected %q, got %v", wantDate, got.meta["enrollment_date"])
	}
}

func TestService_Update_PublishesStudentUpdated(t *testing.T) {
	svc, pub := newEventService()
	ctx := eventCtx()
	actor := eventActor(application.PermCreate, application.PermUpdate)

	created, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Yaa", LastName: "Asantewaa"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pub.events = nil

	first := "Nana"
	if _, err := svc.Update(ctx, actor, created.ID, application.UpdateStudentRequest{FirstName: &first}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	got := pub.events[0]
	if got.eventType != "student.updated.v1" {
		t.Fatalf("expected student.updated.v1, got %q", got.eventType)
	}
	if got.student == nil || got.student.ID != created.ID {
		t.Fatalf("event not bound to updated student: %+v", got.student)
	}
	changed, ok := got.meta["changed_fields"].([]string)
	if !ok || !reflect.DeepEqual(changed, []string{"first_name"}) {
		t.Errorf("changed_fields: expected [first_name], got %v", got.meta["changed_fields"])
	}
}

func TestService_Update_NoChange_PublishesNothing(t *testing.T) {
	svc, pub := newEventService()
	ctx := eventCtx()
	actor := eventActor(application.PermCreate, application.PermUpdate)

	created, err := svc.Create(ctx, actor, application.CreateStudentRequest{FirstName: "Kofi", LastName: "Annan"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pub.events = nil

	if _, err := svc.Update(ctx, actor, created.ID, application.UpdateStudentRequest{}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(pub.events) != 0 {
		t.Fatalf("expected no events for a no-op update, got %d", len(pub.events))
	}
}

func TestService_Import_PublishesStudentEnrolledPerRow(t *testing.T) {
	svc, pub := newEventService()

	result, err := svc.ImportStudents(eventCtx(), eventActor(application.PermCreate), []application.ImportStudentRow{
		{FirstName: "Ada", LastName: "Lovelace"},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.StudentsCreated != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(pub.events))
	}
	if got := pub.events[0]; got.eventType != "student.enrolled.v1" {
		t.Fatalf("expected student.enrolled.v1, got %q", got.eventType)
	}
}
