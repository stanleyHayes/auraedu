package domain

import (
	"testing"
	"time"
)

func TestScoreLeadExplainsEvidenceAndExcludesSensitiveInputs(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	programme, intake, campaign := "programme", "intake", "campaign"
	lead := Lead{PreferredProgrammeIDs: []string{programme}, PreferredIntakeID: &intake, CampaignID: &campaign, Stage: StageApplicationStarted, CreatedAt: now.Add(-20 * 24 * time.Hour)}
	last := now.Add(-2 * time.Hour)
	score := ScoreLead(lead, ScoringEvidence{InboundProspectInteractions: 3, LastInboundAt: &last}, now)
	if score.Score != 100 || score.Confidence != "high" {
		t.Fatalf("unexpected score: %+v", score)
	}
	if score.RuleVersion == "" || len(score.PositiveFactors) == 0 {
		t.Fatalf("missing explanation: %+v", score)
	}
}

func TestScoreLeadShowsActionableMissingEvidence(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	score := ScoreLead(Lead{Stage: StageNew, CreatedAt: now.Add(-10 * 24 * time.Hour)}, ScoringEvidence{}, now)
	if score.Score != 0 || score.Confidence != "low" {
		t.Fatalf("unexpected sparse score: %+v", score)
	}
	codes := map[string]bool{}
	for _, factor := range score.NegativeFactors {
		codes[factor.Code] = true
	}
	for _, code := range []string{"programme_unknown", "intake_unknown", "no_recent_response"} {
		if !codes[code] {
			t.Fatalf("missing factor %s: %+v", code, score.NegativeFactors)
		}
	}
}
