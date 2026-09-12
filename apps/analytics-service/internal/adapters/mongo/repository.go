// Package mongo provides the MongoDB implementation of the analytics-service repository.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Tenant isolation comes from platform/mongo.Scope rather than row-level
// security: a query that is not scoped cannot be written.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	MetricsCollection                      = "metrics"
	ProcessedEventsCollection              = "analytics_processed_events"
	AssessmentScoreFactsCollection         = "assessment_score_facts"
	GrowthLeadAttributionCollection        = "growth_lead_attribution"
	GrowthApplicationAttributionCollection = "growth_application_attribution"
	GrowthEventFactsCollection             = "growth_event_facts"
)

type Repository struct {
	store                 *pmongo.Store
	metrics               *pmongo.Collection
	processedEvents       *pmongo.Collection
	assessmentScoreFacts  *pmongo.Collection
	growthLeadAttribution *pmongo.Collection
	growthAppAttribution  *pmongo.Collection
	growthEventFacts      *pmongo.Collection
}

var _ ports.Repository = (*Repository)(nil)

func NewRepository(store *pmongo.Store) *Repository {
	return &Repository{store: store,
		metrics:               store.Collection(MetricsCollection),
		processedEvents:       store.Collection(ProcessedEventsCollection),
		assessmentScoreFacts:  store.Collection(AssessmentScoreFactsCollection),
		growthLeadAttribution: store.Collection(GrowthLeadAttributionCollection),
		growthAppAttribution:  store.Collection(GrowthApplicationAttributionCollection),
		growthEventFacts:      store.Collection(GrowthEventFactsCollection),
	}
}

// metricDoc mirrors the metrics table so both adapters describe the same record.
type metricDoc struct {
	ID          string            `bson:"_id"`
	TenantID    string            `bson:"tenant_id"`
	MetricName  string            `bson:"metric_name"`
	BucketDate  string            `bson:"bucket_date"`
	Value       float64           `bson:"value"`
	Unit        string            `bson:"unit"`
	Dimensions  map[string]string `bson:"dimensions,omitempty"`
	SampleCount *int64            `bson:"sample_count,omitempty"`
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
}

func (d metricDoc) toDomain() (*domain.Metric, error) {
	m := &domain.Metric{
		ID:          d.ID,
		TenantID:    d.TenantID,
		MetricName:  d.MetricName,
		Value:       d.Value,
		Unit:        domain.Unit(d.Unit),
		Dimensions:  domain.Dimensions(d.Dimensions),
		SampleCount: d.SampleCount,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	date, err := domain.NewDate(d.BucketDate)
	if err != nil {
		return nil, fmt.Errorf("analytics: parse bucket date: %w", err)
	}
	m.BucketDate = date
	return m, nil
}

func metricFields(m *domain.Metric) bson.M {
	fields := bson.M{
		"metric_name": m.MetricName,
		"bucket_date": m.BucketDate.String(),
		"value":       m.Value,
		"unit":        string(m.Unit),
		"created_at":  m.CreatedAt,
		"updated_at":  m.UpdatedAt,
	}
	if len(m.Dimensions) > 0 {
		dimMap := bson.M{}
		for k, v := range m.Dimensions {
			dimMap[k] = v
		}
		fields["dimensions"] = dimMap
	}
	if m.SampleCount != nil {
		fields["sample_count"] = *m.SampleCount
	}
	return fields
}

// UpsertMetric matches PostgreSQL's natural-key aggregation, preserving ID and
// creation time while adding counts/sums or weighting average samples.
func (r *Repository) UpsertMetric(ctx context.Context, tenantID string, m *domain.Metric) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.serializeTenant(ctx, tenantID); err != nil {
			return err
		}
		previous, err := r.findMetric(ctx, tenantID, m)
		if err != nil {
			return err
		}
		next := *m
		if previous != nil {
			next.ID = previous.ID
			next.CreatedAt = previous.CreatedAt
			switch m.Unit {
			case domain.UnitPercentage:
				// Percentage samples replace the previous value.
			case domain.UnitCount, domain.UnitSum:
				next.Value = previous.Value + m.Value
			case domain.UnitAverage:
				old := int64(0)
				if previous.SampleCount != nil {
					old = *previous.SampleCount
				}
				n := *m.SampleCount
				if old > 0 {
					next.Value = (previous.Value*float64(old) + m.Value*float64(n)) / float64(old+n)
				}
				n += old
				next.SampleCount = &n
			}
		}
		return r.writeMetric(ctx, tenantID, &next, previous != nil)
	})
}
func dimensionKey(d domain.Dimensions) string {
	if d == nil {
		d = domain.Dimensions{}
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		panic(fmt.Sprintf("encode string dimensions: %v", err))
	}
	return string(encoded)
}
func (r *Repository) findMetric(ctx context.Context, tenantID string, m *domain.Metric) (*metricDoc, error) {
	scope, err := r.metrics.ScopeTo(tenantID)
	if err != nil {
		return nil, err
	}
	cur, err := scope.Find(ctx, bson.M{"metric_name": m.MetricName, "bucket_date": m.BucketDate.String()})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cur.Close(ctx); err != nil {
			slog.Warn("close Mongo cursor", "error", err)
		}
	}()
	for cur.Next(ctx) {
		var doc metricDoc
		if err = cur.Decode(&doc); err != nil {
			return nil, err
		}
		if dimensionKey(domain.Dimensions(doc.Dimensions)) == dimensionKey(m.Dimensions) {
			return &doc, nil
		}
	}
	return nil, cur.Err()
}
func (r *Repository) writeMetric(ctx context.Context, tenantID string, m *domain.Metric, exists bool) error {
	scope, err := r.metrics.ScopeTo(tenantID)
	if err != nil {
		return err
	}
	fields := metricFields(m)
	fields["dimensions_key"] = dimensionKey(m.Dimensions)
	if exists {
		_, err = scope.UpdateOne(ctx, bson.M{"_id": m.ID}, bson.M{"$set": fields})
		return err
	}
	fields["_id"] = m.ID
	_, err = scope.InsertOne(ctx, fields)
	return err
}

