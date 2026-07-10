package unit

import (
	"testing"

	"github.com/auraedu/report-service/internal/domain"
)

const studentA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
const template1 = "77777777-7777-7777-7777-777777777777"

func TestNewReportCard_RequiresTenant(t *testing.T) {
	if _, err := domain.NewReportCard("", studentA, ay1, template1); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewReportCard_RequiresStudentID(t *testing.T) {
	if _, err := domain.NewReportCard(tenantA, "", ay1, template1); err == nil {
		t.Fatal("expected error when student_id is empty")
	}
}

func TestNewReportCard_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewReportCard(tenantA, studentA, "", template1); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewReportCard_RequiresTemplateID(t *testing.T) {
	if _, err := domain.NewReportCard(tenantA, studentA, ay1, ""); err == nil {
		t.Fatal("expected error when template_id is empty")
	}
}

func TestNewReportCard_Valid(t *testing.T) {
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.TenantID != tenantA {
		t.Fatalf("tenant not set: got %q", card.TenantID)
	}
	if card.Status != string(domain.ReportCardStatusDraft) {
		t.Fatalf("expected draft status, got %q", card.Status)
	}
}

func TestReportCard_ApplyUpdate(t *testing.T) {
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	student := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	status := string(domain.ReportCardStatusArchived)
	changed, err := card.ApplyUpdate(&student, nil, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if card.StudentID != student {
		t.Fatalf("student_id not updated: got %q", card.StudentID)
	}
	if card.Status != status {
		t.Fatalf("status not updated: got %q", card.Status)
	}
}

func TestReportCard_ApplyUpdate_InvalidStatus(t *testing.T) {
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := card.ApplyUpdate(nil, nil, nil, &status); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestReportCard_SetPublished(t *testing.T) {
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	card.SetPublished("/tmp/reports/test.pdf")
	if card.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published status, got %q", card.Status)
	}
	if card.PDFPath == nil || *card.PDFPath != "/tmp/reports/test.pdf" {
		t.Fatalf("expected pdf_path to be set")
	}
	if card.GeneratedAt == nil {
		t.Fatal("expected generated_at to be set")
	}
}

func TestReportCard_Validate_InvalidStatus(t *testing.T) {
	card, _ := domain.NewReportCard(tenantA, studentA, ay1, template1)
	card.Status = "unknown"
	if err := card.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
