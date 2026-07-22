// Package ports defines Admissions Service boundaries.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
)

type CatalogueRepository interface {
	CreateProgramme(context.Context, domain.Programme) error
	GetProgramme(context.Context, string, string) (domain.Programme, error)
	ListProgrammes(context.Context, string, bool, time.Time, int) ([]domain.Programme, error)
	UpdateProgramme(context.Context, domain.Programme, int) error
	CreateIntake(context.Context, domain.Intake) error
	GetIntake(context.Context, string, string) (domain.Intake, error)
	UpdateIntake(context.Context, domain.Intake, int) error
	ResolveAvailableIntake(context.Context, string, string, string, time.Time) (domain.Programme, domain.Intake, error)
}

type Repository interface {
	Create(context.Context, domain.Application) error
	Get(context.Context, string, string) (domain.Application, error)
	List(context.Context, string, string, domain.Status, int) ([]domain.Application, error)
	Update(context.Context, domain.Application, domain.Status) error
}

type EventPublisher interface {
	Publish(context.Context, string, string, map[string]any) error
}

// ApplicationEventData builds the exact public payload for an admissions
// lifecycle event. It intentionally keys fields by event type instead of by
// aggregate state so strict schemas never receive data belonging to a later
// lifecycle stage.
func ApplicationEventData(eventType string, application domain.Application, at time.Time) map[string]any {
	payload := map[string]any{
		"application_id":    application.ID,
		"applicant_user_id": application.ApplicantUserID,
		"lead_id":           application.LeadID,
		"programme_id":      application.ProgrammeID,
		"intake_id":         application.IntakeID,
	}
	switch eventType {
	case "application.started.v1":
		payload["started_at"] = at.UTC().Format(time.RFC3339)
	case "application.submitted.v1":
		payload["submitted_at"] = at.UTC().Format(time.RFC3339)
	case "application.admitted.v1":
		payload["reviewed_at"] = at.UTC().Format(time.RFC3339)
		if application.ReviewedBy != nil {
			payload["reviewed_by"] = *application.ReviewedBy
		}
	case "offer.issued.v1":
		payload["issued_at"] = at.UTC().Format(time.RFC3339)
		if application.OfferExpiresAt != nil {
			payload["offer_expires_at"] = application.OfferExpiresAt.UTC().Format(time.RFC3339)
		}
	case "offer.accepted.v1":
		payload["accepted_at"] = at.UTC().Format(time.RFC3339)
	}
	return payload
}

type DocumentVerifier interface {
	Verify(context.Context, string, string, string) error
}
type TransactionalRepository interface {
	CreateWithEvent(context.Context, domain.Application, string, map[string]any) error
	UpdateWithEvent(context.Context, domain.Application, domain.Status, string, map[string]any) error
}
type CatalogueTransactionalRepository interface {
	CreateForAvailableIntake(context.Context, domain.Application, time.Time, string, map[string]any) error
}
type CatalogueEventRepository interface {
	CreateProgrammeWithEvent(context.Context, domain.Programme, string, map[string]any) error
	UpdateProgrammeWithEvent(context.Context, domain.Programme, int, string, map[string]any) error
	CreateIntakeWithEvent(context.Context, domain.Intake, string, map[string]any) error
	UpdateIntakeWithEvent(context.Context, domain.Intake, int, string, map[string]any) error
}
type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
	CreatedAt               time.Time
}
type OutboxRepository interface {
	ClaimPending(context.Context, int) ([]OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}
