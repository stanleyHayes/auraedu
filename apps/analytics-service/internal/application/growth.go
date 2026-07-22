package application

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

const PermExecutiveRead = "analytics.executive.read"

type GrowthQuery struct {
	From string
	To   string
}

type FunnelStep struct {
	Stage                  string   `json:"stage"`
	Count                  int      `json:"count"`
	ConversionFromPrevious *float64 `json:"conversion_from_previous"`
	ConversionFromLead     *float64 `json:"conversion_from_lead"`
}

type GrowthBreakdown struct {
	Key                    string   `json:"key"`
	Leads                  int      `json:"leads"`
	ApplicationsStarted    int      `json:"applications_started"`
	ApplicationsSubmitted  int      `json:"applications_submitted"`
	Admitted               int      `json:"admitted"`
	OffersIssued           int      `json:"offers_issued"`
	OffersAccepted         int      `json:"offers_accepted"`
	LeadToApplicationRate  *float64 `json:"lead_to_application_rate"`
	ApplicationToOfferRate *float64 `json:"application_to_offer_rate"`
}

type EnrolmentForecast struct {
	HorizonDays               int      `json:"horizon_days"`
	ProjectedOfferAcceptances float64  `json:"projected_offer_acceptances"`
	ObservedDays              int      `json:"observed_days"`
	Method                    string   `json:"method"`
	Confidence                string   `json:"confidence"`
	CalculationNotes          []string `json:"calculation_notes"`
}

type GrowthExecutive struct {
	GeneratedAt time.Time         `json:"generated_at"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Funnel      []FunnelStep      `json:"funnel"`
	BySource    []GrowthBreakdown `json:"by_source"`
	ByProgramme []GrowthBreakdown `json:"by_programme"`
	Forecast    EnrolmentForecast `json:"forecast"`
	DataQuality map[string]int    `json:"data_quality"`
}

func (s *Service) GrowthExecutive(ctx context.Context, actor auth.Actor, query GrowthQuery) (GrowthExecutive, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermExecutiveRead)
	if err != nil {
		return GrowthExecutive{}, err
	}
	from, to, err := normalizeGrowthRange(query.From, query.To, time.Now().UTC())
	if err != nil {
		return GrowthExecutive{}, err
	}
	rollups, err := s.repo.GrowthRollups(ctx, tenantID, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return GrowthExecutive{}, err
	}
	return buildGrowthExecutive(rollups, from, to, time.Now().UTC()), nil
}

func normalizeGrowthRange(fromRaw, toRaw string, now time.Time) (time.Time, time.Time, error) {
	to := now.UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -89)
	var err error
	if toRaw != "" {
		to, err = time.Parse(time.DateOnly, toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: to must be YYYY-MM-DD", domain.ErrValidation)
		}
	}
	if fromRaw != "" {
		from, err = time.Parse(time.DateOnly, fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: from must be YYYY-MM-DD", domain.ErrValidation)
		}
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: from must be on or before to", domain.ErrValidation)
	}
	if to.Sub(from) > 365*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date range cannot exceed 366 days", domain.ErrValidation)
	}
	return from, to, nil
}

func buildGrowthExecutive(rows []domain.GrowthRollup, from, to, now time.Time) GrowthExecutive {
	totals := map[string]int{}
	sources := map[string]map[string]int{}
	programmes := map[string]map[string]int{}
	unattributed := 0
	for _, row := range rows {
		value := int(math.Round(row.Value))
		totals[row.Stage] += value
		source := row.Source
		if source == "" {
			source = "unattributed"
			if row.Stage != domain.GrowthLeads {
				unattributed += value
			}
		}
		if sources[source] == nil {
			sources[source] = map[string]int{}
		}
		sources[source][row.Stage] += value
		if row.ProgrammeID != "" {
			if programmes[row.ProgrammeID] == nil {
				programmes[row.ProgrammeID] = map[string]int{}
			}
			programmes[row.ProgrammeID][row.Stage] += value
		}
	}
	stageOrder := domain.GrowthStageOrder()
	funnel := make([]FunnelStep, 0, len(stageOrder))
	previous := 0
	for i, stage := range stageOrder {
		count := totals[stage]
		step := FunnelStep{Stage: stage, Count: count}
		if i > 0 {
			step.ConversionFromPrevious = percent(count, previous)
		}
		if i > 0 {
			step.ConversionFromLead = percent(count, totals[domain.GrowthLeads])
		}
		funnel = append(funnel, step)
		previous = count
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	accepted := totals[domain.GrowthOffersAccepted]
	confidence := "low"
	if accepted >= 30 && days >= 30 {
		confidence = "high"
	} else if accepted >= 10 && days >= 14 {
		confidence = "medium"
	}
	projected := math.Round((float64(accepted)/float64(days))*30*10) / 10
	return GrowthExecutive{
		GeneratedAt: now.UTC(),
		From:        from.Format(time.DateOnly),
		To:          to.Format(time.DateOnly),
		Funnel:      funnel,
		BySource:    breakdowns(sources),
		ByProgramme: breakdowns(programmes),
		Forecast: EnrolmentForecast{
			HorizonDays:               30,
			ProjectedOfferAcceptances: projected,
			ObservedDays:              days,
			Method:                    "observed_offer_acceptance_run_rate",
			Confidence:                confidence,
			CalculationNotes: []string{
				"Projection equals accepted offers in the selected period divided by observed days, multiplied by 30.",
				"This is a transparent operating estimate, not a machine-learning prediction.",
				"Revenue is not forecast until verified programme fee data and currency are available.",
			},
		},
		DataQuality: map[string]int{"unattributed_application_events": unattributed},
	}
}

func breakdowns(input map[string]map[string]int) []GrowthBreakdown {
	out := make([]GrowthBreakdown, 0, len(input))
	for key, stages := range input {
		out = append(out, GrowthBreakdown{
			Key:                    key,
			Leads:                  stages[domain.GrowthLeads],
			ApplicationsStarted:    stages[domain.GrowthApplicationsStarted],
			ApplicationsSubmitted:  stages[domain.GrowthApplicationsDone],
			Admitted:               stages[domain.GrowthAdmitted],
			OffersIssued:           stages[domain.GrowthOffersIssued],
			OffersAccepted:         stages[domain.GrowthOffersAccepted],
			LeadToApplicationRate:  percent(stages[domain.GrowthApplicationsStarted], stages[domain.GrowthLeads]),
			ApplicationToOfferRate: percent(stages[domain.GrowthOffersAccepted], stages[domain.GrowthApplicationsStarted]),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OffersAccepted == out[j].OffersAccepted {
			return out[i].ApplicationsStarted > out[j].ApplicationsStarted
		}
		return out[i].OffersAccepted > out[j].OffersAccepted
	})
	return out
}

func percent(numerator, denominator int) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := math.Round((float64(numerator)/float64(denominator))*1000) / 10
	return &value
}
