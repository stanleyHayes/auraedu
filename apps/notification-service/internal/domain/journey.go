package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JourneyStatus is the reviewable lifecycle of a communication journey.
type JourneyStatus string

const (
	JourneyStatusDraft    JourneyStatus = "draft"
	JourneyStatusActive   JourneyStatus = "active"
	JourneyStatusPaused   JourneyStatus = "paused"
	JourneyStatusArchived JourneyStatus = "archived"
)

// JourneyConditionOperator controls whether a step is included for an event.
type JourneyConditionOperator string

const (
	JourneyConditionAlways    JourneyConditionOperator = "always"
	JourneyConditionEquals    JourneyConditionOperator = "equals"
	JourneyConditionNotEquals JourneyConditionOperator = "not_equals"
)

const (
	minJourneyFrequencyWindow = time.Hour
	maxJourneyFrequencyWindow = 30 * 24 * time.Hour
	maxJourneyDelay           = 90 * 24 * time.Hour
	maxJourneyDuration        = 180 * 24 * time.Hour
)

var journeyTemplateVariable = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]*)\s*\}\}`)

var journeyEvents = map[string]struct{}{ //nolint:gochecknoglobals // Immutable protocol allowlist shared by validation and worker subscription.
	"lead.created.v1":             {},
	"lead.interaction_created.v1": {},
	"lead.scored.v1":              {},
	"application.started.v1":      {},
	"application.submitted.v1":    {},
	"application.admitted.v1":     {},
	"offer.issued.v1":             {},
	"offer.accepted.v1":           {},
}

var journeyContextFields = map[string]struct{}{ //nolint:gochecknoglobals // Immutable privacy allowlist for event projection.
	"source": {}, "stage": {}, "campaign_id": {}, "programme_id": {},
	"intake_id": {}, "score": {}, "confidence": {}, "channel": {},
	"direction": {},
}

var journeyTemplateFields = map[string]struct{}{ //nolint:gochecknoglobals // Immutable personalization allowlist.
	"first_name": {}, "source": {}, "stage": {}, "campaign_id": {},
	"programme_id": {}, "intake_id": {}, "score": {}, "confidence": {},
	"channel": {}, "direction": {},
}

// JourneyStep is one consent-checked delivery in a linear journey. A condition
// may skip the step, which provides deterministic branches without executable
// expressions or arbitrary event access.
type JourneyStep struct {
	ID                string `json:"id"`
	Position          int    `json:"position"`
	Channel           string `json:"channel"`
	TemplateID        string `json:"template_id"`
	DelayMinutes      int    `json:"delay_minutes"`
	ConditionOperator string `json:"condition_operator"`
	ConditionField    string `json:"condition_field,omitempty"`
	ConditionValue    string `json:"condition_value,omitempty"`
}

// Journey defines a versioned, tenant-owned communication workflow.
type Journey struct {
	ID                    string        `json:"id"`
	TenantID              string        `json:"tenant_id"`
	Name                  string        `json:"name"`
	TriggerEvent          string        `json:"trigger_event"`
	Status                string        `json:"status"`
	Timezone              string        `json:"timezone"`
	QuietHoursStartMinute *int          `json:"quiet_hours_start_minute,omitempty"`
	QuietHoursEndMinute   *int          `json:"quiet_hours_end_minute,omitempty"`
	FrequencyWindowHours  int           `json:"frequency_window_hours"`
	FrequencyLimit        int           `json:"frequency_limit"`
	CancelOnEvents        []string      `json:"cancel_on_events"`
	Steps                 []JourneyStep `json:"steps"`
	Version               int           `json:"version"`
	CreatedBy             string        `json:"created_by"`
	ActivatedBy           *string       `json:"activated_by,omitempty"`
	ActivatedAt           *time.Time    `json:"activated_at,omitempty"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

type NewJourneyInput struct {
	TenantID              string
	Name                  string
	TriggerEvent          string
	Timezone              string
	QuietHoursStartMinute *int
	QuietHoursEndMinute   *int
	FrequencyWindowHours  int
	FrequencyLimit        int
	CancelOnEvents        []string
	Steps                 []JourneyStep
	CreatedBy             string
}

