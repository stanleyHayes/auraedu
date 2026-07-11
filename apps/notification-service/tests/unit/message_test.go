package unit

import (
	"strings"
	"testing"

	"github.com/auraedu/notification-service/internal/domain"
)

func TestNewMessage_RequiresTenant(t *testing.T) {
	if _, err := domain.NewMessage("", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewMessage_RequiresRecipient(t *testing.T) {
	if _, err := domain.NewMessage("tenant-1", "", "email", "Subject", "Body", nil, nil, nil); err == nil {
		t.Fatal("expected error when recipient_id is empty")
	}
}

func TestNewMessage_ValidatesChannel(t *testing.T) {
	if _, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "pager", "Subject", "Body", nil, nil, nil); err == nil {
		t.Fatal("expected error for invalid channel")
	}
}

func TestNewMessage_Valid(t *testing.T) {
	m, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != string(domain.MessageStatusPending) {
		t.Fatalf("expected pending status, got %q", m.Status)
	}
	if m.Channel != "email" {
		t.Fatalf("expected normalized channel email, got %q", m.Channel)
	}
}

func TestMessage_ApplyUpdate(t *testing.T) {
	m, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	status := string(domain.MessageStatusSent)
	changed, err := m.ApplyUpdate(domain.MessagePatch{Status: &status})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "status" {
		t.Fatalf("expected status changed, got %v", changed)
	}
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", m.Status)
	}
}

func TestMessage_MarkSent(t *testing.T) {
	m, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	m.MarkSent()
	if m.Status != string(domain.MessageStatusSent) {
		t.Fatalf("expected sent status, got %q", m.Status)
	}
	if m.SentAt == nil {
		t.Fatal("expected sent_at set")
	}
	if m.Error != nil {
		t.Fatal("expected error cleared")
	}
}

func TestMessage_MarkFailed(t *testing.T) {
	m, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	m.MarkFailed("boom")
	if m.Status != string(domain.MessageStatusFailed) {
		t.Fatalf("expected failed status, got %q", m.Status)
	}
	if m.Error == nil || !strings.Contains(*m.Error, "boom") {
		t.Fatalf("expected error set, got %v", m.Error)
	}
}

func TestMessage_ApplyUpdate_RejectsInvalidStatus(t *testing.T) {
	m, err := domain.NewMessage("tenant-1", "550e8400-e29b-41d4-a716-446655440000", "email", "Subject", "Body", nil, nil, nil)
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	status := "unknown"
	if _, err := m.ApplyUpdate(domain.MessagePatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
