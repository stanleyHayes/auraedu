package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Enrollment is the durable history of a learner's class assignment for an
// academic year. The Student aggregate keeps the current assignment as a fast
// roster projection; this record remains the historical source of truth.
type Enrollment struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	StudentID      string    `json:"student_id"`
	ClassID        string    `json:"class_id"`
	AcademicYearID string    `json:"academic_year_id"`
	EnrolledAt     time.Time `json:"enrolled_at"`
}

func NewEnrollment(tenantID, studentID, classID, academicYearID string, enrolledAt time.Time) (*Enrollment, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	for name, value := range map[string]string{"student_id": studentID, "class_id": classID, "academic_year_id": academicYearID} {
		if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
			return nil, fmt.Errorf("%w: %s must be a UUID", ErrValidation, name)
		}
	}
	if enrolledAt.IsZero() {
		enrolledAt = time.Now().UTC()
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("student: generate enrollment id: %w", err)
	}
	return &Enrollment{
		ID: id.String(), TenantID: tenantID, StudentID: studentID,
		ClassID: classID, AcademicYearID: academicYearID, EnrolledAt: enrolledAt.UTC(),
	}, nil
}
