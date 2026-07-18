package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/platform/eventbus"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// fakeJS captures published messages the way platform/eventbus tests do.
type fakeJS struct {
	published []*nats.Msg
}

func (f *fakeJS) PublishMsg(msg *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	f.published = append(f.published, msg)
	return &nats.PubAck{Stream: "AURA"}, nil
}

func (f *fakeJS) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}

func (f *fakeJS) AddStream(cfg *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *cfg}, nil
}

func (f *fakeJS) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return nil, nil
}

const testTenant = "11111111-1111-1111-1111-111111111111"

func newTestPublisher() (*Publisher, *fakeJS) {
	js := &fakeJS{}
	return NewPublisher(eventbus.NewPublisher(js)), js
}

// readContractSchema parses a source-of-truth contract from contracts/events/. The
// os.ReadFile call sites below use literal paths so gosec G304 stays quiet.
func readContractSchema(t *testing.T, raw []byte, err error) map[string]any {
	t.Helper()
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return schema
}

func yearCreatedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/academic.year_created.v1.json")
	return readContractSchema(t, raw, err)
}

func classCreatedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/academic.class_created.v1.json")
	return readContractSchema(t, raw, err)
}

func subjectCreatedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/academic.subject_created.v1.json")
	return readContractSchema(t, raw, err)
}

func objectAt(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("expected object at key %q", key)
	}
	return v
}

func stringSlice(t *testing.T, v any) []string {
	t.Helper()
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", v)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected string item, got %T", item)
		}
		out = append(out, s)
	}
	return out
}

func asString(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("expected string at key %q, got %v", key, m[key])
	}
	return s
}

// assertContractConformance validates the emitted event against the parts of the
// contract the bus can guarantee: required envelope/data keys, const values, and
// declared uuid/date/date-time formats. The "type" const is checked separately:
// contracts name the base type ("academic.class_created") while the bus convention —
// followed by every publisher and subscriber in this repo — versions it
// ("academic.class_created.v1"), so the subject and type carry the .v1 suffix.
func assertContractConformance(t *testing.T, schema map[string]any, event map[string]any) {
	t.Helper()
	for _, key := range stringSlice(t, schema["required"]) {
		if _, ok := event[key]; !ok {
			t.Errorf("envelope missing required key %q", key)
		}
	}
	props := objectAt(t, schema, "properties")
	for key, raw := range props {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		got, present := event[key]
		if c, ok := prop["const"].(string); ok && present && key != "type" {
			if got != c {
				t.Errorf("envelope key %q: expected const %q, got %v", key, c, got)
			}
		}
	}

	data := objectAt(t, event, "data")
	dataSchema := objectAt(t, props, "data")
	for _, key := range stringSlice(t, dataSchema["required"]) {
		if _, ok := data[key]; !ok {
			t.Errorf("data missing required key %q", key)
		}
	}
	dataProps := objectAt(t, dataSchema, "properties")
	for key, raw := range dataProps {
		value, present := data[key]
		if !present {
			continue // optional field not populated
		}
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if prop["type"] != "string" {
			continue // nullable and non-string types are asserted by the specific tests
		}
		s, ok := value.(string)
		if !ok {
			t.Errorf("data[%q]: expected string, got %T", key, value)
			continue
		}
		switch prop["format"] {
		case "uuid":
			if _, err := uuid.Parse(s); err != nil {
				t.Errorf("data[%q]: invalid uuid %q", key, s)
			}
		case "date":
			if _, err := time.Parse(time.DateOnly, s); err != nil {
				t.Errorf("data[%q]: invalid date %q", key, s)
			}
		case "date-time":
			if _, err := time.Parse(time.RFC3339, s); err != nil {
				t.Errorf("data[%q]: invalid date-time %q", key, s)
			}
		}
	}
}

func assertEnvelope(t *testing.T, event map[string]any, eventType, subjectID string) {
	t.Helper()
	if got := event["specversion"]; got != "1.0" {
		t.Errorf("specversion: expected 1.0, got %v", got)
	}
	if got := event["type"]; got != eventType {
		t.Errorf("type: expected %q, got %v", eventType, got)
	}
	if got := event["source"]; got != "academic-service" {
		t.Errorf("source: expected academic-service, got %v", got)
	}
	if got := event["tenant_id"]; got != testTenant {
		t.Errorf("tenant_id: expected %q, got %v", testTenant, got)
	}
	if got := event["subject"]; got != subjectID {
		t.Errorf("subject: expected %q, got %v", subjectID, got)
	}
	if _, err := uuid.Parse(asString(t, event, "id")); err != nil {
		t.Errorf("id: expected a uuid event id, got %v", event["id"])
	}
	if _, err := time.Parse(time.RFC3339, asString(t, event, "time")); err != nil {
		t.Errorf("time: expected RFC3339, got %v", event["time"])
	}
}

func oneMessage(t *testing.T, js *fakeJS, eventType string) map[string]any {
	t.Helper()
	if len(js.published) != 1 {
		t.Fatalf("expected 1 message on the bus, got %d", len(js.published))
	}
	msg := js.published[0]
	if want := "AURA." + eventType; msg.Subject != want {
		t.Fatalf("subject: expected %q, got %q", want, msg.Subject)
	}
	var event map[string]any
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		t.Fatalf("unmarshal published event: %v", err)
	}
	return event
}

