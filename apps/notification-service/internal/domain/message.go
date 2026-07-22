// Package domain holds the notification-service aggregate roots and value objects.
package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NotificationChannel enumerates the supported notification channels.
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"
	ChannelSMS      NotificationChannel = "sms"
	ChannelWhatsApp NotificationChannel = "whatsapp"
	ChannelInApp    NotificationChannel = "in_app"
	ChannelPush     NotificationChannel = "push"
)

// MessageStatus enumerates the lifecycle states of a message.
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusFailed    MessageStatus = "failed"
	MessageStatusCancelled MessageStatus = "cancelled"
)

// DeliveryStatus records the provider-observed notification lifecycle independently
// from MessageStatus, whose "sent" value means the provider accepted AuraEDU's
// delivery request.
type DeliveryStatus string

const (
	DeliveryStatusAccepted   DeliveryStatus = "accepted"
	DeliveryStatusDelivered  DeliveryStatus = "delivered"
	DeliveryStatusDelayed    DeliveryStatus = "delayed"
	DeliveryStatusBounced    DeliveryStatus = "bounced"
	DeliveryStatusComplained DeliveryStatus = "complained"
	DeliveryStatusFailed     DeliveryStatus = "failed"
	DeliveryStatusSuppressed DeliveryStatus = "suppressed"
)

// Message is the aggregate root for a notification message.
type Message struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	RecipientID      string         `json:"recipient_id"`
	Channel          string         `json:"channel"`
	TemplateID       *string        `json:"template_id,omitempty"`
	Subject          string         `json:"subject"`
	Body             string         `json:"body"`
	Status           string         `json:"status"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	ScheduledAt      *time.Time     `json:"scheduled_at,omitempty"`
	SentAt           *time.Time     `json:"sent_at,omitempty"`
	Error            *string        `json:"error,omitempty"`
	Provider         *string        `json:"provider,omitempty"`
	DeliveryStatus   *string        `json:"delivery_status,omitempty"`
	DeliveryStatusAt *time.Time     `json:"delivery_status_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// NewMessage constructs a Message, enforcing invariants.
