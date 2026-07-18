// Package http provides the HTTP adapter for the notification service.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the notification use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/messages", h.listMessages)
	mux.HandleFunc("POST /api/v1/messages", h.createMessage)
	mux.HandleFunc("GET /api/v1/messages/{message_id}", h.getMessage)
	mux.HandleFunc("PATCH /api/v1/messages/{message_id}", h.updateMessage)
	mux.HandleFunc("DELETE /api/v1/messages/{message_id}", h.deleteMessage)
	mux.HandleFunc("POST /api/v1/messages/{message_id}/send", h.sendMessage)

	mux.HandleFunc("GET /api/v1/notification-templates", h.listTemplates)
	mux.HandleFunc("POST /api/v1/notification-templates", h.createTemplate)
	mux.HandleFunc("GET /api/v1/notification-templates/{template_id}", h.getTemplate)
	mux.HandleFunc("PATCH /api/v1/notification-templates/{template_id}", h.updateTemplate)
	mux.HandleFunc("DELETE /api/v1/notification-templates/{template_id}", h.deleteTemplate)

	mux.HandleFunc("GET /api/v1/notification-subscriptions", h.listSubscriptions)
	mux.HandleFunc("POST /api/v1/notification-subscriptions", h.createSubscription)
	mux.HandleFunc("GET /api/v1/notification-subscriptions/{subscription_id}", h.getSubscription)
	mux.HandleFunc("PATCH /api/v1/notification-subscriptions/{subscription_id}", h.updateSubscription)
	mux.HandleFunc("DELETE /api/v1/notification-subscriptions/{subscription_id}", h.deleteSubscription)

	h.registerAnnouncements(mux)
}

// --- Message handlers ---

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	filter := ports.MessageFilter{
		Limit:       parseLimit(r.URL.Query().Get("limit")),
		Cursor:      r.URL.Query().Get("cursor"),
		Channel:     r.URL.Query().Get("channel"),
		Status:      r.URL.Query().Get("status"),
		RecipientID: r.URL.Query().Get("recipient_id"),
	}
	records, nextCursor, err := h.svc.ListMessages(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpRespondList(w, r, records, nextCursor)
}

type createMessageBody struct {
	RecipientID string         `json:"recipient_id"`
	Channel     string         `json:"channel"`
	TemplateID  *string        `json:"template_id,omitempty"`
	Subject     string         `json:"subject"`
	Body        string         `json:"body"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ScheduledAt *string        `json:"scheduled_at,omitempty"`
}

func (h *Handler) createMessage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateMessage(ctx, actor, application.CreateMessageRequest{
		RecipientID: body.RecipientID,
		Channel:     body.Channel,
		TemplateID:  body.TemplateID,
		Subject:     body.Subject,
		Body:        body.Body,
		Metadata:    body.Metadata,
		ScheduledAt: body.ScheduledAt,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getMessage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetMessage(ctx, actor, r.PathValue("message_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateMessageBody struct {
	RecipientID *string        `json:"recipient_id,omitempty"`
	Channel     *string        `json:"channel,omitempty"`
	TemplateID  *string        `json:"template_id,omitempty"`
	Subject     *string        `json:"subject,omitempty"`
	Body        *string        `json:"body,omitempty"`
	Status      *string        `json:"status,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	ScheduledAt *string        `json:"scheduled_at,omitempty"`
	SentAt      *string        `json:"sent_at,omitempty"`
	Error       *string        `json:"error,omitempty"`
}

func (h *Handler) updateMessage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateMessage(ctx, actor, r.PathValue("message_id"), application.UpdateMessageRequest{
		RecipientID: body.RecipientID,
		Channel:     body.Channel,
		TemplateID:  body.TemplateID,
		Subject:     body.Subject,
		Body:        body.Body,
		Status:      body.Status,
		Metadata:    body.Metadata,
		ScheduledAt: body.ScheduledAt,
		SentAt:      body.SentAt,
		Error:       body.Error,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteMessage(ctx, actor, r.PathValue("message_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.SendMessage(ctx, actor, r.PathValue("message_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

// --- Template handlers ---

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	filter := ports.TemplateFilter{
		Limit:   parseLimit(r.URL.Query().Get("limit")),
		Cursor:  r.URL.Query().Get("cursor"),
		Channel: r.URL.Query().Get("channel"),
		Status:  r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListTemplates(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpRespondList(w, r, records, nextCursor)
}

type createTemplateBody struct {
	Name            string `json:"name"`
	Channel         string `json:"channel"`
	SubjectTemplate string `json:"subject_template"`
	BodyTemplate    string `json:"body_template"`
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateTemplate(ctx, actor, application.CreateTemplateRequest{
		Name:            body.Name,
		Channel:         body.Channel,
		SubjectTemplate: body.SubjectTemplate,
		BodyTemplate:    body.BodyTemplate,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetTemplate(ctx, actor, r.PathValue("template_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateTemplateBody struct {
	Name            *string `json:"name,omitempty"`
	Channel         *string `json:"channel,omitempty"`
	SubjectTemplate *string `json:"subject_template,omitempty"`
	BodyTemplate    *string `json:"body_template,omitempty"`
	Status          *string `json:"status,omitempty"`
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateTemplate(ctx, actor, r.PathValue("template_id"), application.UpdateTemplateRequest{
		Name:            body.Name,
		Channel:         body.Channel,
		SubjectTemplate: body.SubjectTemplate,
		BodyTemplate:    body.BodyTemplate,
		Status:          body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteTemplate(ctx, actor, r.PathValue("template_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Subscription handlers ---

func (h *Handler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	filter := ports.SubscriptionFilter{
		Limit:   parseLimit(r.URL.Query().Get("limit")),
		Cursor:  r.URL.Query().Get("cursor"),
		Channel: r.URL.Query().Get("channel"),
		UserID:  r.URL.Query().Get("user_id"),
	}
	records, nextCursor, err := h.svc.ListSubscriptions(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpRespondList(w, r, records, nextCursor)
}

type createSubscriptionBody struct {
	UserID    string `json:"user_id"`
	Channel   string `json:"channel"`
	IsEnabled bool   `json:"is_enabled"`
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createSubscriptionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateSubscription(ctx, actor, application.CreateSubscriptionRequest{
		UserID:    body.UserID,
		Channel:   body.Channel,
		IsEnabled: body.IsEnabled,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetSubscription(ctx, actor, r.PathValue("subscription_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateSubscriptionBody struct {
	Channel   *string `json:"channel,omitempty"`
	IsEnabled *bool   `json:"is_enabled,omitempty"`
}

func (h *Handler) updateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateSubscriptionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateSubscription(ctx, actor, r.PathValue("subscription_id"), application.UpdateSubscriptionRequest{
		Channel:   body.Channel,
		IsEnabled: body.IsEnabled,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteSubscription(ctx, actor, r.PathValue("subscription_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func (h *Handler) context(r *http.Request) (context.Context, auth.Actor, bool) {
	actor := auth.FromHeaders(r.Header)
	tenantID := r.Header.Get(tenancy.HeaderTenantID)
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-Code")
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{
		TenantID:  tenantID,
		RequestID: r.Header.Get(tenancy.HeaderRequestID),
		ActorID:   actor.UserID,
		ActorRole: actor.Role,
	})
	ctx = auth.WithActor(ctx, actor)
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureNotifications)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func httpRespondList(w http.ResponseWriter, r *http.Request, data any, nextCursor string) {
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": data, "next_cursor": nullIfEmpty(nextCursor)})
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}
