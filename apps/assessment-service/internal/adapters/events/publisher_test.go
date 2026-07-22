package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
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

const (
	testTenant  = "11111111-1111-1111-1111-111111111111"
	testYear    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	testSubject = "55555555-5555-5555-5555-555555555555"
	testClass1  = "77777777-7777-4777-8777-777777777771"
	testClass2  = "88888888-8888-4888-8888-888888888882"
)

// readContract parses the source-of-truth assignment.published contract.
// The os.ReadFile call site uses a literal path so gosec G304 stays quiet.
func assignmentPublishedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/assignment.published.v1.json")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return schema
}

func assessmentScoreRecordedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/assessment.score_recorded.v1.json")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return schema
}

func assessmentScoreUpdatedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/assessment.score_updated.v1.json")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return schema
}

func assessmentScoreDeletedContract(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/events/assessment.score_deleted.v1.json")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return schema
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

// assertContractConformance validates the emitted event against the parts of
// the contract the bus can guarantee: required envelope/data keys, const
// values, and declared uuid/date-time formats.
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
			if prop["format"] == "uuid" {
				if _, err := uuid.Parse(s); err != nil {
					t.Errorf("data[%q]: invalid uuid %q", key, s)
				}
			}
			if prop["format"] == "date-time" {
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
			itemSchema, ok := prop["items"].(map[string]any)
			if !ok {
				t.Errorf("data[%q]: missing array item schema", key)
				continue
			}
			for i, item := range items {
				s, ok := item.(string)
				if !ok {
					t.Errorf("data[%q][%d]: expected string item, got %T", key, i, item)
					continue
				}
				if itemSchema["format"] == "uuid" {
					if _, err := uuid.Parse(s); err != nil {
						t.Errorf("data[%q][%d]: invalid uuid %q", key, i, s)
					}
				}
			}
		}
	}
}