// NewJourney validates and constructs a draft journey.
func NewJourney(input NewJourneyInput) (*Journey, error) {
	journeyID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("notifications: generate journey id: %w", err)
	}
	steps := append([]JourneyStep(nil), input.Steps...)
	for index := range steps {
		stepID, idErr := uuid.NewV7()
		if idErr != nil {
			return nil, fmt.Errorf("notifications: generate journey step id: %w", idErr)
		}
		steps[index].ID = stepID.String()
		steps[index].Position = index + 1
		steps[index].Channel = strings.ToLower(strings.TrimSpace(steps[index].Channel))
		steps[index].TemplateID = strings.TrimSpace(steps[index].TemplateID)
		steps[index].ConditionOperator = strings.ToLower(strings.TrimSpace(steps[index].ConditionOperator))
		steps[index].ConditionField = strings.ToLower(strings.TrimSpace(steps[index].ConditionField))
		steps[index].ConditionValue = strings.TrimSpace(steps[index].ConditionValue)
	}
	now := time.Now().UTC()
	journey := &Journey{
		ID:                    journeyID.String(),
		TenantID:              strings.TrimSpace(input.TenantID),
		Name:                  strings.TrimSpace(input.Name),
		TriggerEvent:          strings.ToLower(strings.TrimSpace(input.TriggerEvent)),
		Status:                string(JourneyStatusDraft),
		Timezone:              strings.TrimSpace(input.Timezone),
		QuietHoursStartMinute: input.QuietHoursStartMinute,
		QuietHoursEndMinute:   input.QuietHoursEndMinute,
		FrequencyWindowHours:  input.FrequencyWindowHours,
		FrequencyLimit:        input.FrequencyLimit,
		CancelOnEvents:        normalizeJourneyEvents(input.CancelOnEvents),
		Steps:                 steps,
		Version:               1,
		CreatedBy:             strings.TrimSpace(input.CreatedBy),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := journey.Validate(); err != nil {
		return nil, err
	}
	return journey, nil
}

//nolint:gocyclo // Each branch enforces a separate, auditable communication safety invariant.
func (j Journey) Validate() error {
	if j.TenantID == "" {
		return ErrMissingTenant
	}
	if len([]rune(j.Name)) < 3 || len([]rune(j.Name)) > 160 {
		return fmt.Errorf("%w: journey name must contain 3 to 160 characters", ErrValidation)
	}
	if !IsJourneyEvent(j.TriggerEvent) {
		return fmt.Errorf("%w: unsupported journey trigger event", ErrValidation)
	}
	if !validJourneyStatus(JourneyStatus(j.Status)) {
		return fmt.Errorf("%w: invalid journey status", ErrValidation)
	}
	if len(j.Timezone) == 0 || len(j.Timezone) > 100 {
		return fmt.Errorf("%w: timezone must contain 1 to 100 characters", ErrValidation)
	}
	if _, err := time.LoadLocation(j.Timezone); err != nil {
		return fmt.Errorf("%w: timezone must be a valid IANA name", ErrValidation)
	}
	if (j.QuietHoursStartMinute == nil) != (j.QuietHoursEndMinute == nil) {
		return fmt.Errorf("%w: quiet hour start and end must be supplied together", ErrValidation)
	}
	if j.QuietHoursStartMinute != nil {
		if *j.QuietHoursStartMinute < 0 || *j.QuietHoursStartMinute > 1439 ||
			*j.QuietHoursEndMinute < 0 || *j.QuietHoursEndMinute > 1439 ||
			*j.QuietHoursStartMinute == *j.QuietHoursEndMinute {
			return fmt.Errorf("%w: quiet hours must be distinct minutes from 0 to 1439", ErrValidation)
		}
	}
	window := time.Duration(j.FrequencyWindowHours) * time.Hour
	if window < minJourneyFrequencyWindow || window > maxJourneyFrequencyWindow {
		return fmt.Errorf("%w: frequency window must be between 1 and 720 hours", ErrValidation)
	}
	if j.FrequencyLimit < 1 || j.FrequencyLimit > 100 {
		return fmt.Errorf("%w: frequency limit must be between 1 and 100", ErrValidation)
	}
	if len(j.Steps) == 0 || len(j.Steps) > 10 {
		return fmt.Errorf("%w: a journey must contain 1 to 10 steps", ErrValidation)
	}
	var totalDelay time.Duration
	for index, step := range j.Steps {
		if err := step.Validate(index + 1); err != nil {
			return err
		}
		totalDelay += time.Duration(step.DelayMinutes) * time.Minute
		if totalDelay > maxJourneyDuration {
			return fmt.Errorf("%w: journey duration cannot exceed 180 days", ErrValidation)
		}
	}
	if len(j.CancelOnEvents) > 8 {
		return fmt.Errorf("%w: a journey can have at most 8 cancellation events", ErrValidation)
	}
	for _, eventType := range j.CancelOnEvents {
		if !IsJourneyEvent(eventType) {
			return fmt.Errorf("%w: unsupported cancellation event", ErrValidation)
		}
	}
	if _, err := uuid.Parse(j.CreatedBy); err != nil {
		return fmt.Errorf("%w: created_by must be a UUID", ErrValidation)
	}
	return nil
}

