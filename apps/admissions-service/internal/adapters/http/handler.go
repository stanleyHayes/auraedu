// Package http exposes Admissions Service use cases over HTTP.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/admissions-service/internal/application"
	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type Handler struct{ svc *application.Service }

func NewHandler(s *application.Service) *Handler { return &Handler{svc: s} }
func (h *Handler) Register(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/public/programmes", h.publicProgrammes)
	m.HandleFunc("GET /api/v1/programmes", h.listProgrammes)
	m.HandleFunc("POST /api/v1/programmes", h.createProgramme)
	m.HandleFunc("PATCH /api/v1/programmes/{id}", h.updateProgramme)
	m.HandleFunc("POST /api/v1/programmes/{id}/intakes", h.createIntake)
	m.HandleFunc("PATCH /api/v1/intakes/{id}", h.updateIntake)
	m.HandleFunc("POST /api/v1/applications", h.start)
	m.HandleFunc("GET /api/v1/applications", h.list)
	m.HandleFunc("GET /api/v1/applications/{id}", h.get)
	m.HandleFunc("PATCH /api/v1/applications/{id}", h.update)
	m.HandleFunc("POST /api/v1/applications/{id}/documents", h.document)
	m.HandleFunc("POST /api/v1/applications/{id}/submit", h.submit)
	m.HandleFunc("POST /api/v1/applications/{id}/review", h.review)
	m.HandleFunc("POST /api/v1/applications/{id}/offer", h.offer)
	m.HandleFunc("POST /api/v1/applications/{id}/offer/accept", h.accept)
}

func requestLimit(r *http.Request, fallback int) (int, error) {
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return 0, domain.ErrValidation
		}
		return value, nil
	}
	return fallback, nil
}

