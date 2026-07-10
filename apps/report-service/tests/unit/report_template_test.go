package unit

import (
	"testing"

	"github.com/auraedu/report-service/internal/domain"
)

const tenantA = "11111111-1111-1111-1111-111111111111"
const ay1 = "cccccccc-cccc-cccc-cccc-cccccccccccc"

func TestNewReportTemplate_RequiresTenant(t *testing.T) {
	if _, err := domain.NewReportTemplate("", "Midterm", ay1, "# Template"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewReportTemplate_RequiresName(t *testing.T) {
	if _, err := domain.NewReportTemplate(tenantA, "", ay1, "# Template"); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewReportTemplate_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewReportTemplate(tenantA, "Midterm", "", "# Template"); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewReportTemplate_RequiresBodyTemplate(t *testing.T) {
	if _, err := domain.NewReportTemplate(tenantA, "Midterm", ay1, ""); err == nil {
		t.Fatal("expected error when body_template is empty")
	}
}

func TestNewReportTemplate_Valid(t *testing.T) {
	tmpl, err := domain.NewReportTemplate(tenantA, "Midterm", ay1, "# Report")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.TenantID != tenantA {
		t.Fatalf("tenant not set: got %q", tmpl.TenantID)
	}
	if tmpl.Status != string(domain.TemplateStatusDraft) {
		t.Fatalf("expected draft status, got %q", tmpl.Status)
	}
}

func TestReportTemplate_ApplyUpdate(t *testing.T) {
	tmpl, err := domain.NewReportTemplate(tenantA, "Midterm", ay1, "# Report")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "Final"
	status := string(domain.TemplateStatusActive)
	changed, err := tmpl.ApplyUpdate(&name, nil, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if tmpl.Name != name {
		t.Fatalf("name not updated: got %q", tmpl.Name)
	}
	if tmpl.Status != status {
		t.Fatalf("status not updated: got %q", tmpl.Status)
	}
}

func TestReportTemplate_ApplyUpdate_InvalidStatus(t *testing.T) {
	tmpl, err := domain.NewReportTemplate(tenantA, "Midterm", ay1, "# Report")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := tmpl.ApplyUpdate(nil, nil, nil, &status); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestReportTemplate_Validate_InvalidStatus(t *testing.T) {
	tmpl, _ := domain.NewReportTemplate(tenantA, "Midterm", ay1, "# Report")
	tmpl.Status = "unknown"
	if err := tmpl.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