func NewMessage(tenantID, recipientID, channel, subject, body string, templateID *string, metadata map[string]any, scheduledAt *time.Time) (*Message, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(recipientID) == "" {
		return nil, fmt.Errorf("%w: recipient_id is required", ErrValidation)
	}
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("%w: channel is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(channel)) {
		return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
	}
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("%w: subject is required", ErrValidation)
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%w: body is required", ErrValidation)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("notifications: generate message id: %w", err)
	}
	now := time.Now().UTC()
	return &Message{
		ID:          id.String(),
		TenantID:    tenantID,
		RecipientID: strings.TrimSpace(recipientID),
		Channel:     strings.TrimSpace(strings.ToLower(channel)),
		TemplateID:  templateID,
		Subject:     strings.TrimSpace(subject),
		Body:        body,
		Status:      string(MessageStatusPending),
		Metadata:    metadata,
		ScheduledAt: scheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (m Message) Validate() error {
	if m.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(m.RecipientID) == "" {
		return fmt.Errorf("%w: recipient_id is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(m.Channel)) {
		return fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
	}
	if strings.TrimSpace(m.Subject) == "" {
		return fmt.Errorf("%w: subject is required", ErrValidation)
	}
	if strings.TrimSpace(m.Body) == "" {
		return fmt.Errorf("%w: body is required", ErrValidation)
	}
	if !isValidMessageStatus(MessageStatus(m.Status)) {
		return fmt.Errorf("%w: status must be pending, sent, failed or cancelled", ErrValidation)
	}
	if m.DeliveryStatus != nil && !isValidDeliveryStatus(DeliveryStatus(*m.DeliveryStatus)) {
		return fmt.Errorf("%w: delivery_status is invalid", ErrValidation)
	}
	return nil
}

// MessagePatch carries optional update fields.
type MessagePatch struct {
	RecipientID *string
	Channel     *string
	TemplateID  *string
	Subject     *string
	Body        *string
	Status      *string
	Metadata    map[string]any
	ScheduledAt *time.Time
	SentAt      *time.Time
	Error       *string
}

// ApplyUpdate mutates the message with non-nil patch fields.
func (m *Message) ApplyUpdate(patch MessagePatch) ([]string, error) {
	var changed []string

	if patch.RecipientID != nil {
		if strings.TrimSpace(*patch.RecipientID) == "" {
			return nil, fmt.Errorf("%w: recipient_id cannot be empty", ErrValidation)
		}
		m.RecipientID = strings.TrimSpace(*patch.RecipientID)
		changed = append(changed, "recipient_id")
	}
	if patch.Channel != nil {
		if !isValidChannel(NotificationChannel(*patch.Channel)) {
			return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
		}
		m.Channel = strings.TrimSpace(strings.ToLower(*patch.Channel))
		changed = append(changed, "channel")
	}
	if patch.TemplateID != nil {
		m.TemplateID = patch.TemplateID
		changed = append(changed, "template_id")
	}
	if patch.Subject != nil {
		if strings.TrimSpace(*patch.Subject) == "" {
			return nil, fmt.Errorf("%w: subject cannot be empty", ErrValidation)
		}
		m.Subject = strings.TrimSpace(*patch.Subject)
		changed = append(changed, "subject")
	}
	if patch.Body != nil {
		if strings.TrimSpace(*patch.Body) == "" {
			return nil, fmt.Errorf("%w: body cannot be empty", ErrValidation)
		}
		m.Body = *patch.Body
		changed = append(changed, "body")
	}
	if patch.Status != nil {
		if !isValidMessageStatus(MessageStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be pending, sent, failed or cancelled", ErrValidation)
		}
		m.Status = *patch.Status
		changed = append(changed, "status")
	}
	if patch.Metadata != nil {
		m.Metadata = patch.Metadata
		changed = append(changed, "metadata")
	}
	if patch.ScheduledAt != nil {
		m.ScheduledAt = patch.ScheduledAt
		changed = append(changed, "scheduled_at")
	}
	if patch.SentAt != nil {
		m.SentAt = patch.SentAt
		changed = append(changed, "sent_at")
	}
	if patch.Error != nil {
		m.Error = patch.Error
		changed = append(changed, "error")
	}

	if len(changed) > 0 {
		m.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// MarkSent transitions the message to sent status.
func (m *Message) MarkSent() {
	now := time.Now().UTC()
	m.Status = string(MessageStatusSent)
	m.SentAt = &now
	m.Error = nil
	m.UpdatedAt = now
}

// MarkProviderAccepted records the provider receipt without exposing the
// provider's message identifier through the public Message representation.
func (m *Message) MarkProviderAccepted(provider string, at time.Time) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	status := string(DeliveryStatusAccepted)
	at = at.UTC()
	m.Provider = &provider
	m.DeliveryStatus = &status
	m.DeliveryStatusAt = &at
	m.UpdatedAt = at
}

// MarkFailed transitions the message to failed status and records the error.
func (m *Message) MarkFailed(reason string) {
	now := time.Now().UTC()
	m.Status = string(MessageStatusFailed)
	m.Error = &reason
	m.UpdatedAt = now
}

// MarkCancelled prevents a pending message from reaching a provider.
func (m *Message) MarkCancelled(reason string) {
	now := time.Now().UTC()
	m.Status = string(MessageStatusCancelled)
	if strings.TrimSpace(reason) == "" {
		m.Error = nil
	} else {
		reason = strings.TrimSpace(reason)
		m.Error = &reason
	}
	m.UpdatedAt = now
}

// MarshalMetadata returns the metadata as JSON bytes for persistence.
func (m Message) MarshalMetadata() ([]byte, error) {
	if m.Metadata == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m.Metadata)
}

func isValidChannel(c NotificationChannel) bool {
	switch c {
	case ChannelEmail, ChannelSMS, ChannelWhatsApp, ChannelInApp, ChannelPush:
		return true
	}
	return false
}

func isValidMessageStatus(s MessageStatus) bool {
	switch s {
	case MessageStatusPending, MessageStatusSent, MessageStatusFailed, MessageStatusCancelled:
		return true
	}
	return false
}

func isValidDeliveryStatus(s DeliveryStatus) bool {
	switch s {
	case DeliveryStatusAccepted, DeliveryStatusDelivered, DeliveryStatusDelayed,
		DeliveryStatusBounced, DeliveryStatusComplained, DeliveryStatusFailed, DeliveryStatusSuppressed:
		return true
	}
	return false
}