// ApplyMetricEvent atomically deduplicates one CloudEvent and applies every
// metric derived from it.
func (r *Repository) ApplyMetricEvent(ctx context.Context, tenantID, eventID, eventType string, metrics []*domain.Metric) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.serializeTenant(ctx, tenantID); err != nil {
			return err
		}

		if eventID == "" || eventType == "" {
			return fmt.Errorf("analytics: event id and type are required")
		}
		for _, metric := range metrics {
			if metric == nil {
				return fmt.Errorf("analytics: metric event contains a nil metric")
			}
			if err := metric.Validate(); err != nil {
				return err
			}
		}

		scope, err := r.processedEvents.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: processed events scope: %w", err)
		}

		// Check if already processed
		res, err := scope.UpsertOne(ctx,
			bson.M{"_id": tenantID + "/" + eventID},
			bson.M{"$setOnInsert": bson.M{"event_type": eventType, "processed_at": time.Now().UTC()}},
		)
		if err != nil {
			return fmt.Errorf("analytics: record processed metric event: %w", err)
		}

		// Only a genuine insert claims the event; anything else is a replay.
		if res.UpsertedID == nil {
			return nil
		}

		// Apply each metric
		for _, metric := range metrics {
			if err := r.UpsertMetric(ctx, tenantID, metric); err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyAssessmentScoreEvent atomically deduplicates one lifecycle event,
// mutates the current score fact, and recomputes every affected aggregate.
func (r *Repository) ApplyAssessmentScoreEvent(ctx context.Context, tenantID string, event domain.AssessmentScoreEvent) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.serializeTenant(ctx, tenantID); err != nil {
			return err
		}

		if err := event.Validate(); err != nil {
			return err
		}

		scope, err := r.processedEvents.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: processed events scope: %w", err)
		}

		// Check if already processed
		res, err := scope.UpsertOne(ctx,
			bson.M{"_id": tenantID + "/" + event.EventID},
			bson.M{"$setOnInsert": bson.M{"event_type": event.EventType, "processed_at": time.Now().UTC()}},
		)
		if err != nil {
			return fmt.Errorf("analytics: record processed score event: %w", err)
		}

		// Only a genuine insert claims the event; anything else is a replay.
		if res.UpsertedID == nil {
			return nil
		}

		// Load previous fact for rollup key
		previous, err := r.loadScoreFact(ctx, tenantID, event.ScoreID)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}

		scoreScope, err := r.assessmentScoreFacts.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: score facts scope: %w", err)
		}

		// Handle delete or upsert
		if event.Operation == domain.ScoreDeleted {
			_, err := scoreScope.DeleteOne(ctx, bson.M{"_id": event.ScoreID})
			if err != nil {
				return fmt.Errorf("analytics: delete score fact: %w", err)
			}
		} else {
			occurredAt := event.OccurredAt
			if occurredAt.IsZero() {
				occurredAt = time.Now().UTC()
			}
			scoreDoc := bson.M{
				"_id":              event.ScoreID,
				"assessment_id":    event.AssessmentID,
				"student_id":       event.StudentID,
				"subject_id":       event.SubjectID,
				"academic_year_id": event.AcademicYearID,
				"bucket_date":      event.BucketDate(),
				"score":            event.Score,
				"max_score":        event.MaxScore,
				"recorded_at":      event.RecordedAt,
				"updated_at":       occurredAt,
			}
			_, err := scoreScope.UpsertOne(ctx,
				bson.M{"_id": event.ScoreID},
				bson.M{"$set": scoreDoc},
			)
			if err != nil {
				return fmt.Errorf("analytics: upsert score fact: %w", err)
			}
		}

		// Recompute affected rollups
		keys := []scoreRollupKey{}
		if previous != nil {
			keys = append(keys, previous.key())
		}
		if event.Operation != domain.ScoreDeleted {
			keys = appendUniqueScoreKey(keys, scoreRollupKey{
				BucketDate:     event.BucketDate(),
				StudentID:      event.StudentID,
				SubjectID:      event.SubjectID,
				AcademicYearID: event.AcademicYearID,
			})
		}

		for _, key := range keys {
			if err := r.recomputeScoreRollup(ctx, tenantID, key); err != nil {
				return err
			}
		}
		return nil
	})
}

