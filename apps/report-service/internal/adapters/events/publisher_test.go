package events

import (
	"context"
	"testing"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/testkit"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type captureJetStream struct{ messages []*nats.Msg }

func (c *captureJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.messages = append(c.messages, message)
	return &nats.PubAck{Stream: "AURA"}, nil
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

func TestTemplateCreatedAndReportPublishedConformToContracts(t *testing.T) {
	js := &captureJetStream{}
	publisher := NewPublisher(eventbus.NewPublisher(js))
	tenantID := "school-one"
	yearID := uuid.NewString()
	template, err := domain.NewReportTemplate(tenantID, "Term report", yearID, "{{student_name}}")
	if err != nil {
		t.Fatalf("new template: %v", err)
	}
	if err := publisher.PublishReportTemplate(context.Background(), "report.created.v1", template, nil); err != nil {
		t.Fatalf("publish template: %v", err)
	}
	card, err := domain.NewReportCard(tenantID, uuid.NewString(), yearID, template.ID)
	if err != nil {
		t.Fatalf("new report card: %v", err)
	}
	card.TermID = uuid.NewString()
	card.SetPublished("private/report.pdf")
	if err := publisher.PublishReportCard(context.Background(), "report.published.v1", card, nil); err != nil {
		t.Fatalf("publish report card: %v", err)
	}
	if len(js.messages) != 2 {
		t.Fatalf("messages=%d", len(js.messages))
	}
	created := testkit.AssertEventContract(t, "report.created.v1", js.messages[0].Data)
	published := testkit.AssertEventContract(t, "report.published.v1", js.messages[1].Data)
	if created["subject"] != template.ID || published["subject"] != card.ID {
		t.Fatalf("subjects created=%v published=%v", created["subject"], published["subject"])
	}
}
