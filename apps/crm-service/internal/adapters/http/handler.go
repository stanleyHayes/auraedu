// Package http adapts the Growth CRM contract to application use cases.
//
//nolint:lll // Explicit HTTP request mappings remain adjacent for contract review.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/crm-service/internal/application"
	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

type Handler struct{ service *application.Service }

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/public/leads", h.capture)
	mux.HandleFunc("POST /api/v1/public/feedback", h.submitFeedback)
	mux.HandleFunc("POST /api/v1/public/callback-requests", h.scheduleCallback)
	mux.HandleFunc("GET /api/v1/callback-requests", h.listCallbacks)
	mux.HandleFunc("GET /api/v1/leads", h.listLeads)
	mux.HandleFunc("GET /api/v1/leads/{lead_id}", h.getLead)
	mux.HandleFunc("PATCH /api/v1/leads/{lead_id}", h.updateLead)
	mux.HandleFunc("POST /api/v1/leads/{lead_id}/score", h.rescoreLead)
	mux.HandleFunc("GET /api/v1/leads/{lead_id}/interactions", h.listInteractions)
	mux.HandleFunc("POST /api/v1/leads/{lead_id}/interactions", h.createInteraction)
}

func (h *Handler) rescoreLead(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	score, err := h.service.RescoreLead(ctx, actor, r.PathValue("lead_id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, score)
}

type callbackBody struct {
	FirstName   string      `json:"first_name"`
	LastName    string      `json:"last_name"`
	Email       *string     `json:"email"`
	Phone       string      `json:"phone"`
	PreferredAt time.Time   `json:"preferred_at"`
	Timezone    string      `json:"timezone"`
	Locale      string      `json:"locale"`
	Message     string      `json:"message"`
	Consent     consentBody `json:"consent"`
}

func (h *Handler) scheduleCallback(w http.ResponseWriter, r *http.Request) {
	tenantID, key := strings.TrimSpace(r.Header.Get("X-Tenant-Code")), strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var body callbackBody
	if tenantID == "" || key == "" || decode(r, &body) != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	phone, message := body.Phone, body.Message
	result, err := h.service.ScheduleCallback(r.Context(), tenantID, key, application.ScheduleCallbackRequest{
		FirstName: body.FirstName, LastName: body.LastName, Email: body.Email, Phone: &phone,
		PreferredAt: body.PreferredAt, Timezone: body.Timezone, Locale: body.Locale, Message: &message,
		Consent: domain.Consent{PrivacyNoticeVersion: body.Consent.PrivacyNoticeVersion, Email: body.Consent.Email, SMS: body.Consent.SMS, WhatsApp: body.Consent.WhatsApp, Voice: body.Consent.Voice},
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	if result.Replay {
		status = http.StatusOK
	}
	httpx.RespondJSON(w, r, status, result.Callback)
}

func (h *Handler) listCallbacks(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	n, err := limit(r)
	if err != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	status := domain.CallbackStatus(strings.TrimSpace(r.URL.Query().Get("status")))
	items, err := h.service.ListCallbacks(ctx, actor, status, n)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /internal/v1/leads/{lead_id}/welcome-recipient", func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.Header.Get("Authorization") != "Bearer "+token {
			httpx.Unauthorized(w, r, "valid service credentials are required")
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get(tenancy.HeaderTenantID))
		recipient, err := h.service.ResolveWelcomeRecipient(r.Context(), tenantID, r.PathValue("lead_id"))
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		httpx.RespondJSON(w, r, http.StatusOK, recipient)
	})
}

type feedbackBody struct {
	InteractionID *string `json:"interaction_id"`
	AIRunID       *string `json:"ai_run_id"`
	FeedbackType  string  `json:"feedback_type"`
	Rating        *int    `json:"rating"`
	Comment       *string `json:"comment"`
}

func (h *Handler) submitFeedback(w http.ResponseWriter, r *http.Request) {
	tenantID, key := strings.TrimSpace(r.Header.Get("X-Tenant-Code")), strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var body feedbackBody
	if tenantID == "" || key == "" || decode(r, &body) != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	_, err := h.service.SubmitFeedback(r.Context(), tenantID, key, application.SubmitFeedbackRequest{InteractionID: body.InteractionID, AIRunID: body.AIRunID, FeedbackType: body.FeedbackType, Rating: body.Rating, Comment: body.Comment})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type consentBody struct {
	PrivacyNoticeVersion string `json:"privacy_notice_version"`
	Email                bool   `json:"email"`
	SMS                  bool   `json:"sms"`
	WhatsApp             bool   `json:"whatsapp"`
	Voice                bool   `json:"voice"`
}
type captureBody struct {
	FirstName             string      `json:"first_name"`
	LastName              string      `json:"last_name"`
	Email                 *string     `json:"email"`
	Phone                 *string     `json:"phone"`
	InstitutionID         *string     `json:"institution_id"`
	PreferredProgrammeIDs []string    `json:"preferred_programme_ids"`
	PreferredIntakeID     *string     `json:"preferred_intake_id"`
	Source                string      `json:"source"`
	CampaignID            *string     `json:"campaign_id"`
	Message               *string     `json:"message"`
	Consent               consentBody `json:"consent"`
}

func (h *Handler) capture(w http.ResponseWriter, r *http.Request) {
	tenantID, key := strings.TrimSpace(r.Header.Get("X-Tenant-Code")), strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var body captureBody
	if tenantID == "" || key == "" || decode(r, &body) != nil {
		httpx.ValidationError(w, r, map[string]any{"detail": "tenant, idempotency key and valid JSON are required"})
		return
	}
	result, err := h.service.Capture(r.Context(), tenantID, key, application.CaptureRequest{FirstName: body.FirstName, LastName: body.LastName, Email: body.Email, Phone: body.Phone, InstitutionID: body.InstitutionID, ProgrammeIDs: body.PreferredProgrammeIDs, IntakeID: body.PreferredIntakeID, Source: body.Source, CampaignID: body.CampaignID, Message: body.Message, Consent: domain.Consent{PrivacyNoticeVersion: body.Consent.PrivacyNoticeVersion, Email: body.Consent.Email, SMS: body.Consent.SMS, WhatsApp: body.Consent.WhatsApp, Voice: body.Consent.Voice}})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	httpx.RespondJSON(w, r, status, map[string]any{"lead_id": result.Lead.ID, "created": result.Created, "stage": result.Lead.Stage})
}

func (h *Handler) listLeads(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	limit, err := limit(r)
	if err != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	filter := ports.LeadFilter{Search: strings.TrimSpace(r.URL.Query().Get("search"))}
	if value := r.URL.Query().Get("stage"); value != "" {
		stage, e := domain.ParseLeadStage(value)
		if e != nil {
			h.writeError(w, r, e)
			return
		}
		filter.Stage = &stage
	}
	if value := strings.TrimSpace(r.URL.Query().Get("owner_user_id")); value != "" {
		filter.OwnerUserID = &value
	}
	items, next, err := h.service.ListLeads(ctx, actor, limit, r.URL.Query().Get("cursor"), filter)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": items, "next_cursor": nullable(next)})
}
func (h *Handler) getLead(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	lead, err := h.service.GetLead(ctx, actor, r.PathValue("lead_id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, lead)
}

type updateBody struct {
	Stage                 *string   `json:"stage"`
	OwnerUserID           *string   `json:"owner_user_id"`
	PreferredProgrammeIDs *[]string `json:"preferred_programme_ids"`
}

func (h *Handler) updateLead(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	var body updateBody
	if decode(r, &body) != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	var stage *domain.LeadStage
	if body.Stage != nil {
		parsed, err := domain.ParseLeadStage(*body.Stage)
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		stage = &parsed
	}
	lead, err := h.service.UpdateLead(ctx, actor, r.PathValue("lead_id"), application.UpdateLeadRequest{Stage: stage, OwnerUserID: body.OwnerUserID, ProgrammeIDs: body.PreferredProgrammeIDs})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, lead)
}

type interactionBody struct {
	Channel    string  `json:"channel"`
	Direction  string  `json:"direction"`
	Summary    string  `json:"summary"`
	OccurredAt *string `json:"occurred_at"`
}

func (h *Handler) createInteraction(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	var body interactionBody
	if decode(r, &body) != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	var occurred *time.Time
	if body.OccurredAt != nil {
		parsed, err := time.Parse(time.RFC3339, *body.OccurredAt)
		if err != nil {
			h.writeError(w, r, domain.ErrValidation)
			return
		}
		occurred = &parsed
	}
	item, err := h.service.CreateInteraction(ctx, actor, r.PathValue("lead_id"), body.Channel, body.Direction, body.Summary, occurred)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, item)
}
func (h *Handler) listInteractions(w http.ResponseWriter, r *http.Request) {
	ctx, actor := requestContext(r)
	n, err := limit(r)
	if err != nil {
		h.writeError(w, r, domain.ErrValidation)
		return
	}
	items, next, err := h.service.ListInteractions(ctx, actor, r.PathValue("lead_id"), n, r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": items, "next_cursor": nullable(next)})
}

func requestContext(r *http.Request) (context.Context, auth.Actor) {
	actor := auth.FromHeaders(r.Header)
	tenantID := r.Header.Get(tenancy.HeaderTenantID)
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-Code")
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{TenantID: tenantID, RequestID: r.Header.Get(tenancy.HeaderRequestID), ActorID: actor.UserID, ActorRole: actor.Role})
	return auth.WithActor(ctx, actor), actor
}
func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
func limit(r *http.Request) (int, error) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": "request failed validation"})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "lead")
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondJSON(w, r, http.StatusConflict, httpx.Error{Code: httpx.ErrValidation, Message: "idempotency key conflicts with an earlier request"})
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrUnauthorized):
		httpx.Unauthorized(w, r, "authentication is required")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, flags.ErrFeatureDisabled):
		feature := application.FeatureGrowthCRM
		if strings.Contains(err.Error(), application.FeatureLeadScoring) {
			feature = application.FeatureLeadScoring
		}
		httpx.FeatureDisabled(w, r, feature)
	default:
		slog.ErrorContext(r.Context(), "crm request failed", "method", r.Method, "path", r.URL.Path, "request_id", r.Header.Get(tenancy.HeaderRequestID), "err", err)
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}
