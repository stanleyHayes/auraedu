package integration

import (
	"context"
	"testing"

	"github.com/auraedu/student-service/internal/application"
)

func TestService_ImportStudents(t *testing.T) {
	ctx := withTenant(context.Background(), tenantA)
	svc, _ := newService(t)
	actor := actorWith(application.PermCreate, application.PermRead)

	result, err := svc.ImportStudents(ctx, actor, []application.ImportStudentRow{
		{
			FirstName:         "Kwame",
			LastName:          "Nkrumah",
			DateOfBirth:       strPtr("2005-01-15"),
			Gender:            strPtr("male"),
			Relationship:      strPtr("father"),
			GuardianFirstName: strPtr("Father"),
			GuardianLastName:  strPtr("Guardian"),
			GuardianPhone:     strPtr("+233"),
			GuardianEmail:     strPtr("father@example.com"),
		},
		{
			FirstName:         "Yaa",
			LastName:          "Asantewaa",
			GuardianFirstName: strPtr("Mother"),
			GuardianLastName:  strPtr("Guardian"),
			GuardianEmail:     strPtr("mother@example.com"),
		},
		{
			FirstName: "",
			LastName:  "MissingName",
		},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.StudentsCreated != 2 {
		t.Fatalf("expected 2 students created, got %d", result.StudentsCreated)
	}
	if result.GuardiansCreated != 2 {
		t.Fatalf("expected 2 guardians created, got %d", result.GuardiansCreated)
	}
	if result.LinksCreated != 2 {
		t.Fatalf("expected 2 links created, got %d", result.LinksCreated)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 row error, got %d", len(result.Errors))
	}
}

func strPtr(s string) *string { return &s }
