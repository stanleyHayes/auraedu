// Package webhooks verifies and minimizes external delivery callbacks.
package webhooks

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/google/uuid"
	svix "github.com/svix/svix-webhooks/go"
)

var (
	ErrInvalidSignature = errors.New("notifications: invalid Resend webhook signature")
	ErrInvalidPayload   = errors.New("notifications: invalid Resend webhook payload")
)

type ResendVerifier struct {
	webhook *svix.Webhook
}

func NewResendVerifier(secret string) (*ResendVerifier, error) {
	webhook, err := svix.NewWebhook(strings.TrimSpace(secret))
	if err != nil {
		return nil, fmt.Errorf("notifications: configure Resend webhook verifier: %w", err)
	}
	return &ResendVerifier{webhook: webhook}, nil
}

// Verify authenticates the exact request bytes and maps only lifecycle events
// needed for operational delivery state. Open/click events are intentionally
// ignored so AuraEDU does not create an engagement-surveillance dataset.
func (v *ResendVerifier) Verify(payload []byte, headers http.Header) (ports.DeliveryFeedback, bool, error) {
	if v == nil || v.webhook == nil || v.webhook.Verify(payload, headers) != nil {
		return ports.DeliveryFeedback{}, false, ErrInvalidSignature
	}
	var event struct {
		Type      string    `json:"type"`
		CreatedAt time.Time `json:"created_at"`
		Data      struct {
			EmailID string            `json:"email_id"`
			To      []string          `json:"to"`
			Tags    map[string]string `json:"tags"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return ports.DeliveryFeedback{}, false, ErrInvalidPayload
	}
	status, relevant := resendDeliveryStatus(event.Type)
	if !relevant {
		return ports.DeliveryFeedback{}, false, nil
	}
	messageID := strings.TrimSpace(event.Data.Tags["aura_message"])
	providerMessageID := strings.TrimSpace(event.Data.EmailID)
	if _, err := uuid.Parse(messageID); err != nil {
		return ports.DeliveryFeedback{}, false, ErrInvalidPayload
	}
	if _, err := uuid.Parse(providerMessageID); err != nil || event.CreatedAt.IsZero() || len(event.Data.To) != 1 {
		return ports.DeliveryFeedback{}, false, ErrInvalidPayload
	}
	address := strings.ToLower(strings.TrimSpace(event.Data.To[0]))
	if address == "" || !strings.Contains(address, "@") {
		return ports.DeliveryFeedback{}, false, ErrInvalidPayload
	}
	eventID := strings.TrimSpace(headers.Get("svix-id"))
	if eventID == "" {
		eventID = strings.TrimSpace(headers.Get("webhook-id"))
	}
	if eventID == "" || len(eventID) > 255 {
		return ports.DeliveryFeedback{}, false, ErrInvalidPayload
	}
	return ports.DeliveryFeedback{
		ID:                eventID,
		Provider:          "resend",
		ProviderMessageID: providerMessageID,
		MessageID:         messageID,
		EventType:         event.Type,
		Status:            status,
		AddressHash:       fmt.Sprintf("%x", sha256.Sum256([]byte(address))),
		OccurredAt:        event.CreatedAt.UTC(),
	}, true, nil
}

func resendDeliveryStatus(eventType string) (string, bool) {
	switch eventType {
	case "email.sent":
		return string(domain.DeliveryStatusAccepted), true
	case "email.delivered":
		return string(domain.DeliveryStatusDelivered), true
	case "email.delivery_delayed":
		return string(domain.DeliveryStatusDelayed), true
	case "email.bounced":
		return string(domain.DeliveryStatusBounced), true
	case "email.complained":
		return string(domain.DeliveryStatusComplained), true
	case "email.failed":
		return string(domain.DeliveryStatusFailed), true
	case "email.suppressed":
		return string(domain.DeliveryStatusSuppressed), true
	default:
		return "", false
	}
}
