package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the fees use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/fee-structures", h.listFeeStructures)
	mux.HandleFunc("POST /api/v1/fee-structures", h.createFeeStructure)
	mux.HandleFunc("GET /api/v1/fee-structures/{fee_structure_id}", h.getFeeStructure)
	mux.HandleFunc("PATCH /api/v1/fee-structures/{fee_structure_id}", h.updateFeeStructure)
	mux.HandleFunc("DELETE /api/v1/fee-structures/{fee_structure_id}", h.deleteFeeStructure)

	mux.HandleFunc("GET /api/v1/invoices", h.listInvoices)
	mux.HandleFunc("POST /api/v1/invoices", h.createInvoice)
	mux.HandleFunc("GET /api/v1/invoices/{invoice_id}", h.getInvoice)
	mux.HandleFunc("PATCH /api/v1/invoices/{invoice_id}", h.updateInvoice)
	mux.HandleFunc("DELETE /api/v1/invoices/{invoice_id}", h.deleteInvoice)
}

// --- FeeStructure handlers ---

func (h *Handler) listFeeStructures(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.FeeStructureFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListFeeStructures(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createFeeStructureBody struct {
	Name           string  `json:"name"`
	AcademicYearID string  `json:"academic_year_id"`
	AmountCents    int     `json:"amount_cents"`
	Currency       string  `json:"currency"`
	Recurrence     string  `json:"recurrence"`
	Target         string  `json:"target"`
	DueDay         *int    `json:"due_day"`
	Description    *string `json:"description"`
}

func (h *Handler) createFeeStructure(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createFeeStructureBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateFeeStructure(ctx, actor, application.CreateFeeStructureRequest{
		Name:           body.Name,
		AcademicYearID: body.AcademicYearID,
		AmountCents:    body.AmountCents,
		Currency:       body.Currency,
		Recurrence:     body.Recurrence,
		Target:         body.Target,
		DueDay:         body.DueDay,
		Description:    body.Description,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getFeeStructure(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetFeeStructure(ctx, actor, r.PathValue("fee_structure_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateFeeStructureBody struct {
	Name           *string `json:"name"`
	AcademicYearID *string `json:"academic_year_id"`
	AmountCents    *int    `json:"amount_cents"`
	Currency       *string `json:"currency"`
	Recurrence     *string `json:"recurrence"`
	Target         *string `json:"target"`
	DueDay         *int    `json:"due_day"`
	Description    *string `json:"description"`
	Status         *string `json:"status"`
}

func (h *Handler) updateFeeStructure(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateFeeStructureBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateFeeStructure(ctx, actor, r.PathValue("fee_structure_id"), application.UpdateFeeStructureRequest{
		Name:           body.Name,
		AcademicYearID: body.AcademicYearID,
		AmountCents:    body.AmountCents,
		Currency:       body.Currency,
		Recurrence:     body.Recurrence,
		Target:         body.Target,
		DueDay:         body.DueDay,
		Description:    body.Description,
		Status:         body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteFeeStructure(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteFeeStructure(ctx, actor, r.PathValue("fee_structure_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Invoice handlers ---

func (h *Handler) listInvoices(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.InvoiceFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		StudentID:      r.URL.Query().Get("student_id"),
		FeeStructureID: r.URL.Query().Get("fee_structure_id"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListInvoices(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createInvoiceBody struct {
	StudentID      string  `json:"student_id"`
	FeeStructureID string  `json:"fee_structure_id"`
	AmountCents    int     `json:"amount_cents"`
	BalanceCents   *int    `json:"balance_cents"`
	DueDate        string  `json:"due_date"`
	Notes          *string `json:"notes"`
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
		StudentID:      body.StudentID,
		FeeStructureID: body.FeeStructureID,
		AmountCents:    body.AmountCents,
		BalanceCents:   body.BalanceCents,
		DueDate:        body.DueDate,
		Notes:          body.Notes,
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
	AmountCents  *int    `json:"amount_cents"`
	BalanceCents *int    `json:"balance_cents"`
	Status       *string `json:"status"`
	DueDate      *string `json:"due_date"`
	Notes        *string `json:"notes"`
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
		AmountCents:  body.AmountCents,
		BalanceCents: body.BalanceCents,
		Status:       body.Status,
		DueDate:      body.DueDate,
		Notes:        body.Notes,
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
		httpx.FeatureDisabled(w, r, application.FeatureFees)
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