func (s JourneyStep) Validate(expectedPosition int) error {
	if s.Position != expectedPosition {
		return fmt.Errorf("%w: journey step positions must be contiguous", ErrValidation)
	}
	if !isJourneyChannel(NotificationChannel(s.Channel)) {
		return fmt.Errorf("%w: journey channel must be email, sms, whatsapp or in_app", ErrValidation)
	}
	if _, err := uuid.Parse(s.TemplateID); err != nil {
		return fmt.Errorf("%w: journey template_id must be a UUID", ErrValidation)
	}
	delay := time.Duration(s.DelayMinutes) * time.Minute
	if s.DelayMinutes < 0 || delay > maxJourneyDelay {
		return fmt.Errorf("%w: step delay must be between 0 and 90 days", ErrValidation)
	}
	switch JourneyConditionOperator(s.ConditionOperator) {
	case JourneyConditionAlways:
		if s.ConditionField != "" || s.ConditionValue != "" {
			return fmt.Errorf("%w: always steps cannot include a condition field or value", ErrValidation)
		}
	case JourneyConditionEquals, JourneyConditionNotEquals:
		if _, ok := journeyContextFields[s.ConditionField]; !ok || s.ConditionValue == "" || len([]rune(s.ConditionValue)) > 160 {
			return fmt.Errorf("%w: invalid journey step condition", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: invalid journey condition operator", ErrValidation)
	}
	return nil
}

func (j *Journey) Activate(actorID string, now time.Time) error {
	if j.Status != string(JourneyStatusDraft) && j.Status != string(JourneyStatusPaused) {
		return fmt.Errorf("%w: only draft or paused journeys can be activated", ErrConflict)
	}
	actorID = strings.TrimSpace(actorID)
	if _, err := uuid.Parse(actorID); err != nil {
		return fmt.Errorf("%w: activation actor must be a UUID", ErrValidation)
	}
	now = now.UTC()
	j.Status = string(JourneyStatusActive)
	j.ActivatedBy = &actorID
	j.ActivatedAt = &now
	j.UpdatedAt = now
	return nil
}

func (j *Journey) Pause(now time.Time) error {
	if j.Status != string(JourneyStatusActive) {
		return fmt.Errorf("%w: only active journeys can be paused", ErrConflict)
	}
	j.Status = string(JourneyStatusPaused)
	j.UpdatedAt = now.UTC()
	return nil
}

func (j *Journey) Archive(now time.Time) error {
	if j.Status == string(JourneyStatusArchived) {
		return nil
	}
	j.Status = string(JourneyStatusArchived)
	j.UpdatedAt = now.UTC()
	return nil
}

func (s JourneyStep) Matches(context map[string]string) bool {
	switch JourneyConditionOperator(s.ConditionOperator) {
	case JourneyConditionAlways:
		return true
	case JourneyConditionEquals:
		return context[s.ConditionField] == s.ConditionValue
	case JourneyConditionNotEquals:
		return context[s.ConditionField] != s.ConditionValue
	default:
		return false
	}
}

// NextAllowedTime moves a proposed delivery out of configured quiet hours.
func (j Journey) NextAllowedTime(proposed time.Time) time.Time {
	if j.QuietHoursStartMinute == nil || j.QuietHoursEndMinute == nil {
		return proposed.UTC()
	}
	location, err := time.LoadLocation(j.Timezone)
	if err != nil {
		return proposed.UTC()
	}
	local := proposed.In(location)
	minute := local.Hour()*60 + local.Minute()
	start, end := *j.QuietHoursStartMinute, *j.QuietHoursEndMinute
	inQuiet := withinQuietHours(minute, start, end)
	if !inQuiet {
		return proposed.UTC()
	}
	year, month, day := local.Date()
	endHour, endMinute := end/60, end%60
	wake := time.Date(year, month, day, endHour, endMinute, 0, 0, location)
	if !wake.After(local) {
		wake = wake.AddDate(0, 0, 1)
	}
	return wake.UTC()
}

func withinQuietHours(minute, start, end int) bool {
	if start < end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}

func IsJourneyEvent(eventType string) bool {
	_, ok := journeyEvents[strings.ToLower(strings.TrimSpace(eventType))]
	return ok
}

func JourneyEventTypes() []string {
	events := make([]string, 0, len(journeyEvents))
	for eventType := range journeyEvents {
		events = append(events, eventType)
	}
	sort.Strings(events)
	return events
}

func ExtractJourneyContext(data map[string]any) map[string]string {
	result := make(map[string]string)
	for field := range journeyContextFields {
		value, exists := data[field]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" && len([]rune(trimmed)) <= 160 {
				result[field] = trimmed
			}
		case float64:
			result[field] = strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			result[field] = strconv.FormatBool(typed)
		}
	}
	return result
}

