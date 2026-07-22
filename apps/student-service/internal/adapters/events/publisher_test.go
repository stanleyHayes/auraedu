package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/student-service/internal/domain"
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

func newTestPublisher(t *testing.T) (*Publisher, *fakeJS) {
	t.Helper()
	js := &fakeJS{}
	return NewPublisher(eventbus.NewPublisher(js)), js
}

func newTestStudent(t *testing.T) *domain.Student {
	t.Helper()
	s, err := domain.NewStudent(testTenant, "Kwame", "Nkrumah")
	if err != nil {
		t.Fatalf("new student: %v", err)
	}
	return s
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

func enrolledContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/student.enrolled.v1.json")
	return readContractSchema(t, raw, err)
}

func updatedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/student.updated.v1.json")
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
// declared uuid/date/date-time formats.
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
		if c, ok := prop["const"].(string); ok && present {
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
		switch prop["type"] {
		case "string":
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
		case "array":
			items, ok := value.([]any)
			if !ok {
				t.Errorf("data[%q]: expected array, got %T", key, value)
				continue
			}
			for i, item := range items {
				if _, ok := item.(string); !ok {
					t.Errorf("data[%q][%d]: expected string item, got %T", key, i, item)
				}
			}
		}
	}
}

func publishOne(t *testing.T, pub *Publisher, js *fakeJS, eventType string, student *domain.Student, meta map[string]any) map[string]any {
	t.Helper()
	if err := pub.Publish(context.Background(), eventType, student, meta); err != nil {
		t.Fatalf("publish %s: %v", eventType, err)
	}
	if len(js.published) != 1 {
		t.Fatalf("expected 1 message on the bus, got %d", len(js.published))
	}
	msg := js.published[0]
	if want := "AURA." + eventType; msg.Subject != want {
		t.Fatalf("subject: expected %q, got %q", want, msg.Subject)
	}
	return testkit.AssertEventContract(t, eventType, msg.Data)
}

func assertEnvelope(t *testing.T, event map[string]any, eventType string, student *domain.Student) {
	t.Helper()
	if got := event["specversion"]; got != "1.0" {
		t.Errorf("specversion: expected 1.0, got %v", got)
	}
	if got := event["type"]; got != eventType {
		t.Errorf("type: expected %q, got %v", eventType, got)
	}
	if got := event["source"]; got != "student-service" {
		t.Errorf("source: expected student-service, got %v", got)
	}
	if got := event["tenant_id"]; got != testTenant {
		t.Errorf("tenant_id: expected %q, got %v", testTenant, got)
	}
	if got := event["subject"]; got != student.ID {
		t.Errorf("subject: expected %q, got %v", student.ID, got)
	}
	if _, err := uuid.Parse(asString(t, event, "id")); err != nil {
		t.Errorf("id: expected a uuid event id, got %v", event["id"])
	}
	if _, err := time.Parse(time.RFC3339, asString(t, event, "time")); err != nil {
		t.Errorf("time: expected RFC3339, got %v", event["time"])
	}
}

func TestPublisher_StudentEnrolled_ConformsToContract(t *testing.T) {
	pub, js := newTestPublisher(t)
	student := newTestStudent(t)
	classID := uuid.NewString()
	academicYearID := uuid.NewString()

	event := publishOne(t, pub, js, "student.enrolled.v1", student, map[string]any{
		"enrollment_date":  student.CreatedAt.UTC().Format(time.DateOnly),
		"class_id":         classID,
		"academic_year_id": academicYearID,
	})

	assertEnvelope(t, event, "student.enrolled.v1", student)
	assertContractConformance(t, enrolledContract(t), event)

	data := objectAt(t, event, "data")
	if got := data["student_id"]; got != student.ID {
		t.Errorf("data.student_id: expected %q, got %v", student.ID, got)
	}
	if got := data["class_id"]; got != classID {
		t.Errorf("data.class_id: expected %q, got %v", classID, got)
	}
	if got := data["academic_year_id"]; got != academicYearID {
		t.Errorf("data.academic_year_id: expected %q, got %v", academicYearID, got)
	}
}

func TestPublisher_StudentCreatedWithoutEnrollmentConformsToContract(t *testing.T) {
	pub, js := newTestPublisher(t)
	student := newTestStudent(t)

	event := publishOne(t, pub, js, "student.created.v1", student, nil)

	assertEnvelope(t, event, "student.created.v1", student)
	data := objectAt(t, event, "data")
	if got := data["student_id"]; got != student.ID {
		t.Errorf("data.student_id: expected %q, got %v", student.ID, got)
	}
}

func TestPublisher_StudentUpdated_ConformsToContract(t *testing.T) {
	pub, js := newTestPublisher(t)
	student := newTestStudent(t)

	event := publishOne(t, pub, js, "student.updated.v1", student, map[string]any{
		"changed_fields": []string{"first_name"},
	})

	assertEnvelope(t, event, "student.updated.v1", student)
	assertContractConformance(t, updatedContract(t), event)

	data := objectAt(t, event, "data")
	if got := data["student_id"]; got != student.ID {
		t.Errorf("data.student_id: expected %q, got %v", student.ID, got)
	}
	fields, ok := data["changed_fields"].([]any)
	if !ok || len(fields) != 1 || fields[0] != "first_name" {
		t.Errorf("data.changed_fields: expected [first_name], got %v", data["changed_fields"])
	}
}

func TestPublisher_NilBusIsNoop(t *testing.T) {
	var pub *Publisher
	if err := pub.Publish(context.Background(), "student.enrolled.v1", newTestStudent(t), nil); err != nil {
		t.Fatalf("nil publisher should be a no-op, got %v", err)
	}
}