func newTestAssignment(t *testing.T, due *time.Time) *domain.Assessment {
	t.Helper()
	a, err := domain.NewAssignment(testTenant, testYear, testSubject, "Essay 1", "Write 500 words", 50, due, []string{testClass1, testClass2})
	if err != nil {
		t.Fatalf("new assignment: %v", err)
	}
	if err := a.Publish(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	return a
}

func publishOne(t *testing.T, pub *Publisher, js *fakeJS, eventType string, a *domain.Assessment) map[string]any {
	t.Helper()
	if err := pub.PublishAssignment(context.Background(), eventType, a, nil); err != nil {
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

func TestPublisher_AssignmentPublished_ConformsToContract(t *testing.T) {
	js := &fakeJS{}
	pub := NewPublisher(eventbus.NewPublisher(js))
	due := time.Date(2025, 11, 1, 23, 59, 0, 0, time.UTC)
	assignment := newTestAssignment(t, &due)

	event := publishOne(t, pub, js, "assignment.published.v1", assignment)

	// Envelope.
	if got := event["specversion"]; got != "1.0" {
		t.Errorf("specversion: expected 1.0, got %v", got)
	}
	if got := event["type"]; got != "assignment.published.v1" {
		t.Errorf("type: expected assignment.published.v1, got %v", got)
	}
	if got := event["source"]; got != "assessment-service" {
		t.Errorf("source: expected assessment-service, got %v", got)
	}
	if got := event["tenant_id"]; got != testTenant {
		t.Errorf("tenant_id: expected %q, got %v", testTenant, got)
	}
	if got := event["subject"]; got != assignment.ID {
		t.Errorf("subject: expected %q, got %v", assignment.ID, got)
	}
	if _, err := uuid.Parse(asString(t, event, "id")); err != nil {
		t.Errorf("id: expected a uuid event id, got %v", event["id"])
	}
	if _, err := time.Parse(time.RFC3339, asString(t, event, "time")); err != nil {
		t.Errorf("time: expected RFC3339, got %v", event["time"])
	}

	assertContractConformance(t, assignmentPublishedContract(t), event)

	// Payload values.
	data := objectAt(t, event, "data")
	if got := data["assignment_id"]; got != assignment.ID {
		t.Errorf("data.assignment_id: expected %q, got %v", assignment.ID, got)
	}
	if got := data["subject_id"]; got != testSubject {
		t.Errorf("data.subject_id: expected %q, got %v", testSubject, got)
	}
	ids, ok := data["class_ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != testClass1 || ids[1] != testClass2 {
		t.Errorf("data.class_ids: expected [%s %s], got %v", testClass1, testClass2, data["class_ids"])
	}
	if got := data["due_date"]; got != due.Format(time.RFC3339) {
		t.Errorf("data.due_date: expected %q, got %v", due.Format(time.RFC3339), got)
	}
}

func TestPublisher_AssignmentPublished_OmitsEmptyOptionalFields(t *testing.T) {
	js := &fakeJS{}
	pub := NewPublisher(eventbus.NewPublisher(js))
	assignment := newTestAssignment(t, nil)
	assignment.ClassIDs = nil

	event := publishOne(t, pub, js, "assignment.published.v1", assignment)
	data := objectAt(t, event, "data")
	if _, ok := data["class_ids"]; ok {
		t.Errorf("data.class_ids should be omitted when empty, got %v", data["class_ids"])
	}
	if _, ok := data["due_date"]; ok {
		t.Errorf("data.due_date should be omitted when unset, got %v", data["due_date"])
	}
}

func TestPublisher_AssessmentScoreRecorded_ConformsToContract(t *testing.T) {
	js := &fakeJS{}
	pub := NewPublisher(eventbus.NewPublisher(js))
	studentID := "99999999-9999-4999-8999-999999999999"
	assessmentID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	score, err := domain.NewScore(testTenant, assessmentID, studentID, 72, "teacher-1", "", 100)
	if err != nil {
		t.Fatalf("new score: %v", err)
	}
	meta := map[string]any{
		"assessment_id":    assessmentID,
		"subject_id":       testSubject,
		"academic_year_id": testYear,
		"max_score":        100,
		"recorded_at":      score.CreatedAt.UTC().Format(time.RFC3339),
		"class_ids":        []string{testClass1, testClass2},
	}
	if err := pub.PublishScore(context.Background(), "assessment.score_recorded.v1", score, meta); err != nil {
		t.Fatalf("publish score: %v", err)
	}
	if len(js.published) != 1 {
		t.Fatalf("expected one published event, got %d", len(js.published))
	}
	var event map[string]any
	if err := json.Unmarshal(js.published[0].Data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	assertContractConformance(t, assessmentScoreRecordedContract(t), event)
	if got := event["type"]; got != "assessment.score_recorded.v1" {
		t.Fatalf("event type = %v", got)
	}
	if got := event["subject"]; got != score.ID {
		t.Fatalf("event subject = %v, want %s", got, score.ID)
	}
	data := objectAt(t, event, "data")
	for key, want := range map[string]any{
		"score_id":         score.ID,
		"assessment_id":    assessmentID,
		"student_id":       studentID,
		"subject_id":       testSubject,
		"academic_year_id": testYear,
		"score":            float64(72),
		"max_score":        float64(100),
	} {
		if got := data[key]; got != want {
			t.Errorf("data[%s] = %v, want %v", key, got, want)
		}
	}
}

func TestPublisher_AssessmentScoreLifecycleEvents_ConformToContracts(t *testing.T) {
	studentID := "99999999-9999-4999-8999-999999999999"
	assessmentID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	score, err := domain.NewScore(testTenant, assessmentID, studentID, 72, "teacher-1", "", 100)
	if err != nil {
		t.Fatalf("new score: %v", err)
	}
	baseMeta := map[string]any{
		"assessment_id":    assessmentID,
		"subject_id":       testSubject,
		"academic_year_id": testYear,
		"max_score":        100,
		"recorded_at":      score.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":       score.UpdatedAt.UTC().Format(time.RFC3339),
		"class_ids":        []string{testClass1, testClass2},
	}

	tests := []struct {
		name      string
		eventType string
		contract  func(*testing.T) map[string]any
		extraMeta map[string]any
		required  string
	}{
		{
			name:      "updated",
			eventType: "assessment.score_updated.v1",
			contract:  assessmentScoreUpdatedContract,
			extraMeta: map[string]any{"changed_fields": []string{"score"}},
			required:  "changed_fields",
		},
		{
			name:      "deleted",
			eventType: "assessment.score_deleted.v1",
			contract:  assessmentScoreDeletedContract,
			extraMeta: map[string]any{"deleted_at": time.Now().UTC().Format(time.RFC3339)},
			required:  "deleted_at",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			js := &fakeJS{}
			pub := NewPublisher(eventbus.NewPublisher(js))
			meta := make(map[string]any, len(baseMeta)+len(tc.extraMeta))
			for key, value := range baseMeta {
				meta[key] = value
			}
			for key, value := range tc.extraMeta {
				meta[key] = value
			}
			if err := pub.PublishScore(context.Background(), tc.eventType, score, meta); err != nil {
				t.Fatalf("publish score: %v", err)
			}
			if len(js.published) != 1 {
				t.Fatalf("expected one published event, got %d", len(js.published))
			}
			var event map[string]any
			if err := json.Unmarshal(js.published[0].Data, &event); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}
			assertContractConformance(t, tc.contract(t), event)
			if got := event["type"]; got != tc.eventType {
				t.Errorf("event type = %v, want %s", got, tc.eventType)
			}
			data := objectAt(t, event, "data")
			if _, ok := data[tc.required]; !ok {
				t.Errorf("data missing lifecycle field %q", tc.required)
			}
		})
	}
}

func TestPublisher_NilBusIsNoop(t *testing.T) {
	var pub *Publisher
	if err := pub.PublishAssignment(context.Background(), "assignment.published.v1", newTestAssignment(t, nil), nil); err != nil {
		t.Fatalf("nil publisher should be a no-op, got %v", err)
	}
}
