package unit

import (
	"testing"

	"github.com/auraedu/payment-service/internal/domain"
)

func TestNewWebhookEvent_RequiresProvider(t *testing.T) {
	if _, err := domain.NewWebhookEvent("", "charge.success", []byte(`{}`), nil); err == nil {
		t.Fatal("expected error when provider is empty")
	}
}

func TestNewWebhookEvent_RequiresEventType(t *testing.T) {
	if _, err := domain.NewWebhookEvent("mock", "", []byte(`{}`), nil); err == nil {
		t.Fatal("expected error when event_type is empty")
	}
}

func TestNewWebhookEvent_RequiresPayload(t *testing.T) {
	if _, err := domain.NewWebhookEvent("mock", "charge.success", nil, nil); err == nil {
		t.Fatal("expected error when payload is empty")
	}
}

func TestNewWebhookEvent_Valid(t *testing.T) {
	w, err := domain.NewWebhookEvent("mock", "charge.success", []byte(`{"reference":"ref-1"}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Provider != "mock" {
		t.Fatalf("expected provider mock, got %q", w.Provider)
	}
	if w.Processed {
		t.Fatal("expected webhook not processed")
	}
}

func TestWebhookEvent_MarkProcessed(t *testing.T) {
	w, err := domain.NewWebhookEvent("mock", "charge.success", []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w.MarkProcessed()
	if !w.Processed {
		t.Fatal("expected webhook processed")
	}
	if w.ProcessedAt == nil {
		t.Fatal("expected processed_at set")
	}
}
