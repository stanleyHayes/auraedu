package events

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
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

func TestCBTPublishersConformToAuthoritativeContracts(t *testing.T) {
	const tenantID = "readiness-academy"
	academicYearID := uuid.NewString()
	subjectID := uuid.NewString()
	question, err := domain.NewQuestionBank(
		tenantID,
		academicYearID,
		subjectID,
		"Which number is prime?",
		string(domain.TypeMultipleChoice),
		"2",
		1,
		[]string{"2", "4", "6"},
	)
	if err != nil {
		t.Fatalf("new question: %v", err)
	}
	startAt := time.Date(2026, time.July, 21, 9, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	exam, err := domain.NewExamSession(
		tenantID,
		"Mathematics readiness",
		academicYearID,
		subjectID,
		[]string{question.ID},
		60,
		&startAt,
		&endAt,
	)
	if err != nil {
		t.Fatalf("new exam: %v", err)
	}
	submission, err := domain.NewSubmission(tenantID, exam.ID, uuid.NewString())
	if err != nil {
		t.Fatalf("new submission: %v", err)
	}
	if err := submission.Submit(map[string]string{question.ID: "2"}); err != nil {
		t.Fatalf("submit exam: %v", err)
	}
	if err := submission.Grade(1, 1); err != nil {
		t.Fatalf("grade exam: %v", err)
	}

	capture := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(capture))
	assertLastContract := func(eventType, subject, eventID string) {
		t.Helper()
		if len(capture.messages) == 0 {
			t.Fatal("publisher did not call JetStream")
		}
		event := testkit.AssertEventContract(t, eventType, capture.messages[len(capture.messages)-1].Data)
		if event["source"] != "cbt-service" || event["tenant_id"] != tenantID || event["subject"] != subject {
			t.Fatalf("unexpected envelope identity: %+v", event)
		}
		if eventID != "" && event["id"] != eventID {
			t.Fatalf("outbox identity is not stable: %+v", event)
		}
		if eventID != "" && capture.messages[len(capture.messages)-1].Header.Get("Nats-Msg-Id") != eventID {
			t.Fatalf("broker deduplication key does not match outbox identity")
		}
	}

	questionEvents := []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "cbt.question_created.v1"},
		{eventType: "cbt.question_updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "cbt.question_deleted.v1"},
	}
	for _, testCase := range questionEvents {
		if err := publisher.PublishQuestionBank(context.Background(), testCase.eventType, question, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, question.ID, "")
	}

	examEvents := []struct {
		eventType string
		meta      map[string]any
	}{
		{eventType: "cbt.exam_created.v1"},
		{eventType: "cbt.exam_updated.v1", meta: map[string]any{"changed_fields": []string{"status"}}},
		{eventType: "cbt.exam_deleted.v1"},
	}
	for _, testCase := range examEvents {
		if err := publisher.PublishExamSession(context.Background(), testCase.eventType, exam, testCase.meta); err != nil {
			t.Fatalf("publish %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, exam.ID, "")
	}

	for _, eventType := range []string{"cbt.exam_submitted.v1", "cbt.graded.v1"} {
		if err := publisher.PublishSubmission(context.Background(), eventType, submission, nil); err != nil {
			t.Fatalf("publish %s: %v", eventType, err)
		}
		assertLastContract(eventType, submission.ID, "")
	}

	outboxCases := []struct {
		eventType string
		subject   string
		data      map[string]any
	}{
		{eventType: "cbt.question_created.v1", subject: question.ID, data: ports.QuestionEventData(question, nil)},
		{eventType: "cbt.exam_created.v1", subject: exam.ID, data: ports.ExamEventData(exam, nil)},
		{eventType: "cbt.exam_submitted.v1", subject: submission.ID, data: ports.SubmissionEventData("cbt.exam_submitted.v1", submission)},
		{eventType: "cbt.graded.v1", subject: submission.ID, data: ports.SubmissionEventData("cbt.graded.v1", submission)},
	}
	for _, testCase := range outboxCases {
		eventID := uuid.NewString()
		if err := publisher.PublishWithID(context.Background(), eventID, testCase.eventType, tenantID, testCase.data); err != nil {
			t.Fatalf("publish outbox %s: %v", testCase.eventType, err)
		}
		assertLastContract(testCase.eventType, testCase.subject, eventID)
	}
}
