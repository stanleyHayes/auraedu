package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the payment use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/payments", h.listPayments)
	mux.HandleFunc("POST /api/v1/payments", h.createPayment)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}", h.getPayment)
	mux.HandleFunc("PATCH /api/v1/payments/{payment_id}", h.updatePayment)
	mux.HandleFunc("DELETE /api/v1/payments/{payment_id}", h.deletePayment)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/initiate", h.initiatePayment)

	mux.HandleFunc("GET /api/v1/payments/{payment_id}/transactions", h.listTransactionsByPayment)
	mux.HandleFunc("GET /api/v1/transactions/{transaction_id}", h.getTransaction)

	mux.HandleFunc("GET /api/v1/webhook-events", h.listWebhookEvents)
	mux.HandleFunc("POST /api/v1/webhook-events", h.createWebhookEvent)
	mux.HandleFunc("GET /api/v1/webhook-events/{event_id}", h.getWebhookEvent)
	mux.HandleFunc("POST /api/v1/webhooks/{provider}", h.processWebhook)
}

// --- Payment handlers ---

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.PaymentFilter{
		Limit:     limit,
		Cursor:    r.URL.Query().Get("cursor"),
		Status:    r.URL.Query().Get("status"),
		Provider:  r.URL.Query().Get("provider"),
		InvoiceID: r.URL.Query().Get("invoice_id"),
	}
	records, nextCursor, err := h.svc.ListPayments(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createPaymentBody struct {
	InvoiceID   string          `json:"invoice_id"`
	AmountCents int             `json:"amount_cents"`
	Currency    string          `json:"currency"`
	Provider    string          `json:"provider"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createPaymentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreatePayment(ctx, actor, application.CreatePaymentRequest{
		InvoiceID:   body.InvoiceID,
		AmountCents: body.AmountCents,
		Currency:    body.Currency,
		Provider:    body.Provider,
		Metadata:    body.Metadata,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetPayment(ctx, actor, r.PathValue("payment_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updatePaymentBody struct {
	AmountCents       *int            `json:"amount_cents,omitempty"`
	Currency          *string         `json:"currency,omitempty"`
	Provider          *string         `json:"provider,omitempty"`
	ProviderReference *string         `json:"provider_reference,omitempty"`
	Status            *string         `json:"status,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CompletedAt       *string         `json:"completed_at,omitempty"`
}

func (h *Handler) updatePayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updatePaymentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdatePayment(ctx, actor, r.PathValue("payment_id"), application.UpdatePaymentRequest{
		AmountCents:       body.AmountCents,
		Currency:          body.Currency,
		Provider:          body.Provider,
		ProviderReference: body.ProviderReference,
		Status:            body.Status,
		Metadata:          body.Metadata,
		CompletedAt:       body.CompletedAt,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deletePayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeletePayment(ctx, actor, r.PathValue("payment_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) initiatePayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.InitiatePayment(ctx, actor, r.PathValue("payment_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

// --- Transaction handlers ---

func (h *Handler) listTransactionsByPayment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.TransactionFilter{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	}
	records, nextCursor, err := h.svc.ListTransactionsByPayment(ctx, actor, r.PathValue("payment_id"), filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

func (h *Handler) getTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetTransaction(ctx, actor, r.PathValue("transaction_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

// --- Webhook event handlers ---

func (h *Handler) listWebhookEvents(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.WebhookEventFilter{
		Limit:     limit,
		Cursor:    r.URL.Query().Get("cursor"),
		Provider:  r.URL.Query().Get("provider"),
		EventType: r.URL.Query().Get("event_type"),
	}
	records, nextCursor, err := h.svc.ListWebhookEvents(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createWebhookEventBody struct {
	Provider  string          `json:"provider"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Signature *string         `json:"signature,omitempty"`
}

func (h *Handler) createWebhookEvent(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createWebhookEventBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateWebhookEvent(ctx, actor, application.CreateWebhookEventRequest{
		Provider:  body.Provider,
		EventType: body.EventType,
		Payload:   body.Payload,
		Signature: body.Signature,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getWebhookEvent(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetWebhookEvent(ctx, actor, r.PathValue("event_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) processWebhook(w http.ResponseWriter, r *http.Request) {
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	sig := r.Header.Get("X-Webhook-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Paystack-Signature")
	}
	if sig == "" {
		sig = r.Header.Get("X-Flutterwave-Signature")
	}

	_, err := h.svc.ProcessWebhook(r.Context(), application.ProcessWebhookRequest{
		Provider:  r.PathValue("provider"),
		Payload:   payload,
		Signature: sig,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeaturePayments)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
