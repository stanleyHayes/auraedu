package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
)

// eventRule maps a consumed domain event to the notification it triggers.
type eventRule struct {
	// flagKey gates the side effect (agent_plan §9); the event is acked and
	// skipped when the flag is off for the tenant.
	flagKey string
	// channel is the preferred delivery channel. When the recipient has no
	// enabled subscription for it, the worker falls back to the in-app inbox.
	channel string
	// build derives recipient, subject and body from the event payload. It
	// reports ok=false when the event is not notification-worthy (e.g. a
	// student marked present).
	build func(data map[string]any) (recipient, subject, body string, ok bool)
}

// eventRules is the event → notification mapping for the worker. Event types
// are matched without the ".v1" suffix so both "payment.received" and
// "payment.received.v1" CloudEvent types land on the same rule.
var eventRules = map[string]eventRule{
	"payment.received": {
		flagKey: FeatureEmailNotifications,
		channel: string(domain.ChannelEmail),
		build: func(data map[string]any) (string, string, string, bool) {
			recipient := firstField(data, "guardian_id", "user_id", "student_id")
			subject := "Payment received"
			body := fmt.Sprintf("A payment of %s was received for invoice %s.",
				amountField(data, "amount"), stringField(data, "invoice_id"))
			return recipient, subject, body, true
		},
	},
	"invoice.created": {
		flagKey: FeatureEmailNotifications,
		channel: string(domain.ChannelEmail),
		build: func(data map[string]any) (string, string, string, bool) {
			recipient := firstField(data, "guardian_id", "user_id", "student_id")
			subject := "New invoice issued"
			body := fmt.Sprintf("A new invoice for %s has been issued.", amountField(data, "amount_due"))
			if due := stringField(data, "due_date"); due != "" {
				body += fmt.Sprintf(" Payment is due on %s.", due)
			}
			return recipient, subject, body, true
		},
	},
	"attendance.marked": {
		flagKey: FeatureSMSNotifications,
		channel: string(domain.ChannelSMS),
		build: func(data map[string]any) (string, string, string, bool) {
			// Only guardian-alert-worthy statuses trigger a notification.
			status := stringField(data, "status")
			switch status {
			case "absent", "late":
			default:
				return "", "", "", false
			}
			recipient := firstField(data, "guardian_id", "user_id", "student_id")
			subject := "Attendance alert"
			body := fmt.Sprintf("A student was marked %s on %s.", status, stringField(data, "date"))
			return recipient, subject, body, true
		},
	},
	"assessment.score_recorded": {
		flagKey: FeatureEmailNotifications,
		channel: string(domain.ChannelEmail),
		build: func(data map[string]any) (string, string, string, bool) {
			recipient := firstField(data, "guardian_id", "user_id", "student_id")
			subject := "New score recorded"
			body := fmt.Sprintf("A score of %s was recorded.", amountField(data, "score"))
			if max := amountField(data, "max_score"); max != "" {
				body = fmt.Sprintf("A score of %s out of %s was recorded.", amountField(data, "score"), max)
			}
			return recipient, subject, body, true
		},
	},
	"report.published": {
		flagKey: FeatureEmailNotifications,
		channel: string(domain.ChannelEmail),
		build: func(data map[string]any) (string, string, string, bool) {
			recipient := firstField(data, "guardian_id", "user_id", "student_id")
			subject := "Report card published"
			body := "A report card has been published."
			return recipient, subject, body, true
		},
	},
}

// HandleCloudEvent applies the notification side effects for one consumed domain
// event: feature-flag gate → idempotency claim → create pending message →
// deliver via the channel notifier (MockNotifier today). Unknown event types,
// flag-off events and duplicate deliveries are acked without side effects.
// A non-nil error means the event should be redelivered.
func (s *Service) HandleCloudEvent(ctx context.Context, event tenancy.CloudEvent) error {
	base := strings.TrimSuffix(event.Type, ".v1")
	rule, ok := eventRules[base]
	if !ok {
		return nil
	}
	tenantID := event.TenantID
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, rule.flagKey) {
		slog.Default().InfoContext(ctx, "notification side effect skipped; feature disabled",
			"event_type", base, "flag", rule.flagKey, "tenant_id", tenantID)
		return nil
	}

	if s.processedRepo != nil {
		claimed, err := s.processedRepo.Claim(ctx, tenantID, event.ID, base)
		if err != nil {
			return err
		}
		if !claimed {
			// Duplicate redelivery of an already-processed event.
			return nil
		}
	}

	if err := s.dispatchEvent(ctx, tenantID, base, rule, event); err != nil {
		// Release the claim so the redelivery can retry the side effect.
		if s.processedRepo != nil {
			if rErr := s.processedRepo.Release(ctx, tenantID, event.ID); rErr != nil {
				slog.Default().ErrorContext(ctx, "failed to release processed-event claim", "event_id", event.ID, "err", rErr)
			}
		}
		return err
	}
	return nil
}

// dispatchEvent builds the message for one event and sends it.
func (s *Service) dispatchEvent(ctx context.Context, tenantID, base string, rule eventRule, event tenancy.CloudEvent) error {
	var data map[string]any
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("notifications: decode %s payload: %w", base, err)
		}
	}

	recipient, subject, body, ok := rule.build(data)
	if !ok {
		return nil
	}

	channel := rule.channel
	metadata := map[string]any{
		"event_id":   event.ID,
		"event_type": base,
		"source":     "worker",
	}
	if recipient == "" {
		// No resolvable recipient in the payload: drop the notification into the
		// tenant's in-app inbox (recipient_id = tenant_id convention).
		recipient = tenantID
		channel = string(domain.ChannelInApp)
		metadata["audience"] = "tenant"
	} else if channel != string(domain.ChannelInApp) && !s.hasEnabledSubscription(ctx, tenantID, recipient, channel) {
		// Preferred channel unavailable for this recipient: fall back to in-app.
		channel = string(domain.ChannelInApp)
	}

	m, err := domain.NewMessage(tenantID, recipient, channel, subject, body, nil, metadata, nil)
	if err != nil {
		return err
	}
	if err := s.messageRepo.Create(ctx, tenantID, m); err != nil {
		return err
	}
	_, err = s.deliver(ctx, tenantID, m)
	return err
}

// hasEnabledSubscription reports whether the recipient has an enabled
// subscription for the channel. When no subscription repository is configured
// the preferred channel is kept.
func (s *Service) hasEnabledSubscription(ctx context.Context, tenantID, userID, channel string) bool {
	if s.subscriptionRepo == nil {
		return true
	}
	subs, _, err := s.subscriptionRepo.List(ctx, tenantID, ports.SubscriptionFilter{
		Limit:   1,
		Channel: channel,
		UserID:  userID,
	})
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to check channel subscription", "channel", channel, "err", err)
		return false
	}
	return len(subs) > 0 && subs[0].IsEnabled
}

// firstField returns the first non-empty string value among keys.
func firstField(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := stringField(data, k); v != "" {
			return v
		}
	}
	return ""
}

func stringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// amountField renders a numeric payload field compactly (72.5 → "72.5", 72 → "72").
func amountField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	switch v := data[key].(type) {
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	case string:
		return strings.TrimSpace(v)
	}
	return ""
}