type scoreFact struct {
	BucketDate     string
	StudentID      string
	SubjectID      string
	AcademicYearID string
}

func (fact scoreFact) key() scoreRollupKey {
	return scoreRollupKey(fact)
}

type scoreRollupKey struct {
	BucketDate     string
	StudentID      string
	SubjectID      string
	AcademicYearID string
}

func (r *Repository) loadScoreFact(ctx context.Context, tenantID, scoreID string) (*scoreFact, error) {
	scope, err := r.assessmentScoreFacts.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("analytics: score facts scope: %w", err)
	}

	var doc struct {
		BucketDate     string `bson:"bucket_date"`
		StudentID      string `bson:"student_id"`
		SubjectID      string `bson:"subject_id"`
		AcademicYearID string `bson:"academic_year_id"`
	}
	err = scope.FindOne(ctx, bson.M{"_id": scoreID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongo.ErrNoDocuments
		}
		return nil, fmt.Errorf("analytics: load score fact: %w", err)
	}
	return &scoreFact{
		BucketDate:     doc.BucketDate,
		StudentID:      doc.StudentID,
		SubjectID:      doc.SubjectID,
		AcademicYearID: doc.AcademicYearID,
	}, nil
}

func appendUniqueScoreKey(keys []scoreRollupKey, candidate scoreRollupKey) []scoreRollupKey {
	for _, key := range keys {
		if key == candidate {
			return keys
		}
	}
	return append(keys, candidate)
}

