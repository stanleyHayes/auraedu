package domain

import (
	"testing"
	"time"
)

const journeyTemplateID = "11111111-1111-1111-1111-111111111111"

func validJourneyInput() NewJourneyInput {
	start, end := 20*60, 7*60
	return NewJourneyInput{
		TenantID: "upshs", Name: "Application nurture", TriggerEvent: "application.started.v1",
		Timezone: "Africa/Accra", QuietHoursStartMinute: &start, QuietHoursEndMinute: &end,
		FrequencyWindowHours: 168, FrequencyLimit: 3,
		CancelOnEvents: []string{"application.submitted.v1"}, CreatedBy: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Steps: []JourneyStep{{Channel: "whatsapp", TemplateID: journeyTemplateID, DelayMinutes: 30, ConditionOperator: "always"}},
	}
}

func TestNewJourneyValidatesPolicyAndIndependentActivation(t *testing.T) {
	journey, err := NewJourney(validJourneyInput())
	if err != nil {
		t.Fatalf("new journey: %v", err)
	}
	if journey.Status != "draft" || journey.Steps[0].Position != 1 || journey.Steps[0].ID == "" {
		t.Fatalf("unexpected journey: %+v", journey)
	}
	if err := journey.Activate("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", time.Now()); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := journey.Activate("cccccccc-cccc-cccc-cccc-cccccccccccc", time.Now()); err == nil {
		t.Fatal("active journey was activated twice")
	}
}

func TestNewJourneyRejectsUnsafeConditionsAndDuration(t *testing.T) {
	input := validJourneyInput()
	input.Steps[0].ConditionOperator = "equals"
	input.Steps[0].ConditionField = "email"
	input.Steps[0].ConditionValue = "prospect@example.com"
	if _, err := NewJourney(input); err == nil {
		t.Fatal("PII condition field was accepted")
	}

	input = validJourneyInput()
	input.Steps = []JourneyStep{
		{Channel: "email", TemplateID: journeyTemplateID, DelayMinutes: 129600, ConditionOperator: "always"},
		{Channel: "email", TemplateID: journeyTemplateID, DelayMinutes: 129600, ConditionOperator: "always"},
		{Channel: "email", TemplateID: journeyTemplateID, DelayMinutes: 1, ConditionOperator: "always"},
	}
	if _, err := NewJourney(input); err == nil {
		t.Fatal("journey longer than 180 days was accepted")
	}
}

func TestJourneyQuietHoursCrossMidnight(t *testing.T) {
	journey, err := NewJourney(validJourneyInput())
	if err != nil {
		t.Fatal(err)
	}
	proposed := time.Date(2026, time.July, 21, 22, 30, 0, 0, time.UTC)
	got := journey.NextAllowedTime(proposed)
	want := time.Date(2026, time.July, 22, 7, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next allowed=%s want=%s", got, want)
	}
	daytime := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	if got := journey.NextAllowedTime(daytime); !got.Equal(daytime) {
		t.Fatalf("allowed daytime moved to %s", got)
	}
}

func TestJourneyTemplateRenderingIsAllowlistedAndFailClosed(t *testing.T) {
	if err := ValidateJourneyTemplateVariables("Hello {{ first_name }}, see {{programme_id}}"); err != nil {
		t.Fatalf("valid variables: %v", err)
	}
	if err := ValidateJourneyTemplateVariables("Token {{access_token}}"); err == nil {
		t.Fatal("unsafe template variable was accepted")
	}
	rendered, err := RenderJourneyTemplate("Hello {{first_name}}", map[string]string{"first_name": "Ama"})
	if err != nil || rendered != "Hello Ama" {
		t.Fatalf("rendered=%q err=%v", rendered, err)
	}
	if _, err := RenderJourneyTemplate("Hello {{first_name}}", map[string]string{}); err == nil {
		t.Fatal("missing personalization value was silently rendered")
	}
}
