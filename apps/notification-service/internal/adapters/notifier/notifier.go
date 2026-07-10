package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
)

// MockNotifier is a deterministic notifier for tests and local development.
// It succeeds unless the message body contains the word "fail" (case-insensitive).
type MockNotifier struct {
	channel string
}

// NewMockNotifier creates a mock notifier for the given channel.
func NewMockNotifier(channel string) *MockNotifier {
	return &MockNotifier{channel: channel}
}

// Send attempts to deliver the message. It fails when the body contains "fail".
func (n *MockNotifier) Send(ctx context.Context, msg domain.Message) error {
	_ = ctx
	if strings.Contains(strings.ToLower(msg.Body), "fail") {
		return fmt.Errorf("mock %s notifier: forced failure", n.channel)
	}
	return nil
}

// Registry returns a map of mock notifiers for all supported channels.
func Registry() map[string]ports.Notifier {
	return map[string]ports.Notifier{
		"email":    NewMockNotifier("email"),
		"sms":      NewMockNotifier("sms"),
		"whatsapp": NewMockNotifier("whatsapp"),
		"in_app":   NewMockNotifier("in_app"),
	}
}

var _ ports.Notifier = (*MockNotifier)(nil)

// ErrNoNotifier is returned when a channel has no registered notifier.
var ErrNoNotifier = errors.New("notifications: no notifier configured for channel")
