package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Subscription is the aggregate root for a user's channel subscription preferences.
type Subscription struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Channel   string    `json:"channel"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSubscription constructs a Subscription, enforcing invariants.
func NewSubscription(tenantID, userID, channel string, isEnabled bool) (*Subscription, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrValidation)
	}
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("%w: channel is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(channel)) {
		return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("notifications: generate subscription id: %w", err)
	}
	now := time.Now().UTC()
	return &Subscription{
		ID:        id.String(),
		TenantID:  tenantID,
		UserID:    strings.TrimSpace(userID),
		Channel:   strings.TrimSpace(strings.ToLower(channel)),
		IsEnabled: isEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (s Subscription) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.UserID) == "" {
		return fmt.Errorf("%w: user_id is required", ErrValidation)
	}
	if !isValidChannel(NotificationChannel(s.Channel)) {
		return fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
	}
	return nil
}

// SubscriptionPatch carries optional update fields.
type SubscriptionPatch struct {
	Channel   *string
	IsEnabled *bool
}

// ApplyUpdate mutates the subscription with non-nil patch fields.
func (s *Subscription) ApplyUpdate(patch SubscriptionPatch) ([]string, error) {
	var changed []string

	if patch.Channel != nil {
		if !isValidChannel(NotificationChannel(*patch.Channel)) {
			return nil, fmt.Errorf("%w: channel must be email, sms, whatsapp, in_app or push", ErrValidation)
		}
		s.Channel = strings.TrimSpace(strings.ToLower(*patch.Channel))
		changed = append(changed, "channel")
	}
	if patch.IsEnabled != nil {
		s.IsEnabled = *patch.IsEnabled
		changed = append(changed, "is_enabled")
	}

	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}
