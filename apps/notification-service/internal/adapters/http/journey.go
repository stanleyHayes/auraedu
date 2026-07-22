package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/httpx"
)

type createJourneyBody struct {
	Name                  string            `json:"name"`
	TriggerEvent          string            `json:"trigger_event"`
	Timezone              string            `json:"timezone"`
	QuietHoursStartMinute *int              `json:"quiet_hours_start_minute"`
	QuietHoursEndMinute   *int              `json:"quiet_hours_end_minute"`
	FrequencyWindowHours  int               `json:"frequency_window_hours"`
	FrequencyLimit        int               `json:"frequency_limit"`
	CancelOnEvents        []string          `json:"cancel_on_events"`
	Steps                 []journeyStepBody `json:"steps"`
}

type journeyStepBody struct {
	Channel           string `json:"channel"`
	TemplateID        string `json:"template_id"`
	DelayMinutes      int    `json:"delay_minutes"`
	ConditionOperator string `json:"condition_operator"`
	ConditionField    string `json:"condition_field"`
	ConditionValue    string `json:"condition_value"`
}

func (h *Handler) registerJourneys(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/communication-journeys", h.listJourneys)
	mux.HandleFunc("POST /api/v1/communication-journeys", h.createJourney)
	mux.HandleFunc("GET /api/v1/communication-journeys/{journey_id}", h.getJourney)
	mux.HandleFunc("POST /api/v1/communication-journeys/{journey_id}/activate", h.activateJourney)
	mux.HandleFunc("POST /api/v1/communication-journeys/{journey_id}/pause", h.pauseJourney)
	mux.HandleFunc("POST /api/v1/communication-journeys/{journey_id}/archive", h.archiveJourney)
	mux.HandleFunc("GET /api/v1/communication-journeys/{journey_id}/stats", h.getJourneyStats)
}

func (h *Handler) createJourney(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createJourneyBody
	if !decodeJourneyBody(w, r, &body) {
		return
	}
	steps := make([]domain.JourneyStep, 0, len(body.Steps))
	for _, step := range body.Steps {
		steps = append(steps, domain.JourneyStep{
			Channel: step.Channel, TemplateID: step.TemplateID, DelayMinutes: step.DelayMinutes,
			ConditionOperator: step.ConditionOperator, ConditionField: step.ConditionField,
			ConditionValue: step.ConditionValue,
		})
	}
	journey, err := h.svc.CreateJourney(ctx, actor, application.CreateJourneyRequest{
		Name: body.Name, TriggerEvent: body.TriggerEvent, Timezone: body.Timezone,
		QuietHoursStartMinute: body.QuietHoursStartMinute, QuietHoursEndMinute: body.QuietHoursEndMinute,
		FrequencyWindowHours: body.FrequencyWindowHours, FrequencyLimit: body.FrequencyLimit,
		CancelOnEvents: body.CancelOnEvents, Steps: steps,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, journey)
}

func (h *Handler) listJourneys(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	journeys, err := h.svc.ListJourneys(ctx, actor, ports.JourneyFilter{
		Status: r.URL.Query().Get("status"), TriggerEvent: r.URL.Query().Get("trigger_event"),
		Limit: parseLimit(r.URL.Query().Get("limit")),
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": journeys})
}

func (h *Handler) getJourney(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	journey, err := h.svc.GetJourney(ctx, actor, r.PathValue("journey_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, journey)
}

func (h *Handler) activateJourney(w http.ResponseWriter, r *http.Request) {
	h.transitionJourney(w, r, h.svc.ActivateJourney)
}

func (h *Handler) pauseJourney(w http.ResponseWriter, r *http.Request) {
	h.transitionJourney(w, r, h.svc.PauseJourney)
}

func (h *Handler) archiveJourney(w http.ResponseWriter, r *http.Request) {
	h.transitionJourney(w, r, h.svc.ArchiveJourney)
}

func (h *Handler) transitionJourney(
	w http.ResponseWriter,
	r *http.Request,
	transition func(context.Context, auth.Actor, string) (*domain.Journey, error),
) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	journey, err := transition(ctx, actor, r.PathValue("journey_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, journey)
}

func (h *Handler) getJourneyStats(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	stats, err := h.svc.GetJourneyStats(ctx, actor, r.PathValue("journey_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, stats)
}

func decodeJourneyBody(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		httpx.ValidationError(w, r, map[string]any{"body": "only one JSON object is allowed"})
		return false
	}
	return true
}
