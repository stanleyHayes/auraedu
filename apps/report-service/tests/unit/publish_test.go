package unit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	svcevents "github.com/auraedu/report-service/internal/adapters/events"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/nats-io/nats.go"
)

func publishActor(tenantID string, perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantID, Permissions: perms}
}

func TestGenerateReportCard_PublishesWithMaterializedEntries(t *testing.T) {
	repo := newFakeRepo()
	pdfGen := &fakePDFGenerator{}
	pub := &fakePublisher{}
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(pdfGen),
		application.WithPublisher(pub),
		application.WithReportOutputDir(t.TempDir()),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})

	// Feed one score and one attendance day through the materialization path.
	if err := svc.MaterializeScore(ctx, scoreInput(tenantA)); err != nil {
		t.Fatalf("materialize score: %v", err)
	}
	if err := svc.MaterializeAttendance(ctx, application.AttendanceMarkedInput{
		EventID: "evt-a1", TenantID: tenantA, StudentID: studentA, Date: "2026-07-08", Status: "present",
	}); err != nil {
		t.Fatalf("materialize attendance: %v", err)
	}
	card, err := repo.FindDraftReportCard(ctx, tenantA, studentA, term1)
	if err != nil {
		t.Fatalf("find draft: %v", err)
	}

	published, err := svc.GenerateReportCard(ctx, publishActor(tenantA, application.PermPublish), card.ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published, got %q", published.Status)
	}
	if published.PDFPath == nil {
		t.Fatal("expected pdf_path")
	}
	if _, err := os.Stat(*published.PDFPath); err != nil {
		t.Fatalf("expected PDF file on disk: %v", err)
	}

	// The render model must carry the materialized data.
	if pdfGen.lastDoc == nil {
		t.Fatal("pdf generator was not called")
	}
	if len(pdfGen.lastDoc.Scores) != 1 || pdfGen.lastDoc.Scores[0].Score != 72 {
		t.Fatalf("expected aggregated score in document, got %+v", pdfGen.lastDoc.Scores)
	}
	if pdfGen.lastDoc.Attendance.Present != 1 {
		t.Fatalf("expected attendance summary in document, got %+v", pdfGen.lastDoc.Attendance)
	}

	// The publish transition must emit report.published.v1.
	var found bool
	for _, e := range pub.events {
		if e.eventType == "report.published.v1" && e.card != nil && e.card.ID == card.ID {
			found = true
			if e.card.TermID != term1 {
				t.Fatalf("published event card missing term: %+v", e.card)
			}
		}
	}
	if !found {
		t.Fatalf("expected report.published.v1 event, got %+v", pub.events)
	}
}

func TestGenerateReportCard_ConflictWhenGenerating(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(&fakePDFGenerator{}),
		application.WithReportOutputDir(t.TempDir()),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})

	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("new card: %v", err)
	}
	card.SetGenerating()
	if err := repo.CreateReportCard(ctx, tenantA, card); err != nil {
		t.Fatalf("create card: %v", err)
	}

	if _, err := svc.GenerateReportCard(ctx, publishActor(tenantA, application.PermPublish), card.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestGenerateReportCard_RequiresPublishPermission(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(&fakePDFGenerator{}),
		application.WithReportOutputDir(t.TempDir()),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})

	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("new card: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, card); err != nil {
		t.Fatalf("create card: %v", err)
	}

	if _, err := svc.GenerateReportCard(ctx, publishActor(tenantA, application.PermRead), card.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

// --- Event payload conformance (contracts/events/report.published.v1.json). ---

type captureJS struct {
	msg *nats.Msg
}

func (c *captureJS) PublishMsg(msg *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.msg = msg
	return &nats.PubAck{}, nil
}
func (c *captureJS) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) { return nil, nil }
func (c *captureJS) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nil
}
func (c *captureJS) Subscribe(string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error) {
	return nil, nil
}

func TestPublisher_ReportPublishedPayloadConformance(t *testing.T) {
	js := &captureJS{}
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))

	pdfPath := "/tmp/auraedu-reports/card.pdf"
	now := time.Now().UTC()
	card := &domain.ReportCard{
		ID:          "55555555-5555-5555-5555-555555555555",
		TenantID:    tenantA,
		StudentID:   studentA,
		TermID:      term1,
		Status:      string(domain.ReportCardStatusPublished),
		PDFPath:     &pdfPath,
		GeneratedAt: &now,
	}
	if err := pub.PublishReportCard(context.Background(), "report.published.v1", card, nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if js.msg == nil {
		t.Fatal("no message published")
	}

	var event map[string]any
	if err := json.Unmarshal(js.msg.Data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "report.published.v1" {
		t.Fatalf("type: got %v", event["type"])
	}
	if event["source"] != "report-service" {
		t.Fatalf("source: got %v", event["source"])
	}
	if event["tenant_id"] != tenantA {
		t.Fatalf("tenant_id: got %v", event["tenant_id"])
	}

	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data object: %v", event)
	}
	// Required by contracts/events/report.published.v1.json.
	for _, key := range []string{"report_card_id", "student_id", "term_id"} {
		if data[key] == nil || data[key] == "" {
			t.Fatalf("data.%s missing: %v", key, data)
		}
	}
	if data["report_card_id"] != card.ID || data["student_id"] != studentA || data["term_id"] != term1 {
		t.Fatalf("unexpected data: %v", data)
	}
	wantURL := "/api/v1/report-cards/" + card.ID + "/download"
	if data["file_url"] != wantURL {
		t.Fatalf("file_url: got %v, want %q", data["file_url"], wantURL)
	}
}
