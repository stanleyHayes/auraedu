package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	workerTenant = "11111111-1111-1111-1111-111111111111"
	studentID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

// newWorkerService builds the worker-side service with in-memory fakes and all
// channel flags enabled for workerTenant.
func newWorkerService() (*application.Service, *fakeMessageRepo, *fakeSubscriptionRepo, *fakeProcessedEventRepo, *flags.StaticSnapshot) {
	messages := newFakeMessageRepo()
	subscriptions := newFakeSubscriptionRepo()
	processed := newFakeProcessedEventRepo()
	gates := flags.NewStaticSnapshot()
	for _, key := range []string{
		application.FeatureEmailNotifications,
		application.FeatureSMSNotifications,
		application.FeatureWhatsAppNotifications,
	} {
		gates.Set(workerTenant, key, true)
	}
	svc := application.NewService(messages, nil, subscriptions,
		application.WithFeatureGate(gates),
		application.WithNotifiers(notifier.Registry()),
		application.WithProcessedEventRepository(processed),
	)
	return svc, messages, subscriptions, processed, gates
}

func cloudEvent(t *testing.T, eventType, id string, data map[string]any) tenancy.CloudEvent {
	t.Helper()
	event, err := tenancy.NewCloudEvent(eventType, "test", id, workerTenant, data)
	if err != nil {
		t.Fatalf("new cloud event: %v", err)
	}
	return event
}

func workerCtx() context.Context {
	return tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: workerTenant})
}

func TestHandleCloudEvent_PaymentReceived_TenantInboxFallback(t *testing.T) {
	svc, messages, _, _, _ := newWorkerService()
	ctx := workerCtx()

	// payment.received.v1 payload carries no recipient field (see contract), so
	// the notification lands in the tenant's in-app inbox.
	event := cloudEvent(t, "payment.received.v1", "evt-pay-1", map[string]any{
		"payment_id": "p-1",
		"invoice_id": "inv-1",
		"amount":     500,
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := messages.all(workerTenant)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0]
	if m.Channel != string(domain.ChannelInApp) {
		t.Fatalf("expected in_app channel, got %q", m.Channel)
	}
	if m.RecipientID != workerTenant {
		t.Fatalf("expected tenant inbox recipient, got %q", m.RecipientID)
	}
	if m.Subject != "Payment received" {
		t.Fatalf("unexpected subject %q", m.Subject)
	}
	if !strings.Contains(m.Body, "500") || !strings.Contains(m.Body, "inv-1") {
		t.Fatalf("body should mention amount and invoice, got %q", m.Body)
	}
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", m.Status)
	}
	if m.Metadata["event_id"] != "evt-pay-1" {
		t.Fatalf("expected event_id metadata, got %v", m.Metadata)
	}
}

func TestHandleCloudEvent_InvoiceCreated_EmailToStudent(t *testing.T) {
	svc, messages, subscriptions, _, _ := newWorkerService()
	subscriptions.add(workerTenant, studentID, "email", true)
	ctx := workerCtx()

	event := cloudEvent(t, "invoice.created.v1", "evt-inv-1", map[string]any{
		"invoice_id": "inv-9",
		"student_id": studentID,
		"amount_due": 1200,
		"due_date":   "2026-08-01",
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := messages.all(workerTenant)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0]
	if m.Channel != string(domain.ChannelEmail) {
		t.Fatalf("expected email channel, got %q", m.Channel)
	}
	if m.RecipientID != studentID {
		t.Fatalf("expected student recipient, got %q", m.RecipientID)
	}
	if m.Subject != "New invoice issued" {
		t.Fatalf("unexpected subject %q", m.Subject)
	}
	if !strings.Contains(m.Body, "1200") || !strings.Contains(m.Body, "2026-08-01") {
		t.Fatalf("body should mention amount and due date, got %q", m.Body)
	}
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", m.Status)
	}
}