func RenderJourneyTemplate(value string, context map[string]string) (string, error) {
	var renderErr error
	rendered := journeyTemplateVariable.ReplaceAllStringFunc(value, func(match string) string {
		parts := journeyTemplateVariable.FindStringSubmatch(match)
		if len(parts) != 2 {
			renderErr = fmt.Errorf("%w: invalid journey template variable", ErrValidation)
			return ""
		}
		replacement, ok := context[parts[1]]
		if !ok {
			renderErr = fmt.Errorf("%w: journey template variable %s is unavailable", ErrValidation, parts[1])
			return ""
		}
		return replacement
	})
	if renderErr != nil {
		return "", renderErr
	}
	if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
		return "", fmt.Errorf("%w: malformed journey template variable", ErrValidation)
	}
	return rendered, nil
}

func ValidateJourneyTemplateVariables(value string) error {
	for _, parts := range journeyTemplateVariable.FindAllStringSubmatch(value, -1) {
		if len(parts) != 2 {
			return fmt.Errorf("%w: malformed journey template variable", ErrValidation)
		}
		if _, ok := journeyTemplateFields[parts[1]]; !ok {
			return fmt.Errorf("%w: unsupported journey template variable %s", ErrValidation, parts[1])
		}
	}
	withoutVariables := journeyTemplateVariable.ReplaceAllString(value, "")
	if strings.Contains(withoutVariables, "{{") || strings.Contains(withoutVariables, "}}") {
		return fmt.Errorf("%w: malformed journey template variable", ErrValidation)
	}
	return nil
}

func isJourneyChannel(channel NotificationChannel) bool {
	switch channel {
	case ChannelEmail, ChannelSMS, ChannelWhatsApp, ChannelInApp:
		return true
	default:
		return false
	}
}

func validJourneyStatus(status JourneyStatus) bool {
	switch status {
	case JourneyStatusDraft, JourneyStatusActive, JourneyStatusPaused, JourneyStatusArchived:
		return true
	default:
		return false
	}
}

func normalizeJourneyEvents(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
