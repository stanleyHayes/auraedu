// Package http provides the HTTP adapter for the billing service.
//
//nolint:misspell // British spelling "cancelled" is intentional for the billing domain.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/billing-service/internal/application"
	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the billing use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/billing/plans", h.listPlans)
	mux.HandleFunc("POST /api/v1/billing/plans", h.createPlan)
	mux.HandleFunc("GET /api/v1/billing/plans/{plan_id}", h.getPlan)
	mux.HandleFunc("PATCH /api/v1/billing/plans/{plan_id}", h.updatePlan)
	mux.HandleFunc("DELETE /api/v1/billing/plans/{plan_id}", h.deletePlan)

	mux.HandleFunc("GET /api/v1/billing/subscriptions", h.listSubscriptions)
	mux.HandleFunc("POST /api/v1/billing/subscriptions", h.createSubscription)
	mux.HandleFunc("GET /api/v1/billing/subscriptions/{subscription_id}", h.getSubscription)
	mux.HandleFunc("PATCH /api/v1/billing/subscriptions/{subscription_id}", h.updateSubscription)
	mux.HandleFunc("DELETE /api/v1/billing/subscriptions/{subscription_id}", h.deleteSubscription)
	mux.HandleFunc("POST /api/v1/billing/subscriptions/{subscription_id}/change-plan", h.changeSubscriptionPlan)

	mux.HandleFunc("GET /api/v1/billing/invoices", h.listInvoices)
	mux.HandleFunc("POST /api/v1/billing/invoices", h.createInvoice)
	mux.HandleFunc("GET /api/v1/billing/invoices/{invoice_id}", h.getInvoice)
	mux.HandleFunc("PATCH /api/v1/billing/invoices/{invoice_id}", h.updateInvoice)
	mux.HandleFunc("DELETE /api/v1/billing/invoices/{invoice_id}", h.deleteInvoice)
	mux.HandleFunc("POST /api/v1/billing/invoices/{invoice_id}/pay", h.markInvoicePaid)
	mux.HandleFunc("POST /api/v1/billing/invoices/{invoice_id}/void", h.markInvoiceVoid)
}

// --- Plan handlers ---

func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.PlanFilter{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
		Status: r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListPlans(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createPlanBody struct {
	Name            string   `json:"name"`
	Code            string   `json:"code"`
	Description     *string  `json:"description,omitempty"`
	PriceCents      int      `json:"price_cents"`
	Currency        string   `json:"currency"`
	BillingInterval string   `json:"billing_interval"`
	Features        []string `json:"features"`
}

func (h *Handler) createPlan(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createPlanBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreatePlan(ctx, actor, application.CreatePlanRequest{
		Name:            body.Name,
		Code:            body.Code,
		Description:     body.Description,
		PriceCents:      body.PriceCents,
		Currency:        body.Currency,
		BillingInterval: body.BillingInterval,
		Features:        body.Features,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getPlan(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetPlan(ctx, actor, r.PathValue("plan_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updatePlanBody struct {
	Name            *string   `json:"name,omitempty"`
	Code            *string   `json:"code,omitempty"`
	Description     *string   `json:"description,omitempty"`
	PriceCents      *int      `json:"price_cents,omitempty"`
	Currency        *string   `json:"currency,omitempty"`
	BillingInterval *string   `json:"billing_interval,omitempty"`
	Features        *[]string `json:"features,omitempty"`
	Status          *string   `json:"status,omitempty"`
}

func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updatePlanBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdatePlan(ctx, actor, r.PathValue("plan_id"), application.UpdatePlanRequest{
		Name:            body.Name,
		Code:            body.Code,
		Description:     body.Description,
		PriceCents:      body.PriceCents,
		Currency:        body.Currency,
		BillingInterval: body.BillingInterval,
		Features:        body.Features,
		Status:          body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deletePlan(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(ctx, actor, r.PathValue("plan_id")); err != nil {
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
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.SubscriptionFilter{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
		Status: r.URL.Query().Get("status"),
		PlanID: r.URL.Query().Get("plan_id"),
	}
	records, nextCursor, err := h.svc.ListSubscriptions(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createSubscriptionBody struct {
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
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
		PlanID:             body.PlanID,
		Status:             body.Status,
		CurrentPeriodStart: body.CurrentPeriodStart,
		CurrentPeriodEnd:   body.CurrentPeriodEnd,
		TrialEndsAt:        body.TrialEndsAt,
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
	Status             *string    `json:"status,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
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
		Status:             body.Status,
		CurrentPeriodStart: body.CurrentPeriodStart,
		CurrentPeriodEnd:   body.CurrentPeriodEnd,
		TrialEndsAt:        body.TrialEndsAt,
		CancelledAt:        body.CancelledAt,
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

type changePlanBody struct {
	PlanID string `json:"plan_id"`
}

func (h *Handler) changeSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body changePlanBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.ChangeSubscriptionPlan(ctx, actor, r.PathValue("subscription_id"), body.PlanID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

// --- Invoice handlers ---

func (h *Handler) listInvoices(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.SaaSInvoiceFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		Status:         r.URL.Query().Get("status"),
		SubscriptionID: r.URL.Query().Get("subscription_id"),
	}
	records, nextCursor, err := h.svc.ListInvoices(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createInvoiceBody struct {
	SubscriptionID string     `json:"subscription_id"`
	AmountCents    int        `json:"amount_cents"`
	DueDate        *time.Time `json:"due_date,omitempty"`
}

func (h *Handler) createInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createInvoiceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateInvoice(ctx, actor, application.CreateInvoiceRequest{
		SubscriptionID: body.SubscriptionID,
		AmountCents:    body.AmountCents,
		DueDate:        body.DueDate,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetInvoice(ctx, actor, r.PathValue("invoice_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateInvoiceBody struct {
	AmountCents *int       `json:"amount_cents,omitempty"`
	Status      *string    `json:"status,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

func (h *Handler) updateInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateInvoiceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateInvoice(ctx, actor, r.PathValue("invoice_id"), application.UpdateInvoiceRequest{
		AmountCents: body.AmountCents,
		Status:      body.Status,
		DueDate:     body.DueDate,
		PaidAt:      body.PaidAt,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteInvoice(ctx, actor, r.PathValue("invoice_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) markInvoicePaid(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.MarkInvoicePaid(ctx, actor, r.PathValue("invoice_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) markInvoiceVoid(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.MarkInvoiceVoid(ctx, actor, r.PathValue("invoice_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
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
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrValidation, Message: "resource conflict"})
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureBilling)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrValidation, Message: err.Error()})
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
