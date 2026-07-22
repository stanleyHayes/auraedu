package ports

import (
	"context"

	"github.com/auraedu/crm-service/internal/domain"
)

type EventPublisher interface {
	LeadCreated(context.Context, *domain.Lead) error
	InteractionCreated(context.Context, *domain.Interaction) error
	FeedbackSubmitted(context.Context, *domain.Feedback) error
	CallbackRequested(context.Context, *domain.CallbackRequest) error
	LeadScored(context.Context, string, string, domain.LeadScore) error
}
