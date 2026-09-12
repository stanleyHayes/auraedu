package mongo

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/testkit"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func regressionStore(t *testing.T) *pmongo.Store {
	t.Helper()
	s := testkit.NewMongoReplicaSet(context.Background(), t).Store
	if err := EnsureIndexes(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func validation(t *testing.T, s *pmongo.Store, name string, rule bson.M) {
	t.Helper()
	if err := s.Database().RunCommand(context.Background(), bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: rule}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Error(err)
		return
	}
}
func TestMongoProjectionRollbackAndRollupConcurrency(t *testing.T) {
	ctx := context.Background()
	s := regressionStore(t)
	r := NewRepository(s)
	metric, err := domain.NewMetric("tenant", "visits", "2026-09-12", 1, domain.UnitCount, nil)
	if err != nil {
		t.Error(err)
		return
	}
	validation(t, s, MetricsCollection, bson.M{"_reject_test": bson.M{"$exists": true}})
	if err := r.ApplyMetricEvent(ctx, "tenant", "metric-event", "visit.created", []*domain.Metric{metric}); err == nil {
		t.Fatal("fault did not fail")
	}
	scope, err := r.processedEvents.ScopeTo("tenant")
	if err != nil {
		t.Error(err)
		return
	}
	if n, err := scope.CountDocuments(ctx, bson.M{}); err != nil || n != 0 {
		t.Fatalf("poisoned dedupe marker %d %v", n, err)
	}
	growth := domain.GrowthEvent{EventID: "growth-event", EventType: "growth.lead_created.v1", Stage: domain.GrowthLeads, LeadID: "lead", BucketDate: "2026-09-12", OccurredAt: time.Now()}
	if err := r.ApplyGrowthEvent(ctx, "tenant", growth); err == nil {
		t.Fatal("growth fault did not fail")
	}
	facts, err := r.growthEventFacts.ScopeTo("tenant")
	if err != nil {
		t.Error(err)
		return
	}
	if n, err := facts.CountDocuments(ctx, bson.M{}); err != nil || n != 0 {
		t.Fatalf("partial growth fact %d %v", n, err)
	}
	validation(t, s, MetricsCollection, bson.M{})
	for range 2 {
		if err := r.ApplyMetricEvent(ctx, "tenant", "metric-event", "visit.created", []*domain.Metric{metric}); err != nil {
			t.Error(err)
			return
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, err := domain.NewMetric("tenant", "visits", "2026-09-12", 1, domain.UnitCount, nil)
			if err != nil {
				t.Error(err)
				return
			}
			errs <- r.ApplyMetricEvent(ctx, "tenant", fmt.Sprintf("visit-%d", i), "visit.created", []*domain.Metric{m})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
			return
		}
	}
	visits, err := r.findMetric(ctx, "tenant", metric)
	if err != nil || visits.Value != 13 {
		t.Fatalf("natural-key count %+v %v", visits, err)
	}
	events := make([]domain.AssessmentScoreEvent, 8)
	errs = make(chan error, 8)
	for i := range events {
		events[i] = domain.AssessmentScoreEvent{EventID: fmt.Sprintf("score-event-%d", i), EventType: "assessment.score_recorded.v1", Operation: domain.ScoreRecorded, ScoreID: fmt.Sprintf("score-%d", i), AssessmentID: "assessment", StudentID: "student", SubjectID: "subject", AcademicYearID: "year", Score: float64(i + 1), MaxScore: 10, RecordedAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)}
		wg.Add(1)
		go func(e domain.AssessmentScoreEvent) {
			defer wg.Done()
			errs <- r.ApplyAssessmentScoreEvent(ctx, "tenant", e)
		}(events[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
			return
		}
	}
	countMetric, err := domain.NewMetric("tenant", "assessments.count", "2026-09-12", 0, domain.UnitCount, events[0].Dimensions())
	if err != nil {
		t.Error(err)
		return
	}
	count, err := r.findMetric(ctx, "tenant", countMetric)
	if err != nil || count.Value != 8 {
		t.Fatalf("rollup write skew %+v %v", count, err)
	}
	for i, e := range events {
		e.EventID = fmt.Sprintf("delete-%d", i)
		e.Operation = domain.ScoreDeleted
		if err := r.ApplyAssessmentScoreEvent(ctx, "tenant", e); err != nil {
			t.Error(err)
			return
		}
	}
	count, err = r.findMetric(ctx, "tenant", countMetric)
	if err != nil || count != nil {
		t.Fatalf("deleted scores left metric %+v %v", count, err)
	}
}