func (h *Handler) publicProgrammes(w http.ResponseWriter, r *http.Request) {
	limit, err := requestLimit(r, 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	items, err := h.svc.PublicProgrammes(r.Context(), strings.TrimSpace(r.Header.Get("X-Tenant-Code")), limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) listProgrammes(w http.ResponseWriter, r *http.Request) {
	limit, err := requestLimit(r, 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	items, err := h.svc.ListProgrammes(r.Context(), auth.FromHeaders(r.Header), limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) createProgramme(w http.ResponseWriter, r *http.Request) {
	var body application.CreateProgrammeInput
	if !decode(w, r, &body) {
		return
	}
	programme, err := h.svc.CreateProgramme(r.Context(), auth.FromHeaders(r.Header), body)
	respond(w, http.StatusCreated, programme, err)
}

func (h *Handler) updateProgramme(w http.ResponseWriter, r *http.Request) {
	var body application.UpdateProgrammeInput
	if !decode(w, r, &body) {
		return
	}
	if body.Code == nil && body.Name == nil && body.Slug == nil && body.Summary == nil && body.Description == nil && body.Status == nil {
		writeErr(w, domain.ErrValidation)
		return
	}
	programme, err := h.svc.UpdateProgramme(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), body)
	respond(w, http.StatusOK, programme, err)
}

func (h *Handler) createIntake(w http.ResponseWriter, r *http.Request) {
	var body application.CreateIntakeInput
	if !decode(w, r, &body) {
		return
	}
	intake, err := h.svc.CreateIntake(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), body)
	respond(w, http.StatusCreated, intake, err)
}

func (h *Handler) updateIntake(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name                *string              `json:"name"`
		StartsAt            *time.Time           `json:"starts_at"`
		ApplicationOpensAt  *time.Time           `json:"application_opens_at"`
		ApplicationClosesAt *time.Time           `json:"application_closes_at"`
		Capacity            json.RawMessage      `json:"capacity"`
		Status              *domain.IntakeStatus `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	capacitySet := len(body.Capacity) > 0
	var capacity *int
	if capacitySet && string(body.Capacity) != "null" {
		var value int
		if err := json.Unmarshal(body.Capacity, &value); err != nil {
			writeErr(w, domain.ErrValidation)
			return
		}
		capacity = &value
	}
	if body.Name == nil && body.StartsAt == nil && body.ApplicationOpensAt == nil && body.ApplicationClosesAt == nil && !capacitySet && body.Status == nil {
		writeErr(w, domain.ErrValidation)
		return
	}
	intake, err := h.svc.UpdateIntake(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), application.UpdateIntakeInput{
		Name: body.Name, StartsAt: body.StartsAt, ApplicationOpensAt: body.ApplicationOpensAt, ApplicationClosesAt: body.ApplicationClosesAt,
		CapacitySet: capacitySet, Capacity: capacity, Status: body.Status,
	})
	respond(w, http.StatusOK, intake, err)
}
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	var b struct {
		LeadID      *string `json:"lead_id"`
		ProgrammeID string  `json:"programme_id"`
		IntakeID    string  `json:"intake_id"`
	}
	if !decode(w, r, &b) {
		return
	}
	a, e := h.svc.Start(r.Context(), auth.FromHeaders(r.Header), b.LeadID, b.ProgrammeID, b.IntakeID)
	respond(w, http.StatusCreated, a, e)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := requestLimit(r, 25)
	if err != nil {
		writeErr(w, err)
		return
	}
	items, e := h.svc.List(r.Context(), auth.FromHeaders(r.Header), domain.Status(r.URL.Query().Get("status")), limit)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	a, e := h.svc.Get(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, a, e)
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var b struct {
		LegalName *string        `json:"legal_name"`
		Email     *string        `json:"email"`
		Phone     *string        `json:"phone"`
		Answers   map[string]any `json:"answers"`
	}
	if !decode(w, r, &b) {
		return
	}
	a, e := h.svc.Update(
		r.Context(),
		auth.FromHeaders(r.Header),
		r.PathValue("id"),
		application.UpdateInput{
			LegalName: b.LegalName,
			Email:     b.Email,
			Phone:     b.Phone,
			Answers:   b.Answers,
		},
	)
	respond(w, http.StatusOK, a, e)
}
func (h *Handler) document(w http.ResponseWriter, r *http.Request) {
	var b struct {
		FileID       string `json:"file_id"`
		DocumentType string `json:"document_type"`
		FileName     string `json:"file_name"`
	}
	if !decode(w, r, &b) {
		return
	}
	a, e := h.svc.AttachDocument(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.FileID, b.DocumentType, b.FileName)
	respond(w, http.StatusCreated, a, e)
}
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	a, e := h.svc.Submit(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, a, e)
}
func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if !decode(w, r, &b) {
		return
	}
	a, e := h.svc.Review(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.Decision, b.Note)
	respond(w, http.StatusOK, a, e)
}
func (h *Handler) offer(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Conditions string    `json:"conditions"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	if !decode(w, r, &b) {
		return
	}
	a, e := h.svc.IssueOffer(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.Conditions, b.ExpiresAt)
	respond(w, http.StatusOK, a, e)
}
func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	a, e := h.svc.AcceptOffer(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, a, e)
}
func decode(w http.ResponseWriter, r *http.Request, d any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	x := json.NewDecoder(r.Body)
	x.DisallowUnknownFields()
	if e := x.Decode(d); e != nil {
		writeErr(w, domain.ErrValidation)
		return false
	}
	if e := x.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		writeErr(w, domain.ErrValidation)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, value any, e error) {
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, status, value)
}
func writeErr(w http.ResponseWriter, e error) {
	status, code := 500, "internal"
	switch {
	case errors.Is(e, domain.ErrValidation):
		status, code = 422, "validation_error"
	case errors.Is(e, domain.ErrForbidden):
		status, code = 403, "forbidden"
	case errors.Is(e, domain.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(e, domain.ErrConflict):
		status, code = 409, "conflict"
	case errors.Is(e, domain.ErrUnavailable):
		status, code = 503, "service_unavailable"
	}
	jsonOut(w, status, map[string]string{"code": code, "message": e.Error()})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
