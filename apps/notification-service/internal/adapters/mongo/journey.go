package mongo

import (
	"context"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type JourneyRepository struct{ *base }

var _ ports.JourneyRepository = (*JourneyRepository)(nil)

func NewJourneyRepository(s *pmongo.Store) *JourneyRepository { return &JourneyRepository{&base{s}} }
func journeyPayload(j *domain.Journey, actor string) map[string]any {
	return map[string]any{"journey_id": j.ID, "status": j.Status,
		"trigger_event": j.TriggerEvent, "version": j.Version,
		"step_count": len(j.Steps), "changed_by": actor, "changed_at": j.UpdatedAt}
}
func (r *JourneyRepository) CreateJourney(ctx context.Context, tenant string, j *domain.Journey) error {
	j.TenantID = tenant
	e, err := event(tenant, "communication.journey_changed.v1", journeyPayload(j, j.CreatedBy))
	if err != nil {
		return err
	}
	return r.write(ctx, tenant, "communication_journeys", j.ID, j, true, false, []tenancy.CloudEvent{e})
}
func (r *JourneyRepository) GetJourney(ctx context.Context, tenant, id string) (*domain.Journey, error) {
	return get[domain.Journey](ctx, r.base, tenant, "communication_journeys", live(id))
}
func (r *JourneyRepository) ListJourneys(ctx context.Context, tenant string, f ports.JourneyFilter) ([]*domain.Journey, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	q := bson.M{}
	if f.Status != "" {
		q["body.status"] = f.Status
	}
	if f.TriggerEvent != "" {
		q["body.triggerevent"] = f.TriggerEvent
	}

	scope, err := r.store.Collection("communication_journeys").ScopeTo(tenant)
	if err != nil {
		return nil, err
	}
	q["deleted"] = false
	cur, err := scope.Find(ctx, q, options.Find().SetSort(bson.D{{Key: "body.createdat", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(f.Limit)))
	if err != nil {
		return nil, err
	}
	defer closeCursor(ctx, cur)
	out := make([]*domain.Journey, 0)
	for cur.Next(ctx) {
		var d struct {
			Body domain.Journey `bson:"body"`
		}
		if err = cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, &d.Body)
	}
	return out, cur.Err()
}
func (r *JourneyRepository) UpdateJourneyStatus(ctx context.Context, tenant string, j *domain.Journey, actor string) error {
	j.TenantID = tenant
	e, err := event(tenant, "communication.journey_changed.v1", journeyPayload(j, actor))
	if err != nil {
		return err
	}
	return r.write(ctx, tenant, "communication_journeys", j.ID, j, false, false, []tenancy.CloudEvent{e})
}
func (r *JourneyRepository) ListActiveJourneysByTrigger(ctx context.Context, tenant, kind string) ([]*domain.Journey, error) {
	return r.ListJourneys(ctx, tenant, ports.JourneyFilter{Status: "active", TriggerEvent: kind, Limit: 100})
}
func (r *JourneyRepository) EnrollJourney(ctx context.Context, e ports.JourneyEnrollment) (bool, error) {
	created := false
	err := r.tx(ctx, func(ctx context.Context) error {
		created = false
		j, err := r.GetJourney(ctx, e.TenantID, e.JourneyID)
		if err != nil {
			return err
		}
		if j.Status != "active" {
			return nil
		}
		scope, err := r.store.Collection("communication_journeys").ScopeTo(e.TenantID)
		if err != nil {
			return err
		}
		res, err := scope.UpdateOne(ctx, bson.M{"_id": e.JourneyID, "body.status": "active"}, bson.M{"$inc": bson.M{"enrollment_fence": 1}})
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return nil
		}
		enrollments, err := r.store.Collection("communication_journey_enrollments").ScopeTo(e.TenantID)
		if err != nil {
			return err
		}
		n, err := enrollments.CountDocuments(ctx, bson.M{"body.journeyid": e.JourneyID, "body.eventid": e.EventID})
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		state := "active"
		if len(e.Messages) == 0 {
			state = "completed"
		}
		body := e
		body.Messages = nil
		if _, err = enrollments.InsertOne(ctx, bson.M{"_id": e.ID, "body": body, "status": state, "version": j.Version}); err != nil {
			return err
		}
		messages := NewMessageRepository(r.store)
		for _, m := range e.Messages {
			if err = messages.Create(ctx, e.TenantID, m); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return created, err
}
func (r *JourneyRepository) CancelJourneysForEvent(ctx context.Context, tenant, lead, eventID, kind string) (int64, error) {
	var count int64
	err := r.tx(ctx, func(ctx context.Context) error {
		count = 0
		journeys, err := r.store.Collection("communication_journeys").ScopeTo(tenant)
		if err != nil {
			return err
		}
		cur, err := journeys.Find(ctx, bson.M{"body.cancelonevents": kind})
		if err != nil {
			return err
		}
		defer closeCursor(ctx, cur)
		var ids []string
		for cur.Next(ctx) {
			var d struct {
				ID string `bson:"_id"`
			}
			if err = cur.Decode(&d); err != nil {
				return err
			}
			ids = append(ids, d.ID)
		}
		if err = cur.Err(); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		enrollments, err := r.store.Collection("communication_journey_enrollments").ScopeTo(tenant)
		if err != nil {
			return err
		}
		ec, err := enrollments.Find(ctx, bson.M{"body.journeyid": bson.M{"$in": ids}, "body.leadid": lead, "status": "active"})
		if err != nil {
			return err
		}
		defer closeCursor(ctx, ec)
		messages, err := r.store.Collection("messages").ScopeTo(tenant)
		if err != nil {
			return err
		}
		for ec.Next(ctx) {
			var e struct {
				ID string `bson:"_id"`
			}
			if err = ec.Decode(&e); err != nil {
				return err
			}
			if _, err = enrollments.UpdateOne(ctx, bson.M{"_id": e.ID,
				"status": "active"}, bson.M{"$set": bson.M{"status": "cancelled",
				"cancelled_at": time.Now().UTC(), "cancellation_event": kind}}); err != nil {
				return err
			}
			res, err := messages.UpdateMany(ctx, bson.M{"body.status": "pending",
				"body.metadata.journey_enrollment_id": e.ID}, bson.M{"$set": bson.M{"body.status": "cancelled",
				"body.updatedat": time.Now().UTC(), "body.metadata.journey_cancellation_event": kind,
				"body.metadata.journey_cancellation_event_id": eventID}})
			if err != nil {
				return err
			}
			count += res.ModifiedCount
		}
		return ec.Err()
	})
	return count, err
}
func (r *JourneyRepository) FinalizeJourneyEnrollment(ctx context.Context, tenant, id string) error {
	return r.tx(ctx, func(ctx context.Context) error {
		messages, err := r.store.Collection("messages").ScopeTo(tenant)
		if err != nil {
			return err
		}
		n, err := messages.CountDocuments(ctx, bson.M{"body.metadata.journey_enrollment_id": id, "body.status": "pending"})
		if err != nil || n > 0 {
			return err
		}
		es, err := r.store.Collection("communication_journey_enrollments").ScopeTo(tenant)
		if err != nil {
			return err
		}
		_, err = es.UpdateOne(ctx, bson.M{"_id": id, "status": "active"}, bson.M{"$set": bson.M{"status": "completed"}})
		return err
	})
}
func (r *JourneyRepository) JourneyStats(ctx context.Context, tenant, id string) (ports.JourneyStats, error) {
	var stats ports.JourneyStats
	scope, err := r.store.Collection("communication_journey_enrollments").ScopeTo(tenant)
	if err != nil {
		return stats, err
	}
	cur, err := scope.Find(ctx, bson.M{"body.journeyid": id})
	if err != nil {
		return stats, err
	}
	defer closeCursor(ctx, cur)
	for cur.Next(ctx) {
		var d struct {
			Body ports.JourneyEnrollment `bson:"body"`
		}
		if err = cur.Decode(&d); err != nil {
			return stats, err
		}
		stats.Enrolled++
		stats.Skipped += int64(d.Body.SkippedSteps)
	}
	if err = cur.Err(); err != nil {
		return stats, err
	}
	ms, err := r.store.Collection("messages").ScopeTo(tenant)
	if err != nil {
		return stats, err
	}
	mc, err := ms.Find(ctx, bson.M{"body.metadata.journey_id": id, "deleted": false})
	if err != nil {
		return stats, err
	}
	defer closeCursor(ctx, mc)
	for mc.Next(ctx) {
		var d struct {
			Body domain.Message `bson:"body"`
		}
		if err = mc.Decode(&d); err != nil {
			return stats, err
		}
		countMessageStats(&stats, d.Body)
	}
	return stats, mc.Err()
}

func countMessageStats(stats *ports.JourneyStats, m domain.Message) {
	switch m.Status {
	case "pending":
		stats.Scheduled++
	case "sent":
		stats.Sent++
		if m.Provider != nil {
			stats.Accepted++
		}
	case "failed":
		stats.Failed++
	case "cancelled":
		stats.Cancelled++
	}
	if m.DeliveryStatus != nil {
		switch *m.DeliveryStatus {
		case "delivered":
			stats.Delivered++
		case "delayed":
			stats.Delayed++
		case "bounced":
			stats.Bounced++
		case "complained":
			stats.Complained++
		case "suppressed":
			stats.Suppressed++
		}
	}
}
