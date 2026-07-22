package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/admissions-service/internal/ports"
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

func TestAdmissionsPublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	startedAt := time.Date(2026, time.July, 20, 9, 0, 0, 0, time.UTC)
	leadID := uuid.NewString()
	application, err := domain.New(
		tenantID,
		uuid.NewString(),
		&leadID,
		uuid.NewString(),
		uuid.NewString(),
		"Bachelor of Science",
		"September 2026",
		startedAt,
	)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	payloads := map[string]map[string]any{
		"application.started.v1": ports.ApplicationEventData("application.started.v1", application, startedAt),
	}
	application.LegalName = "Ama Mensah"
	application.Email = "ama@example.edu"
	application.Phone = "+233200000000"
	if err := application.AttachDocument(uuid.NewString(), "transcript", "transcript.pdf", startedAt.Add(time.Minute)); err != nil {
		t.Fatalf("attach document: %v", err)
	}
	submittedAt := startedAt.Add(time.Hour)
	if err := application.Submit(submittedAt); err != nil {
		t.Fatalf("submit application: %v", err)
	}
	payloads["application.submitted.v1"] = ports.ApplicationEventData("application.submitted.v1", application, submittedAt)

	reviewedAt := submittedAt.Add(time.Hour)
	if err := application.Review("admitted", uuid.NewString(), "Meets admissions requirements", reviewedAt); err != nil {
		t.Fatalf("admit application: %v", err)
	}
	payloads["application.admitted.v1"] = ports.ApplicationEventData("application.admitted.v1", application, reviewedAt)

	issuedAt := reviewedAt.Add(time.Hour)
	expiresAt := issuedAt.Add(14 * 24 * time.Hour)
	if err := application.IssueOffer(uuid.NewString(), "Submit original transcript", expiresAt, issuedAt); err != nil {
		t.Fatalf("issue offer: %v", err)
	}
	payloads["offer.issued.v1"] = ports.ApplicationEventData("offer.issued.v1", application, issuedAt)

	acceptedAt := issuedAt.Add(time.Hour)
	if err := application.AcceptOffer(application.ApplicantUserID, acceptedAt); err != nil {
		t.Fatalf("accept offer: %v", err)
	}
	payloads["offer.accepted.v1"] = ports.ApplicationEventData("offer.accepted.v1", application, acceptedAt)

	capture := &captureJetStream{}
	publisher := New(eventbus.NewPublisher(capture))
	assertLastContract := func(eventType, eventID string) {
		t.Helper()
		if len(capture.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		event := testkit.AssertEventContract(t, eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "admissions-service" || event["tenant_id"] != tenantID {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		if _, serialized := event["idempotency_key"]; serialized {
			t.Fatalf("transport deduplication key leaked into contracted envelope: %+v", event)
		}
		if capture.messages[len(capture.messages)-1].Header.Get("Nats-Msg-Id") != event["id"] {
			t.Fatalf("broker deduplication key does not match event identity: %+v", event)
		}
		if eventID != "" && event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
	}

	for _, eventType := range []string{
		"application.started.v1",
		"application.submitted.v1",
		"application.admitted.v1",
		"offer.issued.v1",
		"offer.accepted.v1",
	} {
		payload := payloads[eventType]
		if err := publisher.Publish(context.Background(), eventType, tenantID, payload); err != nil {
			t.Fatalf("publish %s: %v", eventType, err)
		}
		assertLastContract(eventType, "")

		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, eventType, tenantID, payload); err != nil {
			t.Fatalf("publish outbox %s: %v", eventType, err)
		}
		assertLastContract(eventType, eventID)
	}
}
