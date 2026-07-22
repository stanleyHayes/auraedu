//nolint:lll // Constructor validation is kept together as a single policy boundary.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Feedback is prospect input held for human review. It is never promoted to
// prompts, knowledge or autonomous actions directly.
type Feedback struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	InteractionID *string   `json:"interaction_id,omitempty"`
	AIRunID       *string   `json:"ai_run_id,omitempty"`
	FeedbackType  string    `json:"feedback_type"`
	Rating        *int      `json:"rating,omitempty"`
	Comment       *string   `json:"comment,omitempty"`
	ReviewStatus  string    `json:"review_status"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewFeedback(tenantID string, interactionID, aiRunID *string, feedbackType string, rating *int, comment *string) (*Feedback, error) {
	feedbackType = strings.ToLower(strings.TrimSpace(feedbackType))
	if strings.TrimSpace(tenantID) == "" {
		return nil, ErrMissingTenant
	}
	if !validFeedbackType(feedbackType) {
		return nil, fmt.Errorf("%w: invalid feedback_type", ErrValidation)
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return nil, fmt.Errorf("%w: rating must be between 1 and 5", ErrValidation)
	}
	if comment != nil {
		trimmed := strings.TrimSpace(*comment)
		if len(trimmed) > 2000 {
			return nil, fmt.Errorf("%w: comment exceeds 2000 characters", ErrValidation)
		}
		comment = &trimmed
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("crm: generate feedback id: %w", err)
	}
	return &Feedback{ID: id.String(), TenantID: tenantID, InteractionID: interactionID, AIRunID: aiRunID, FeedbackType: feedbackType, Rating: rating, Comment: comment, ReviewStatus: "pending", CreatedAt: time.Now().UTC()}, nil
}

func validFeedbackType(value string) bool {
	switch value {
	case "helpful", "unhelpful", "incorrect", "outdated", "unresolved", "escalation_requested":
		return true
	default:
		return false
	}
}
