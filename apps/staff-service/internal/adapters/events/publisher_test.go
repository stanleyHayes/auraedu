package events

import (
	"context"
	"testing"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ messages []*nats.Msg }

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.messages = append(c.messages, message)
	return &nats.PubAck{Stream: "AURA_EVENTS"}, nil
}
func (*captureJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nats.ErrStreamNotFound
}
func (*captureJetStream) AddStream(config *nats.StreamConfig, _ ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: *config}, nil
}
func (*captureJetStream) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return nil, nil
}

func TestStaffPublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	staff, err := domain.NewStaff(tenantID, "Ama", "Mensah", string(domain.StaffTypeTeacher))
	if err != nil {
		t.Fatalf("new staff: %v", err)
	}
	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))

	for _, testCase := range []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "staff.created.v1"},
		{eventType: "staff.updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "staff.deleted.v1"},
	} {
		if err := publisher.Publish(context.Background(), testCase.eventType, staff, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertStaffContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, tenantID, staff.ID, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, tenantID, ports.StaffEventData(staff, testCase.meta)); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertStaffContract(t, capture.messages[len(capture.messages)-1], testCase.eventType, tenantID, staff.ID, eventID)
	}

	assignment, err := domain.NewAssignment(tenantID, staff.ID, "44444444-4444-4444-8444-444444444444", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	eventID := uuid.NewString()
	if err := publisher.PublishWithID(context.Background(), eventID, "staff.assigned.v1", tenantID, ports.AssignmentEventData(assignment)); err != nil {
		t.Fatalf("publish outbox staff.assigned.v1: %v", err)
	}
	assertStaffContract(t, capture.messages[len(capture.messages)-1], "staff.assigned.v1", tenantID, staff.ID, eventID)
}

func assertStaffContract(t *testing.T, message *nats.Msg, eventType, tenantID, subject, eventID string) {
	t.Helper()
	event := testkit.AssertEventContract(t, eventType, message.Data)
	if event["source"] != "staff-service" || event["tenant_id"] != tenantID || event["subject"] != subject {
		t.Fatalf("unexpected envelope identity: %+v", event)
	}
	if _, serialized := event["idempotency_key"]; serialized {
		t.Fatalf("transport deduplication key leaked into envelope: %+v", event)
	}
	if eventID != "" && (event["id"] != eventID || message.Header.Get("Nats-Msg-Id") != eventID) {
		t.Fatalf("durable event identity is not stable: event=%+v header=%q", event, message.Header.Get("Nats-Msg-Id"))
	}
}
