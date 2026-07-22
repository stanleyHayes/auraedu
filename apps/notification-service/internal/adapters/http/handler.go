// Package http provides the HTTP adapter for the notification service.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	providerwebhooks "github.com/auraedu/notification-service/internal/adapters/webhooks"
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
	svc            *application.Service
	internalToken  string
	resendVerifier deliveryWebhookVerifier
	twilioVerifier twilioWebhookVerifier
}

type deliveryWebhookVerifier interface {
	Verify([]byte, http.Header) (ports.DeliveryFeedback, bool, error)
}

type twilioWebhookVerifier interface {
	Verify(string, url.Values, string) (ports.DeliveryFeedback, bool, error)
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) WithInternalToken(token string) *Handler { h.internalToken = token; return h }

func (h *Handler) WithResendWebhookVerifier(verifier deliveryWebhookVerifier) *Handler {
	h.resendVerifier = verifier
	return h
}

func (h *Handler) WithTwilioWebhookVerifier(verifier twilioWebhookVerifier) *Handler {
	h.twilioVerifier = verifier
	return h
}

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/messages", h.listMessages)
	mux.HandleFunc("POST /api/v1/messages", h.createMessage)
	mux.HandleFunc("GET /api/v1/messages/{message_id}", h.getMessage)
	mux.HandleFunc("PATCH /api/v1/messages/{message_id}", h.updateMessage)
	mux.HandleFunc("DELETE /api/v1/messages/{message_id}", h.deleteMessage)
	mux.HandleFunc("POST /api/v1/messages/{message_id}/send", h.sendMessage)
	mux.HandleFunc("POST /api/v1/webhooks/resend", h.resendWebhook)
	mux.HandleFunc("POST /api/v1/webhooks/twilio", h.twilioWebhook)
	mux.HandleFunc("POST /api/v1/email-preferences/unsubscribe", h.unsubscribeEmail)

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
	mux.HandleFunc("POST /api/v1/device-tokens", h.registerDeviceToken)
	mux.HandleFunc("DELETE /api/v1/device-tokens/{device_id}", h.unregisterDeviceToken)

	h.registerAnnouncements(mux)
	h.registerJourneys(mux)
	mux.HandleFunc("POST /internal/v1/transactional-email", h.transactionalEmail)
}

func (h *Handler) unsubscribeEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid email preference request"})
		return
	}
	if err := h.svc.UnsubscribeEmail(r.Context(), body.Token); err != nil {
		if errors.Is(err, domain.ErrValidation) {
			httpx.ValidationError(w, r, map[string]any{"body": "invalid or expired request"})
			return
		}
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resendWebhook(w http.ResponseWriter, r *http.Request) {
	if h.resendVerifier == nil {
		httpx.RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{"error": "unavailable", "message": "delivery feedback is not configured"})
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128<<10))
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "unreadable or too large"})
		return
	}
	feedback, relevant, err := h.resendVerifier.Verify(payload, r.Header)
	if errors.Is(err, providerwebhooks.ErrInvalidSignature) {
		httpx.Unauthorized(w, r, "invalid webhook signature")
		return
	}
	if errors.Is(err, providerwebhooks.ErrInvalidPayload) {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid provider event"})
		return
	}
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
		return
	}
	if !relevant {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := h.svc.ApplyDeliveryFeedback(r.Context(), feedback); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnavailable) {
			// A provider callback can race the transaction that records its receipt.
			// A 503 asks Resend to retry rather than silently losing the event.
			httpx.RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{
				"error": "delivery_feedback_unavailable", "message": "delivery feedback could not be applied",
			})
			return
		}
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) twilioWebhook(w http.ResponseWriter, r *http.Request) {
	if h.twilioVerifier == nil {
		httpx.RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{"error": "unavailable", "message": "delivery feedback is not configured"})
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		httpx.ValidationError(w, r, map[string]any{"body": "Twilio callback must be form encoded"})
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128<<10))
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "unreadable or too large"})
		return
	}
	values, err := url.ParseQuery(string(payload))
	messageIDs := r.URL.Query()["message_id"]
	if err != nil || len(messageIDs) != 1 || strings.TrimSpace(messageIDs[0]) == "" {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid provider event"})
		return
	}
	feedback, relevant, err := h.twilioVerifier.Verify(messageIDs[0], values, r.Header.Get("X-Twilio-Signature"))
	if errors.Is(err, providerwebhooks.ErrInvalidTwilioSignature) {
		httpx.Unauthorized(w, r, "invalid webhook signature")
		return
	}
	if errors.Is(err, providerwebhooks.ErrInvalidTwilioPayload) {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid provider event"})
		return
	}
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
		return
	}
	if !relevant {
		w.WriteHeader(http.StatusOK)
		return
	}
	if _, err := h.svc.ApplyDeliveryFeedback(r.Context(), feedback); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnavailable) {
			// A callback can race the transaction that records its Twilio receipt.
			// A 503 asks Twilio to retry instead of silently losing the event.
			httpx.RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{
				"error": "delivery_feedback_unavailable", "message": "delivery feedback could not be applied",
			})
			return
		}
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) registerDeviceToken(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	var body struct {
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"`
		Token    string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body) != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	device, err := h.svc.RegisterDeviceToken(ctx, actor, body.DeviceID, body.Platform, body.Token)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, device)
}
func (h *Handler) unregisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	if err := h.svc.UnregisterDeviceToken(ctx, actor, r.PathValue("device_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type transactionalEmailBody struct {
	TenantID  string         `json:"tenant_id"`
	Recipient string         `json:"recipient"`
	Template  string         `json:"template"`
	Data      map[string]any `json:"data"`
}

func (h *Handler) transactionalEmail(w http.ResponseWriter, r *http.Request) {
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if h.internalToken == "" || len(provided) != len(h.internalToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(h.internalToken)) != 1 {
		httpx.Unauthorized(w, r, "invalid service credential")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var body transactionalEmailBody
	if err := dec.Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{TenantID: body.TenantID, ActorRole: "internal_service"})
	record, err := h.svc.DeliverTransactionalEmail(ctx, body.TenantID, body.Recipient, body.Template, body.Data)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusAccepted, map[string]any{"message_id": record.ID, "status": record.Status})
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
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondJSON(w, r, http.StatusConflict, map[string]any{"code": "conflict", "message": "invalid lifecycle transition"})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureNotifications)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrUnavailable):
		httpx.RespondJSON(w, r, http.StatusServiceUnavailable, map[string]any{"error": "unavailable", "message": "push token storage is unavailable"})
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
