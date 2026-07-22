package events

import (
	"context"
	"testing"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct {
	messages []*nats.Msg
}

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

func TestAttendancePublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	record, err := domain.NewAttendanceRecord(
		tenantID,
		uuid.NewString(),
		uuid.NewString(),
		"2026-07-20",
		string(domain.StatusPresent),
		uuid.NewString(),
		nil,
	)
	if err != nil {
		t.Fatalf("new attendance record: %v", err)
	}

	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	assertLastContract := func(eventType string, eventID string) {
		t.Helper()
		if len(capture.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		event := testkit.AssertEventContract(t, eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "attendance-service" || event["tenant_id"] != tenantID || event["subject"] != record.ID {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		if eventID != "" && event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
		if eventID != "" && capture.messages[len(capture.messages)-1].Header.Get("Nats-Msg-Id") != eventID {
			t.Fatalf("broker deduplication key does not match outbox identity")
		}
	}

	for _, testCase := range []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "attendance.marked.v1"},
		{eventType: "attendance.updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "attendance.deleted.v1"},
	} {
		if err := publisher.Publish(context.Background(), testCase.eventType, record, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, "")
	}

	outboxID := uuid.NewString()
	if err := publisher.PublishWithID(
		context.Background(),
		outboxID,
		"attendance.marked.v1",
		tenantID,
		ports.AttendanceEventData(record, nil),
	); err != nil {
		t.Fatalf("publish outbox attendance: %v", err)
	}
	assertLastContract("attendance.marked.v1", outboxID)
}
