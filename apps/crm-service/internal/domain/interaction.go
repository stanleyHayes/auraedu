//nolint:lll // Constructor arguments mirror the interaction contract fields.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Interaction struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	LeadID     string    `json:"lead_id"`
	Channel    string    `json:"channel"`
	Direction  string    `json:"direction"`
	ActorType  string    `json:"actor_type"`
	ActorID    *string   `json:"actor_id,omitempty"`
	Summary    string    `json:"summary"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewInteraction(tenantID, leadID, channel, direction, actorType, summary string, actorID *string, occurredAt *time.Time) (*Interaction, error) {
	tenantID, leadID = strings.TrimSpace(tenantID), strings.TrimSpace(leadID)
	channel, direction, actorType, summary = strings.TrimSpace(channel), strings.TrimSpace(direction), strings.TrimSpace(actorType), strings.TrimSpace(summary)
	if tenantID == "" || leadID == "" || !validInteractionChannel(channel) ||
		(direction != "inbound" && direction != "outbound") || summary == "" {
		return nil, ErrValidation
	}
	if actorType != "prospect" && actorType != "staff" && actorType != "ai" && actorType != "system" {
		return nil, ErrValidation
	}
	when := time.Now().UTC()
	if occurredAt != nil {
		when = occurredAt.UTC()
	}
	return &Interaction{ID: uuid.NewString(), TenantID: tenantID, LeadID: leadID, Channel: channel, Direction: direction, ActorType: actorType, ActorID: actorID, Summary: summary, OccurredAt: when}, nil
}

func validInteractionChannel(value string) bool {
	switch value {
	case "website", "email", "sms", "whatsapp", "phone", "in_person", "event", "social", "other":
		return true
	default:
		return false
	}
}
