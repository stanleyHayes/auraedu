package domain

import "time"

const (
	GrowthLeads               = "leads"
	GrowthApplicationsStarted = "applications_started"
	GrowthApplicationsDone    = "applications_submitted"
	GrowthAdmitted            = "admitted"
	GrowthOffersIssued        = "offers_issued"
	GrowthOffersAccepted      = "offers_accepted"
)

// GrowthStageOrder returns the canonical funnel order without exposing mutable global state.
func GrowthStageOrder() []string {
	return []string{
		GrowthLeads,
		GrowthApplicationsStarted,
		GrowthApplicationsDone,
		GrowthAdmitted,
		GrowthOffersIssued,
		GrowthOffersAccepted,
	}
}

// GrowthEvent is the PII-free projection input extracted from a lifecycle CloudEvent.
type GrowthEvent struct {
	EventID, EventType, Stage, BucketDate string
	LeadID, ApplicationID                 string
	ProgrammeID, IntakeID                 string
	Source, CampaignID                    string
	OccurredAt                            time.Time
}

// GrowthRollup is one aggregate row returned by the projection store.
type GrowthRollup struct {
	Stage       string  `json:"stage"`
	Source      string  `json:"source,omitempty"`
	CampaignID  string  `json:"campaign_id,omitempty"`
	ProgrammeID string  `json:"programme_id,omitempty"`
	IntakeID    string  `json:"intake_id,omitempty"`
	Value       float64 `json:"value"`
}
