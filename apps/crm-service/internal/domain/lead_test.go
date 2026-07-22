package domain

import "testing"

func ptr(value string) *string { return &value }

func TestNewLeadNormalizesContactAndStartsNew(t *testing.T) {
	lead, err := NewLead("school-a", " Ama ", " Mensah ", ptr(" AMA@Example.COM "), ptr("+233 20 123 4567"), "website", Consent{PrivacyNoticeVersion: "2026-01", Email: true})
	if err != nil {
		t.Fatalf("NewLead: %v", err)
	}
	if *lead.Email != "ama@example.com" || *lead.Phone != "+233201234567" {
		t.Fatalf("unexpected normalized contact: email=%q phone=%q", *lead.Email, *lead.Phone)
	}
	if lead.Stage != StageNew || lead.TenantID != "school-a" {
		t.Fatalf("unexpected initial state: %#v", lead)
	}
}

func TestNewLeadRequiresTenantContactAndPrivacyVersion(t *testing.T) {
	tests := []struct {
		name, tenant string
		email, phone *string
		consent      Consent
	}{
		{name: "tenant", email: ptr("a@example.com"), consent: Consent{PrivacyNoticeVersion: "v1"}},
		{name: "contact", tenant: "school-a", consent: Consent{PrivacyNoticeVersion: "v1"}},
		{name: "privacy", tenant: "school-a", email: ptr("a@example.com")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewLead(tt.tenant, "Ama", "Mensah", tt.email, tt.phone, "website", tt.consent); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSetStageRejectsUnknownValue(t *testing.T) {
	lead, err := NewLead("school-a", "Ama", "Mensah", ptr("a@example.com"), nil, "website", Consent{PrivacyNoticeVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := lead.SetStage("invented"); err == nil {
		t.Fatal("expected unknown stage to fail")
	}
	if err := lead.SetStage(StageQualified); err != nil || lead.Stage != StageQualified {
		t.Fatalf("expected qualified stage, got %q (%v)", lead.Stage, err)
	}
}
