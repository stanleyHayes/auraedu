package unit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	svcevents "github.com/auraedu/report-service/internal/adapters/events"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	"github.com/nats-io/nats.go"
)

func publishActor(perms ...string) auth.Actor {
	return auth.Actor{UserID: "user-1", TenantID: tenantA, Permissions: perms}
}

func TestQueuedReportCardGeneration_PublishesWithMaterializedEntries(t *testing.T) {
	repo := newFakeRepo()
	pdfGen := &fakePDFGenerator{}
	store := newFakeReportStorage()
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(pdfGen),
		application.WithStorage(store),
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

	queued, err := svc.RequestReportCardGeneration(ctx, publishActor(application.PermPublish), card.ID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if queued.Status != string(domain.ReportCardStatusGenerating) {
		t.Fatalf("expected generating, got %q", queued.Status)
	}
	processed, err := svc.ProcessNextGeneration(context.Background(), time.Minute, 5)
	if err != nil || !processed {
		t.Fatalf("process generation: processed=%v err=%v", processed, err)
	}
	published, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatalf("get published card: %v", err)
	}
	if published.Status != string(domain.ReportCardStatusPublished) {
		t.Fatalf("expected published, got %q", published.Status)
	}
	if published.PDFPath == nil {
		t.Fatal("expected pdf_path")
	}
	if store.objects[*published.PDFPath] != "%PDF-1.7 fake" {
		t.Fatalf("expected durable PDF object at %q", *published.PDFPath)
	}
	reader, _, err := svc.DownloadReportCard(ctx, publishActor(application.PermRead), card.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	downloaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read downloaded PDF: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close downloaded PDF: %v", err)
	}
	if string(downloaded) != "%PDF-1.7 fake" {
		t.Fatalf("unexpected downloaded PDF: %q", downloaded)
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
}

func TestRequestReportCardGeneration_ConflictWhenGenerating(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(&fakePDFGenerator{}),
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

	if _, err := svc.RequestReportCardGeneration(ctx, publishActor(application.PermPublish), card.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRequestReportCardGeneration_RequiresPublishPermission(t *testing.T) {
	repo := newFakeRepo()
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(&fakePDFGenerator{}),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})

	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatalf("new card: %v", err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, card); err != nil {
		t.Fatalf("create card: %v", err)
	}

	if _, err := svc.RequestReportCardGeneration(ctx, publishActor(application.PermRead), card.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestQueuedReportCardGeneration_TerminalStorageFailureRestoresDraft(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeReportStorage()
	store.fail = errors.New("object store unavailable")
	svc := application.NewService(repo,
		application.WithFeatureGate(enabledGates(tenantA)),
		application.WithPDFGenerator(&fakePDFGenerator{}),
		application.WithStorage(store),
	)
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenantA})
	card, err := domain.NewReportCard(tenantA, studentA, ay1, template1)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateReportCard(ctx, tenantA, card); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestReportCardGeneration(ctx, publishActor(application.PermPublish), card.ID); err != nil {
		t.Fatal(err)
	}
	processed, err := svc.ProcessNextGeneration(context.Background(), time.Minute, 1)
	if !processed || err == nil {
		t.Fatalf("expected terminal processing error, processed=%v err=%v", processed, err)
	}
	got, err := repo.GetReportCardByID(ctx, tenantA, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != string(domain.ReportCardStatusDraft) {
		t.Fatalf("terminal failure status = %q, want draft", got.Status)
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
	if _, exposed := data["pdf_path"]; exposed {
		t.Fatalf("private storage path leaked into event: %v", data)
	}
	wantURL := "/api/v1/report-cards/" + card.ID + "/download"
	if data["file_url"] != wantURL {
		t.Fatalf("file_url: got %v, want %q", data["file_url"], wantURL)
	}
}

func TestPublisher_OutboxLifecyclePreservesSubjectAndOmitsEmptyUUIDs(t *testing.T) {
	js := &captureJS{}
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))

	template, err := domain.NewReportTemplate(tenantA, "Term report", ay1, "# Report")
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.PublishWithID(context.Background(), "event-template", "report.created.v1", tenantA, ports.ReportTemplateEventData(template, nil)); err != nil {
		t.Fatalf("publish template outbox event: %v", err)
	}
	var templateEvent map[string]any
	if err := json.Unmarshal(js.msg.Data, &templateEvent); err != nil {
		t.Fatal(err)
	}
	if templateEvent["subject"] != template.ID || templateEvent["id"] != "event-template" {
		t.Fatalf("template identity not preserved: %v", templateEvent)
	}
	if js.msg.Header.Get("Nats-Msg-Id") != "event-template" {
		t.Fatalf("stable broker id missing: %v", js.msg.Header)
	}

	card := &domain.ReportCard{
		ID:             "55555555-5555-5555-5555-555555555555",
		TenantID:       tenantA,
		StudentID:      studentA,
		AcademicYearID: ay1,
		Status:         string(domain.ReportCardStatusDraft),
	}
	if err := pub.PublishWithID(
		context.Background(),
		"event-card",
		"report.created.v1",
		tenantA,
		ports.ReportCardEventData("report.created.v1", card, nil),
	); err != nil {
		t.Fatalf("publish card outbox event: %v", err)
	}
	var cardEvent map[string]any
	if err := json.Unmarshal(js.msg.Data, &cardEvent); err != nil {
		t.Fatal(err)
	}
	data, ok := cardEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing card event data: %v", cardEvent)
	}
	if _, present := data["term_id"]; present {
		t.Fatalf("empty term_id must be omitted, got %v", data)
	}
	if _, present := data["template_id"]; present {
		t.Fatalf("empty template_id must be omitted, got %v", data)
	}
	if cardEvent["subject"] != card.ID || cardEvent["id"] != "event-card" {
		t.Fatalf("card identity not preserved: %v", cardEvent)
	}
}
