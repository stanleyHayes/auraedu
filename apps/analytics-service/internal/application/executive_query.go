package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type ExecutiveQuery struct{ Question, From, To string }
type ExecutiveAnswer struct {
	Answer           string            `json:"answer"`
	Confidence       string            `json:"confidence"`
	SourceDatasets   []string          `json:"source_datasets"`
	From             string            `json:"from"`
	To               string            `json:"to"`
	Filters          map[string]string `json:"filters"`
	CalculationNotes []string          `json:"calculation_notes"`
	DashboardURL     string            `json:"dashboard_url"`
}

// AskExecutive answers only a small audited set of questions from the same report shown in the dashboard.
func (s *Service) AskExecutive(ctx context.Context, actor auth.Actor, query ExecutiveQuery) (ExecutiveAnswer, error) {
	question := strings.TrimSpace(query.Question)
	if len(question) < 5 || len(question) > 500 {
		return ExecutiveAnswer{}, fmt.Errorf("%w: question must contain 5 to 500 characters", domain.ErrValidation)
	}
	report, err := s.GrowthExecutive(ctx, actor, GrowthQuery{From: query.From, To: query.To})
	if err != nil {
		return ExecutiveAnswer{}, err
	}
	lower := strings.ToLower(question)
	answer := "This question is outside the currently approved Growth analytics calculations. " +
		"Use the linked dashboard or ask about the funnel, a source, a programme, or the enrolment forecast."
	confidence := "low"
	notes := []string{"No free-form model inference was used; the question did not match an approved calculation."}
	switch {
	case strings.Contains(lower, "forecast"), strings.Contains(lower, "expected enrol"), strings.Contains(lower, "next 30"):
		answer = fmt.Sprintf(
			"At the observed run-rate, %.1f accepted offers are projected over the next 30 days (%s confidence).",
			report.Forecast.ProjectedOfferAcceptances, report.Forecast.Confidence,
		)
		confidence = report.Forecast.Confidence
		notes = report.Forecast.CalculationNotes
	case strings.Contains(lower, "source"), strings.Contains(lower, "channel"):
		if len(report.BySource) > 0 {
			best := report.BySource[0]
			answer = fmt.Sprintf(
				"%s currently leads attributed sources with %d accepted offers from %d leads.",
				best.Key, best.OffersAccepted, best.Leads,
			)
			confidence = "high"
			notes = []string{"Sources are ranked by accepted offers, then applications started."}
		}
	case strings.Contains(lower, "programme"), strings.Contains(lower, "program"):
		if len(report.ByProgramme) > 0 {
			best := report.ByProgramme[0]
			answer = fmt.Sprintf(
				"Programme %s currently has %d accepted offers from %d started applications.",
				best.Key, best.OffersAccepted, best.ApplicationsStarted,
			)
			confidence = "high"
			notes = []string{
				"Programmes are ranked by accepted offers, then applications started. " +
					"Programme names are resolved in the dashboard catalogue.",
			}
		}
	case strings.Contains(lower, "funnel"), strings.Contains(lower, "conversion"), strings.Contains(lower, "application"):
		leads, accepted := 0, 0
		var rate *float64
		for _, step := range report.Funnel {
			if step.Stage == domain.GrowthLeads {
				leads = step.Count
			}
			if step.Stage == domain.GrowthOffersAccepted {
				accepted = step.Count
				rate = step.ConversionFromLead
			}
		}
		answer = fmt.Sprintf("The selected window contains %d leads and %d accepted offers.", leads, accepted)
		if rate != nil {
			answer += fmt.Sprintf(" Lead-to-accepted-offer conversion is %.1f%%.", *rate)
		}
		confidence = "high"
		notes = []string{"Conversion equals accepted offers divided by captured leads in the selected window."}
	}
	return ExecutiveAnswer{
		Answer:     answer,
		Confidence: confidence,
		SourceDatasets: []string{
			"analytics.growth_event_facts",
			"analytics.growth_lead_attribution",
			"analytics.growth_application_attribution",
		},
		From:             report.From,
		To:               report.To,
		Filters:          map[string]string{"tenant_scope": "current_tenant"},
		CalculationNotes: notes,
		DashboardURL:     "/admin/analytics?from=" + report.From + "&to=" + report.To,
	}, nil
}
