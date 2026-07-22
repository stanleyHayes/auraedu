//nolint:lll // Rule explanations stay beside their point values for policy review.
package domain

import (
	"sort"
	"time"
)

const LeadScoreRuleVersion = "growth-rules-2026-01"

type ScoreFactor struct {
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	Points      int    `json:"points"`
}

type LeadScore struct {
	Score           int           `json:"score"`
	Confidence      string        `json:"confidence"`
	PositiveFactors []ScoreFactor `json:"positive_factors"`
	NegativeFactors []ScoreFactor `json:"negative_factors"`
	RuleVersion     string        `json:"rule_version"`
	EvaluatedAt     time.Time     `json:"evaluated_at"`
}

type ScoringEvidence struct {
	InboundProspectInteractions int
	LastInboundAt               *time.Time
}

// ScoreLead is intentionally rules-based and accepts no demographic or protected attributes.
//
//nolint:gocyclo // Each branch is an independently auditable scoring rule.
func ScoreLead(lead Lead, evidence ScoringEvidence, now time.Time) LeadScore {
	positive := []ScoreFactor{{Code: "valid_enquiry", Explanation: "A consented enquiry with a usable contact method was captured.", Points: 20}}
	negative := []ScoreFactor{}
	if len(lead.PreferredProgrammeIDs) > 0 {
		positive = append(positive, ScoreFactor{Code: "programme_interest", Explanation: "The prospect selected at least one programme.", Points: 15})
	} else {
		negative = append(negative, ScoreFactor{Code: "programme_unknown", Explanation: "No preferred programme has been selected yet.", Points: -10})
	}
	if lead.PreferredIntakeID != nil {
		positive = append(positive, ScoreFactor{Code: "intake_selected", Explanation: "A preferred intake is known.", Points: 10})
	} else {
		negative = append(negative, ScoreFactor{Code: "intake_unknown", Explanation: "No preferred intake has been selected yet.", Points: -5})
	}
	if lead.CampaignID != nil {
		positive = append(positive, ScoreFactor{Code: "campaign_attributed", Explanation: "The enquiry has traceable campaign attribution.", Points: 5})
	}
	if evidence.InboundProspectInteractions > 0 {
		positive = append(positive, ScoreFactor{Code: "prospect_response", Explanation: "The prospect has responded through a tracked interaction.", Points: 10})
	}
	if evidence.InboundProspectInteractions >= 3 {
		positive = append(positive, ScoreFactor{Code: "repeat_engagement", Explanation: "The prospect has engaged at least three times.", Points: 10})
	}
	if evidence.LastInboundAt != nil && now.Sub(*evidence.LastInboundAt) <= 14*24*time.Hour {
		positive = append(positive, ScoreFactor{Code: "recent_engagement", Explanation: "The latest prospect response was within 14 days.", Points: 10})
	} else if now.Sub(lead.CreatedAt) >= 7*24*time.Hour {
		negative = append(negative, ScoreFactor{Code: "no_recent_response", Explanation: "No recent prospect response is recorded.", Points: -10})
	}
	stagePoints := map[LeadStage]int{StageContacted: 5, StageEngaged: 15, StageQualified: 25, StageApplicationStarted: 35, StageApplicationCompleted: 50, StageUnderReview: 55, StageAdmitted: 70, StageOfferAccepted: 85, StageDepositPaid: 90, StageEnrolled: 100}
	if points := stagePoints[lead.Stage]; points > 0 {
		positive = append(positive, ScoreFactor{Code: "lifecycle_progress", Explanation: "The verified admissions lifecycle has advanced to " + string(lead.Stage) + ".", Points: points})
	}
	if lead.Stage == StageLost || lead.Stage == StageWithdrawn || lead.Stage == StageDeferred {
		points := map[LeadStage]int{StageLost: -30, StageWithdrawn: -25, StageDeferred: -10}[lead.Stage]
		negative = append(negative, ScoreFactor{Code: "inactive_lifecycle", Explanation: "The current lifecycle state reduces immediate follow-up priority.", Points: points})
	}
	score := 0
	for _, factor := range positive {
		score += factor.Points
	}
	for _, factor := range negative {
		score += factor.Points
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	confidence := "low"
	if evidence.InboundProspectInteractions > 0 || lead.Stage != StageNew {
		confidence = "medium"
	}
	if evidence.InboundProspectInteractions >= 3 || stagePoints[lead.Stage] >= 35 {
		confidence = "high"
	}
	sort.Slice(positive, func(i, j int) bool { return positive[i].Points > positive[j].Points })
	sort.Slice(negative, func(i, j int) bool { return negative[i].Points < negative[j].Points })
	if len(positive) > 5 {
		positive = positive[:5]
	}
	if len(negative) > 5 {
		negative = negative[:5]
	}
	return LeadScore{Score: score, Confidence: confidence, PositiveFactors: positive, NegativeFactors: negative, RuleVersion: LeadScoreRuleVersion, EvaluatedAt: now.UTC()}
}
