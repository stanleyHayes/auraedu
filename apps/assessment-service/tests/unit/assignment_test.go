package unit

import (
	"testing"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
)

const (
	class1   = "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaa1"
	class2   = "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbb2"
	subject2 = "33333333-cccc-4ccc-8ccc-ccccccccccc3"
)

func TestNewAssignment_Valid(t *testing.T) {
	due := time.Date(2025, 11, 1, 23, 59, 0, 0, time.UTC)
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "Write 500 words", 50, &due, []string{class1, class2, class1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Type != string(domain.TypeAssignment) {
		t.Fatalf("expected type assignment, got %q", a.Type)
	}
	if !a.IsAssignment() {
		t.Fatal("IsAssignment should be true")
	}
	if len(a.ClassIDs) != 2 || a.ClassIDs[0] != class1 || a.ClassIDs[1] != class2 {
		t.Fatalf("class_ids not normalized/deduped: %v", a.ClassIDs)
	}
	if a.Description == nil || *a.Description != "Write 500 words" {
		t.Fatalf("instructions not mapped to description: %v", a.Description)
	}
	if a.Status != string(domain.StatusDraft) || a.PublishedAt != nil {
		t.Fatalf("expected unpublished draft, got status=%q published_at=%v", a.Status, a.PublishedAt)
	}
}

func TestNewAssignment_RequiresTitle(t *testing.T) {
	if _, err := domain.NewAssignment(tenantA, ay1, subject1, "", "", 50, nil, nil); err == nil {
		t.Fatal("expected error when title is empty")
	}
}

func TestNewAssignment_RequiresPositiveMaxScore(t *testing.T) {
	if _, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay", "", 0, nil, nil); err == nil {
		t.Fatal("expected error when max_score is zero")
	}
}

func TestNewAssignment_RejectsEmptyClassID(t *testing.T) {
	if _, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay", "", 50, nil, []string{class1, " "}); err == nil {
		t.Fatal("expected error for empty class id")
	}
}

func TestAssignment_ApplyAssignmentUpdate(t *testing.T) {
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "v1", 50, nil, []string{class1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	title := "Essay 1 (revised)"
	instructions := "v2"
	maxScore := 60
	due := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	changed, err := a.ApplyAssignmentUpdate(&title, &instructions, &maxScore, &due, []string{class2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 5 {
		t.Fatalf("expected 5 changed fields, got %v", changed)
	}
	if a.Title != title || a.MaxScore != maxScore || !a.DueDate.Equal(due) {
		t.Fatalf("assignment not updated: %+v", a)
	}
	if a.Description == nil || *a.Description != instructions {
		t.Fatalf("instructions not updated: %v", a.Description)
	}
	if len(a.ClassIDs) != 1 || a.ClassIDs[0] != class2 {
		t.Fatalf("class_ids not replaced: %v", a.ClassIDs)
	}
}

func TestAssignment_ApplyAssignmentUpdate_Invalid(t *testing.T) {
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "", 50, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	empty := ""
	if _, err := a.ApplyAssignmentUpdate(&empty, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for empty title")
	}
	zero := 0
	if _, err := a.ApplyAssignmentUpdate(nil, nil, &zero, nil, nil); err == nil {
		t.Fatal("expected error for zero max_score")
	}
}

func TestAssignment_Publish(t *testing.T) {
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "", 50, nil, []string{class1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := a.Publish(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != string(domain.StatusPublished) {
		t.Fatalf("expected published status, got %q", a.Status)
	}
	if a.PublishedAt == nil {
		t.Fatal("expected published_at to be set")
	}
}

func TestAssignment_Publish_AlreadyPublished(t *testing.T) {
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "", 50, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := a.Publish(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := a.Publish(); err == nil {
		t.Fatal("expected error when publishing an already-published assignment")
	}
}

func TestAssignment_Publish_Archived(t *testing.T) {
	a, err := domain.NewAssignment(tenantA, ay1, subject1, "Essay 1", "", 50, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a.Status = string(domain.StatusArchived)
	if err := a.Publish(); err == nil {
		t.Fatal("expected error when publishing an archived assignment")
	}
}

func TestAssignment_Publish_NonAssignment(t *testing.T) {
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := a.Publish(); err == nil {
		t.Fatal("expected error when publishing a non-assignment via the assignments API")
	}
}