func TestHandleCloudEvent_AttendanceMarked_AbsentAlertsGuardianChannel(t *testing.T) {
	svc, messages, subscriptions, _, _ := newWorkerService()
	subscriptions.add(workerTenant, studentID, "sms", true)
	ctx := workerCtx()

	event := cloudEvent(t, "attendance.marked.v1", "evt-att-1", map[string]any{
		"student_id": studentID,
		"date":       "2026-07-18",
		"status":     "absent",
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := messages.all(workerTenant)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0]
	if m.Channel != string(domain.ChannelSMS) {
		t.Fatalf("expected sms channel, got %q", m.Channel)
	}
	if m.Subject != "Attendance alert" {
		t.Fatalf("unexpected subject %q", m.Subject)
	}
	if !strings.Contains(m.Body, "absent") || !strings.Contains(m.Body, "2026-07-18") {
		t.Fatalf("body should mention status and date, got %q", m.Body)
	}
}

func TestHandleCloudEvent_AttendanceMarked_PresentSkips(t *testing.T) {
	svc, messages, _, _, _ := newWorkerService()
	ctx := workerCtx()

	for _, status := range []string{"present", "excused"} {
		event := cloudEvent(t, "attendance.marked.v1", "evt-att-"+status, map[string]any{
			"student_id": studentID,
			"date":       "2026-07-18",
			"status":     status,
		})
		if err := svc.HandleCloudEvent(ctx, event); err != nil {
			t.Fatalf("handle %s: %v", status, err)
		}
	}
	if got := messages.all(workerTenant); len(got) != 0 {
		t.Fatalf("expected no messages for present/excused, got %d", len(got))
	}
}

func TestHandleCloudEvent_ScoreRecorded_FallsBackToInAppWithoutSubscription(t *testing.T) {
	svc, messages, _, _, _ := newWorkerService()
	ctx := workerCtx()

	// No email subscription for the student: preferred email channel falls back
	// to the in-app inbox.
	event := cloudEvent(t, "assessment.score_recorded.v1", "evt-score-1", map[string]any{
		"student_id": studentID,
		"subject_id": "subj-1",
		"score":      72,
		"max_score":  100,
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := messages.all(workerTenant)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0]
	if m.Channel != string(domain.ChannelInApp) {
		t.Fatalf("expected in_app fallback, got %q", m.Channel)
	}
	if m.RecipientID != studentID {
		t.Fatalf("expected student recipient, got %q", m.RecipientID)
	}
	if m.Subject != "New score recorded" {
		t.Fatalf("unexpected subject %q", m.Subject)
	}
	if !strings.Contains(m.Body, "72") || !strings.Contains(m.Body, "100") {
		t.Fatalf("body should mention score and max, got %q", m.Body)
	}
}

func TestHandleCloudEvent_ReportPublished(t *testing.T) {
	svc, messages, subscriptions, _, _ := newWorkerService()
	subscriptions.add(workerTenant, studentID, "email", true)
	ctx := workerCtx()

	event := cloudEvent(t, "report.published.v1", "evt-rep-1", map[string]any{
		"report_card_id": "rc-1",
		"student_id":     studentID,
		"term_id":        "term-1",
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := messages.all(workerTenant)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0]
	if m.Subject != "Report card published" {
		t.Fatalf("unexpected subject %q", m.Subject)
	}
	if m.RecipientID != studentID {
		t.Fatalf("expected student recipient, got %q", m.RecipientID)
	}
}

func TestHandleCloudEvent_FlagDisabledSkips(t *testing.T) {
	svc, messages, _, processed, gates := newWorkerService()
	gates.Set(workerTenant, application.FeatureEmailNotifications, false)
	ctx := workerCtx()

	event := cloudEvent(t, "payment.received.v1", "evt-pay-off", map[string]any{
		"payment_id": "p-1",
		"invoice_id": "inv-1",
		"amount":     10,
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got := messages.all(workerTenant); len(got) != 0 {
		t.Fatalf("expected no messages when flag off, got %d", len(got))
	}
	// Flag-off events are acked without claiming, so a later redelivery with the
	// flag enabled can still notify.
	if processed.claimed(workerTenant, "evt-pay-off") {
		t.Fatal("flag-off event should not be claimed")
	}
}

func TestHandleCloudEvent_IdempotentRedelivery(t *testing.T) {
	svc, messages, _, processed, _ := newWorkerService()
	ctx := workerCtx()

	event := cloudEvent(t, "report.published.v1", "evt-rep-dup", map[string]any{
		"report_card_id": "rc-1",
		"student_id":     studentID,
		"term_id":        "term-1",
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("first handle: %v", err)
	}
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("second handle: %v", err)
	}
	if got := messages.all(workerTenant); len(got) != 1 {
		t.Fatalf("expected exactly 1 message after redelivery, got %d", len(got))
	}
	if !processed.claimed(workerTenant, "evt-rep-dup") {
		t.Fatal("expected event to remain claimed")
	}
}

func TestHandleCloudEvent_DeliveryFailureReleasesClaimForRetry(t *testing.T) {
	svc, messages, _, processed, _ := newWorkerService()
	ctx := workerCtx()

	// MockNotifier fails when the body contains "fail"; the attendance date is
	// interpolated into the body, so this event always fails delivery.
	event := cloudEvent(t, "attendance.marked.v1", "evt-att-fail", map[string]any{
		"student_id": studentID,
		"date":       "fail",
		"status":     "absent",
	})
	if err := svc.HandleCloudEvent(ctx, event); err == nil {
		t.Fatal("expected delivery error")
	}
	if processed.claimed(workerTenant, "evt-att-fail") {
		t.Fatal("claim should be released after a failed side effect")
	}

	// A redelivery re-attempts the side effect (and fails again here), proving
	// the claim was released rather than swallowed as a duplicate.
	if err := svc.HandleCloudEvent(ctx, event); err == nil {
		t.Fatal("expected delivery error on retry")
	}
	if got := messages.all(workerTenant); len(got) != 2 {
		t.Fatalf("expected 2 message attempts, got %d", len(got))
	}
	for _, m := range messages.all(workerTenant) {
		if m.Status != string(domain.MessageStatusFailed) {
			t.Fatalf("expected failed status, got %q", m.Status)
		}
	}
}

func TestHandleCloudEvent_UnknownTypeIsNoOp(t *testing.T) {
	svc, messages, _, _, _ := newWorkerService()
	ctx := workerCtx()

	event := cloudEvent(t, "student.enrolled.v1", "evt-unknown", map[string]any{
		"student_id": studentID,
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got := messages.all(workerTenant); len(got) != 0 {
		t.Fatalf("expected no messages for unknown type, got %d", len(got))
	}
}

func TestHandleCloudEvent_AcceptsTypeWithoutVersionSuffix(t *testing.T) {
	svc, messages, _, _, _ := newWorkerService()
	ctx := workerCtx()

	// payment.received.v1.json declares type "payment.received" (no suffix).
	event := cloudEvent(t, "payment.received", "evt-pay-nosuffix", map[string]any{
		"payment_id": "p-1",
		"invoice_id": "inv-1",
		"amount":     5,
	})
	if err := svc.HandleCloudEvent(ctx, event); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got := messages.all(workerTenant); len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
}