func (r *Repository) recomputeScoreRollup(ctx context.Context, tenantID string, key scoreRollupKey) error {
	scope, err := r.assessmentScoreFacts.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("analytics: score facts scope: %w", err)
	}

	// Aggregate the scores
	cursor, err := scope.Find(ctx, bson.M{
		"bucket_date":      key.BucketDate,
		"student_id":       key.StudentID,
		"subject_id":       key.SubjectID,
		"academic_year_id": key.AcademicYearID,
	})
	if err != nil {
		return fmt.Errorf("analytics: find score facts: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var count int64
	var sum, sumPercentage float64
	for cursor.Next(ctx) {
		var doc struct {
			Score    float64 `bson:"score"`
			MaxScore float64 `bson:"max_score"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("analytics: decode score fact: %w", err)
		}
		count++
		sum += doc.Score
		if doc.MaxScore > 0 {
			sumPercentage += (doc.Score / doc.MaxScore) * 100
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("analytics: aggregate score facts: %w", err)
	}

	metricsScope, err := r.metrics.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("analytics: metrics scope: %w", err)
	}

	// If no scores, delete the metrics
	if count == 0 {
		dimensionsJSON := dimensionKey(domain.Dimensions{"student_id": key.StudentID, "subject_id": key.SubjectID, "academic_year_id": key.AcademicYearID})
		metricNames := []string{"assessments.count", "assessments.sum_score", "assessments.avg_score", "assessments.avg_percentage"}
		_, err := metricsScope.DeleteMany(ctx, bson.M{
			"bucket_date":    key.BucketDate,
			"dimensions_key": dimensionsJSON,
			"metric_name":    bson.M{"$in": metricNames},
		})
		return err
	}

	// Recompute and upsert metrics
	dimensions := domain.Dimensions{
		"student_id":       key.StudentID,
		"subject_id":       key.SubjectID,
		"academic_year_id": key.AcademicYearID,
	}

	var average, percentage float64
	if count > 0 {
		average = sum / float64(count)
		percentage = sumPercentage / float64(count)
	}

	metrics := []struct {
		name        string
		value       float64
		unit        domain.Unit
		sampleCount *int64
	}{
		{name: "assessments.count", value: float64(count), unit: domain.UnitCount},
		{name: "assessments.sum_score", value: sum, unit: domain.UnitSum},
		{name: "assessments.avg_score", value: average, unit: domain.UnitAverage, sampleCount: &count},
		{name: "assessments.avg_percentage", value: percentage, unit: domain.UnitAverage, sampleCount: &count},
	}

	for _, spec := range metrics {
		metric, err := domain.NewMetric(tenantID, spec.name, key.BucketDate, spec.value, spec.unit, dimensions)
		if err != nil {
			return err
		}
		metric.SampleCount = spec.sampleCount
		if err := r.replaceMetric(ctx, tenantID, metric); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) replaceMetric(ctx context.Context, tenantID string, m *domain.Metric) error {
	previous, err := r.findMetric(ctx, tenantID, m)
	if err != nil {
		return err
	}
	next := *m
	if previous != nil {
		next.ID = previous.ID
		next.CreatedAt = previous.CreatedAt
	}
	return r.writeMetric(ctx, tenantID, &next, previous != nil)
}

func (r *Repository) ListMetrics(ctx context.Context, tenantID string, filter ports.ListFilter) ([]*domain.Metric, string, error) {
	scope, err := r.metrics.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("analytics: list metrics scope: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := bson.M{}
	if filter.MetricName != "" {
		query["metric_name"] = filter.MetricName
	}
	if filter.BucketDateFrom != "" {
		if _, ok := query["bucket_date"]; !ok {
			query["bucket_date"] = bson.M{}
		}
		query["bucket_date"].(bson.M)["$gte"] = filter.BucketDateFrom
	}
	if filter.BucketDateTo != "" {
		if _, ok := query["bucket_date"]; !ok {
			query["bucket_date"] = bson.M{}
		}
		query["bucket_date"].(bson.M)["$lte"] = filter.BucketDateTo
	}
	if filter.DimensionKey != "" && filter.DimensionValue != "" {
		query["dimensions."+filter.DimensionKey] = filter.DimensionValue
	}
	if filter.StudentIDs != nil && len(filter.StudentIDs) > 0 {
		query["dimensions.student_id"] = bson.M{"$in": filter.StudentIDs}
	}

	if filter.Cursor != "" {
		// Cursor points to the last record's ID; find its created_at and id
		cursorQuery := bson.M{"_id": filter.Cursor}
		var cursorDoc struct {
			CreatedAt time.Time `bson:"created_at"`
		}
		if err := scope.FindOne(ctx, cursorQuery).Decode(&cursorDoc); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", fmt.Errorf("analytics: find cursor: %w", err)
		}
		if err == nil {
			query["$or"] = bson.A{
				bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
				bson.M{"created_at": bson.M{"$eq": cursorDoc.CreatedAt}, "_id": bson.M{"$gt": filter.Cursor}},
			}
		}
	}

	cur, err := scope.Find(ctx, query,
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}).
			SetLimit(int64(limit+1)), // Fetch one extra to detect if there's a next page
	)
	if err != nil {
		return nil, "", fmt.Errorf("analytics: list metrics: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Metric
	for cur.Next(ctx) && len(out) <= limit {
		var doc metricDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("analytics: decode metric: %w", err)
		}
		m, err := doc.toDomain()
		if err != nil {
			return nil, "", err
		}
		out = append(out, m)
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("analytics: list rows: %w", err)
	}

	var nextCursor string
	if len(out) > limit {
		// There's a next page
		out = out[:limit]
		nextCursor = out[len(out)-1].ID
	}

	return out, nextCursor, nil
}

// ApplyGrowthEvent atomically deduplicates, attributes and counts one Growth event.
func (r *Repository) ApplyGrowthEvent(ctx context.Context, tenantID string, event domain.GrowthEvent) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.serializeTenant(ctx, tenantID); err != nil {
			return err
		}

		scope, err := r.processedEvents.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: processed events scope: %w", err)
		}

		// Check if already processed
		res, err := scope.UpsertOne(ctx,
			bson.M{"_id": tenantID + "/" + event.EventID},
			bson.M{"$setOnInsert": bson.M{"event_type": event.EventType, "processed_at": time.Now().UTC()}},
		)
		if err != nil {
			return fmt.Errorf("analytics: record processed growth event: %w", err)
		}

		// Only a genuine insert claims the event; anything else is a replay.
		if res.UpsertedID == nil {
			return nil
		}

		// Store growth attribution
		if err := r.storeGrowthAttribution(ctx, tenantID, event); err != nil {
			return err
		}

		// Store growth fact
		if err := r.storeGrowthFact(ctx, tenantID, event); err != nil {
			return err
		}

		// Hydrate growth attribution
		if err := r.hydrateGrowthAttribution(ctx, tenantID, &event); err != nil {
			return err
		}

		// Create growth metric
		metric, err := domain.NewMetric(
			tenantID, "growth.funnel."+event.Stage, event.BucketDate, 1, domain.UnitCount,
			r.growthDimensions(event),
		)
		if err != nil {
			return err
		}
		if err := r.UpsertMetric(ctx, tenantID, metric); err != nil {
			return err
		}

		// Preserve the EP-56 metric key used by existing smoke checks and clients
		if event.Stage == domain.GrowthLeads {
			legacy, err := domain.NewMetric(tenantID, "growth.leads.count", event.BucketDate, 1, domain.UnitCount, nil)
			if err != nil {
				return err
			}
			return r.UpsertMetric(ctx, tenantID, legacy)
		}
		return nil
	})
}

func (r *Repository) storeGrowthAttribution(ctx context.Context, tenantID string, event domain.GrowthEvent) error {
	if event.Stage == domain.GrowthLeads {
		scope, err := r.growthLeadAttribution.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: growth lead attribution scope: %w", err)
		}

		leadDoc := bson.M{
			"_id":        event.LeadID,
			"source":     event.Source,
			"created_at": event.OccurredAt,
		}
		if event.CampaignID != "" {
			leadDoc["campaign_id"] = event.CampaignID
		}

		_, err = scope.UpsertOne(ctx,
			bson.M{"_id": event.LeadID},
			bson.M{"$set": leadDoc},
		)
		if err != nil {
			return fmt.Errorf("analytics: store lead attribution: %w", err)
		}
	}

	if event.ApplicationID == "" {
		return nil
	}

	scope, err := r.growthAppAttribution.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("analytics: growth app attribution scope: %w", err)
	}

	appDoc := bson.M{
		"_id":          event.ApplicationID,
		"programme_id": event.ProgrammeID,
		"started_at":   event.OccurredAt,
	}
	if event.LeadID != "" {
		appDoc["lead_id"] = event.LeadID
	}
	if event.IntakeID != "" {
		appDoc["intake_id"] = event.IntakeID
	}

	_, err = scope.UpsertOne(ctx,
		bson.M{"_id": event.ApplicationID},
		bson.M{"$set": appDoc},
	)
	if err != nil {
		return fmt.Errorf("analytics: store application attribution: %w", err)
	}
	return nil
}

func (r *Repository) storeGrowthFact(ctx context.Context, tenantID string, event domain.GrowthEvent) error {
	scope, err := r.growthEventFacts.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("analytics: growth event facts scope: %w", err)
	}

	factDoc := bson.M{
		"_id":         event.EventID,
		"event_type":  event.EventType,
		"stage":       event.Stage,
		"bucket_date": event.BucketDate,
		"occurred_at": event.OccurredAt,
	}
	if event.LeadID != "" {
		factDoc["lead_id"] = event.LeadID
	}
	if event.ApplicationID != "" {
		factDoc["application_id"] = event.ApplicationID
	}
	if event.ProgrammeID != "" {
		factDoc["programme_id"] = event.ProgrammeID
	}
	if event.IntakeID != "" {
		factDoc["intake_id"] = event.IntakeID
	}
	if event.Source != "" {
		factDoc["source"] = event.Source
	}
	if event.CampaignID != "" {
		factDoc["campaign_id"] = event.CampaignID
	}

	_, err = scope.InsertOne(ctx, factDoc)
	if err != nil {
		return fmt.Errorf("analytics: store growth event fact: %w", err)
	}
	return nil
}

func (r *Repository) hydrateGrowthAttribution(ctx context.Context, tenantID string, event *domain.GrowthEvent) error {
	if event.Source == "" && event.LeadID != "" {
		scope, err := r.growthLeadAttribution.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: growth lead attribution scope: %w", err)
		}

		var doc struct {
			Source     string `bson:"source"`
			CampaignID string `bson:"campaign_id"`
		}
		err = scope.FindOne(ctx, bson.M{"_id": event.LeadID}).Decode(&doc)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("analytics: load lead attribution: %w", err)
		}
		if err == nil {
			event.Source = doc.Source
			event.CampaignID = doc.CampaignID
		}
	}

	if event.Source == "" && event.ApplicationID != "" {
		scope, err := r.growthAppAttribution.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("analytics: growth app attribution scope: %w", err)
		}

		var appDoc struct {
			LeadID string `bson:"lead_id"`
		}
		err = scope.FindOne(ctx, bson.M{"_id": event.ApplicationID}).Decode(&appDoc)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("analytics: load application attribution: %w", err)
		}

		if err == nil && appDoc.LeadID != "" {
			leadScope, err := r.growthLeadAttribution.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("analytics: growth lead attribution scope: %w", err)
			}

			var leadDoc struct {
				Source     string `bson:"source"`
				CampaignID string `bson:"campaign_id"`
			}
			err = leadScope.FindOne(ctx, bson.M{"_id": appDoc.LeadID}).Decode(&leadDoc)
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				return fmt.Errorf("analytics: load lead attribution for app: %w", err)
			}
			if err == nil {
				event.Source = leadDoc.Source
				event.CampaignID = leadDoc.CampaignID
			}
		}
	}
	return nil
}

func (r *Repository) growthDimensions(event domain.GrowthEvent) domain.Dimensions {
	dimensions := domain.Dimensions{}
	if event.Source != "" {
		dimensions["source"] = event.Source
	}
	if event.CampaignID != "" {
		dimensions["campaign_id"] = event.CampaignID
	}
	if event.ProgrammeID != "" {
		dimensions["programme_id"] = event.ProgrammeID
	}
	if event.IntakeID != "" {
		dimensions["intake_id"] = event.IntakeID
	}
	return dimensions
}

// GrowthRollups returns aggregated growth funnel data.
func (r *Repository) GrowthRollups(ctx context.Context, tenantID, fromDate, toDate string) ([]domain.GrowthRollup, error) {
	scope, err := r.growthEventFacts.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("analytics: growth event facts scope: %w", err)
	}

	cur, err := scope.Find(ctx, bson.M{
		"bucket_date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: growth rollups find: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	// Collect facts
	type factKey struct {
		stage       string
		source      string
		campaignID  string
		programmeID string
		intakeID    string
	}
	factMap := make(map[factKey]int64)

	for cur.Next(ctx) {
		var fact struct {
			Stage       string `bson:"stage"`
			LeadID      string `bson:"lead_id"`
			AppID       string `bson:"application_id"`
			Source      string `bson:"source"`
			CampaignID  string `bson:"campaign_id"`
			ProgrammeID string `bson:"programme_id"`
			IntakeID    string `bson:"intake_id"`
		}
		if err := cur.Decode(&fact); err != nil {
			return nil, fmt.Errorf("analytics: decode fact: %w", err)
		}

		// Try to hydrate from attribution tables if not in fact
		source := fact.Source
		campaignID := fact.CampaignID

		if source == "" && fact.LeadID != "" {
			leadScope, err := r.growthLeadAttribution.ScopeTo(tenantID)
			if err == nil {
				var leadDoc struct {
					Source     string `bson:"source"`
					CampaignID string `bson:"campaign_id"`
				}
				if err := leadScope.FindOne(ctx, bson.M{"_id": fact.LeadID}).Decode(&leadDoc); err == nil {
					source = leadDoc.Source
					campaignID = leadDoc.CampaignID
				}
			}
		}

		if source == "" && fact.AppID != "" {
			appScope, err := r.growthAppAttribution.ScopeTo(tenantID)
			if err == nil {
				var appDoc struct {
					LeadID string `bson:"lead_id"`
				}
				if err := appScope.FindOne(ctx, bson.M{"_id": fact.AppID}).Decode(&appDoc); err == nil && appDoc.LeadID != "" {
					leadScope, err := r.growthLeadAttribution.ScopeTo(tenantID)
					if err == nil {
						var leadDoc struct {
							Source     string `bson:"source"`
							CampaignID string `bson:"campaign_id"`
						}
						if err := leadScope.FindOne(ctx, bson.M{"_id": appDoc.LeadID}).Decode(&leadDoc); err == nil {
							source = leadDoc.Source
							campaignID = leadDoc.CampaignID
						}
					}
				}
			}
		}

		key := factKey{
			stage:       fact.Stage,
			source:      source,
			campaignID:  campaignID,
			programmeID: fact.ProgrammeID,
			intakeID:    fact.IntakeID,
		}
		factMap[key]++
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterate facts: %w", err)
	}

	// Convert to output
	var out []domain.GrowthRollup
	for key, count := range factMap {
		out = append(out, domain.GrowthRollup{
			Stage:       key.stage,
			Source:      key.source,
			CampaignID:  key.campaignID,
			ProgrammeID: key.programmeID,
			IntakeID:    key.intakeID,
			Value:       float64(count),
		})
	}
	return out, nil
}

// EnsureIndexes creates the indexes the queries above rely on.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	indexes := map[string][][]bson.E{
		MetricsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "metric_name", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "bucket_date", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
		},
		ProcessedEventsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
		},
		AssessmentScoreFactsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id",
				Value: 1},
				{Key: "bucket_date",
					Value: 1},
				{Key: "student_id",
					Value: 1},
				{Key: "subject_id",
					Value: 1},
				{Key: "academic_year_id",
					Value: 1}},
		},
		GrowthLeadAttributionCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
		},
		GrowthApplicationAttributionCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
		},
		GrowthEventFactsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "bucket_date", Value: 1}, {Key: "stage", Value: 1}},
		},
	}

	for collName, specs := range indexes {
		coll := store.Database().Collection(collName)
		models := make([]mongo.IndexModel, 0, len(specs))
		for _, spec := range specs {
			models = append(models, mongo.IndexModel{Keys: spec})
		}
		if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("analytics: ensure indexes on %s: %w", collName, err)
		}
	}
	return nil
}

// A per-tenant guard serializes fact writes and aggregate recomputation. Snapshot
// transactions alone permit write skew between distinct facts in one rollup.
func (r *Repository) serializeTenant(ctx context.Context, tenant string) error {
	scope, err := r.store.Collection("analytics_rollup_guards").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = scope.UpsertOne(ctx, bson.M{"_id": tenant}, bson.M{"$inc": bson.M{"version": 1}})
	return err
}
