package domain

import (
	"errors"
	"testing"
	"time"
)

func TestProgrammeAndIntakeAvailabilityRules(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	programme, err := NewProgramme("school-one", "sci", "General Science", "general-science", "Science programme", "Verified programme description", now)
	if err != nil || programme.Code != "SCI" || programme.Status != ProgrammeDraft {
		t.Fatalf("programme=%+v err=%v", programme, err)
	}
	intake, err := NewIntake("school-one", programme.ID, "September 2026", now.Add(60*24*time.Hour), now.Add(-time.Hour), now.Add(30*24*time.Hour), nil, now)
	if err != nil {
		t.Fatal(err)
	}
	open := IntakeOpen
	if err = intake.Apply(IntakeChanges{Status: &open}, now); err != nil || !intake.IsAvailable(now) {
		t.Fatalf("open intake=%+v err=%v", intake, err)
	}
	if intake.IsAvailable(intake.ApplicationClosesAt) {
		t.Fatal("intake must close exactly at application_closes_at")
	}
}

func TestCatalogueRejectsInvalidWindowsAndIdentifiers(t *testing.T) {
	now := time.Now().UTC()
	if _, err := NewProgramme("school-one", "bad code", "Programme", "programme", "Summary", "Description", now); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid code=%v", err)
	}
	programme, err := NewProgramme("school-one", "SCI", "Programme", "programme", "Summary", "Description", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewIntake("school-one", programme.ID, "September", now.Add(30*24*time.Hour), now.Add(10*24*time.Hour), now.Add(5*24*time.Hour), nil, now); !errors.Is(err, ErrValidation) {
		t.Fatalf("reversed window=%v", err)
	}
}