func TestPublisher_YearCreated_ConformsToContract(t *testing.T) {
	pub, js := newTestPublisher()
	year, err := domain.NewAcademicYear(testTenant, "2025/26", "", "2025-09-01", "2026-07-31", true)
	if err != nil {
		t.Fatalf("new academic year: %v", err)
	}

	if err := pub.PublishYear(context.Background(), "academic.year_created.v1", year, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	event := oneMessage(t, js, "academic.year_created.v1")
	assertEnvelope(t, event, "academic.year_created.v1", year.ID)
	assertContractConformance(t, yearCreatedContract(t), event)

	data := objectAt(t, event, "data")
	if got := data["year_id"]; got != year.ID {
		t.Errorf("data.year_id: expected %q, got %v", year.ID, got)
	}
	if got := data["name"]; got != year.Name {
		t.Errorf("data.name: expected %q, got %v", year.Name, got)
	}
}

func TestPublisher_ClassCreated_ConformsToContract(t *testing.T) {
	pub, js := newTestPublisher()
	yearID := uuid.NewString()
	class, err := domain.NewClass(testTenant, yearID, "Form 2", nil, nil)
	if err != nil {
		t.Fatalf("new class: %v", err)
	}

	if err := pub.PublishClass(context.Background(), "academic.class_created.v1", class, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	event := oneMessage(t, js, "academic.class_created.v1")
	assertEnvelope(t, event, "academic.class_created.v1", class.ID)
	assertContractConformance(t, classCreatedContract(t), event)

	data := objectAt(t, event, "data")
	if got := data["class_id"]; got != class.ID {
		t.Errorf("data.class_id: expected %q, got %v", class.ID, got)
	}
	if got := data["name"]; got != class.Name {
		t.Errorf("data.name: expected %q, got %v", class.Name, got)
	}
	if got := data["academic_year_id"]; got != yearID {
		t.Errorf("data.academic_year_id: expected %q, got %v", yearID, got)
	}
}

func TestPublisher_SubjectCreated_ConformsToContract(t *testing.T) {
	pub, js := newTestPublisher()
	code := "MATH"
	subject, err := domain.NewSubject(testTenant, "Mathematics", &code, nil)
	if err != nil {
		t.Fatalf("new subject: %v", err)
	}

	if err := pub.PublishSubject(context.Background(), "academic.subject_created.v1", subject, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	event := oneMessage(t, js, "academic.subject_created.v1")
	assertEnvelope(t, event, "academic.subject_created.v1", subject.ID)
	assertContractConformance(t, subjectCreatedContract(t), event)

	data := objectAt(t, event, "data")
	if got := data["subject_id"]; got != subject.ID {
		t.Errorf("data.subject_id: expected %q, got %v", subject.ID, got)
	}
	if got := data["name"]; got != subject.Name {
		t.Errorf("data.name: expected %q, got %v", subject.Name, got)
	}
	if got := data["code"]; got != code {
		t.Errorf("data.code: expected %q, got %v", code, got)
	}
}

// TestPublisher_SubjectCreated_NullCode pins the contract's nullable code: the key is
// present with a JSON null when the subject has no code (type ["string","null"]).
func TestPublisher_SubjectCreated_NullCode(t *testing.T) {
	pub, js := newTestPublisher()
	subject, err := domain.NewSubject(testTenant, "Mathematics", nil, nil)
	if err != nil {
		t.Fatalf("new subject: %v", err)
	}

	if err := pub.PublishSubject(context.Background(), "academic.subject_created.v1", subject, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	event := oneMessage(t, js, "academic.subject_created.v1")
	data := objectAt(t, event, "data")
	v, present := data["code"]
	if !present {
		t.Fatal("data.code must be present (contract declares it nullable, not optional)")
	}
	if v != nil {
		t.Errorf("data.code: expected null when unset, got %v", v)
	}
}

func TestPublisher_ClassUpdated_CarriesChangedFields(t *testing.T) {
	pub, js := newTestPublisher()
	class, err := domain.NewClass(testTenant, uuid.NewString(), "Form 2", nil, nil)
	if err != nil {
		t.Fatalf("new class: %v", err)
	}

	if err := pub.PublishClass(context.Background(), "academic.class_updated.v1", class, map[string]any{
		"changed_fields": []string{"name"},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	event := oneMessage(t, js, "academic.class_updated.v1")
	assertEnvelope(t, event, "academic.class_updated.v1", class.ID)

	data := objectAt(t, event, "data")
	fields, ok := data["changed_fields"].([]any)
	if !ok || len(fields) != 1 || fields[0] != "name" {
		t.Errorf("data.changed_fields: expected [name], got %v", data["changed_fields"])
	}
}

func TestPublisher_NilBusIsNoop(t *testing.T) {
	var pub *Publisher
	year, err := domain.NewAcademicYear(testTenant, "2025/26", "", "2025-09-01", "2026-07-31", false)
	if err != nil {
		t.Fatalf("new academic year: %v", err)
	}
	if err := pub.PublishYear(context.Background(), "academic.year_created.v1", year, nil); err != nil {
		t.Fatalf("nil publisher should be a no-op, got %v", err)
	}
}
