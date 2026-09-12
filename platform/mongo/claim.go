package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Most services expose their outbox as claim / mark-published / mark-failed
// rather than a single drain, because a worker publishes to NATS between the
// claim and the acknowledgement. ClaimableOutbox is that shape over the events
// embedded by InsertWithEvents and UpdateWithEvents, so a service adapter is
// thin and every service retries and backs off identically.
//
// A claim leases events for a bounded window instead of deleting them, so a
// worker that dies mid-publish releases its events when the lease expires
// rather than losing them.

// ClaimedEvent is a leased event plus the document it came from, which the
// caller needs to acknowledge it.
type ClaimedEvent struct {
	DocumentID any
	Event      PendingEvent
	Attempts   int
}

type ClaimableOutbox struct {
	coll  *Collection
	lease time.Duration
	now   func() time.Time
}

func NewClaimableOutbox(coll *Collection, lease time.Duration) *ClaimableOutbox {
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	return &ClaimableOutbox{coll: coll, lease: lease, now: time.Now}
}

// WithClock replaces the clock, for tests that need to expire a lease.
func (o *ClaimableOutbox) WithClock(now func() time.Time) *ClaimableOutbox {
	o.now = now
	return o
}

const (
	leasedUntilField = "leased_until"
	attemptsField    = "attempts"
	nextAttemptField = "next_attempt_at"
	lastErrorField   = "last_error"
)

// Claim leases up to limit events that are due and not already leased.
//
// It reads across tenants deliberately: this is a service draining its own
// collection, not a request acting for a user.
func (o *ClaimableOutbox) Claim(ctx context.Context, limit int) ([]ClaimedEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	now := o.now().UTC()
	due := bson.M{
		"$elemMatch": bson.M{
			"$or": bson.A{
				bson.M{leasedUntilField: bson.M{"$exists": false}},
				bson.M{leasedUntilField: bson.M{"$lte": now}},
			},
			"$and": bson.A{
				bson.M{"$or": bson.A{
					bson.M{nextAttemptField: bson.M{"$exists": false}},
					bson.M{nextAttemptField: bson.M{"$lte": now}},
				}},
			},
		},
	}

	cursor, err := o.coll.coll.Find(ctx, bson.M{PendingField: due},
		options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("mongo outbox: find claimable: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	claimed := make([]ClaimedEvent, 0, limit)
	for cursor.Next(ctx) && len(claimed) < limit {
		var doc struct {
			ID      any `bson:"_id"`
			Pending []struct {
				PendingEvent `bson:",inline"`
				LeasedUntil  *time.Time `bson:"leased_until,omitempty"`
				Attempts     int        `bson:"attempts,omitempty"`
				NextAttempt  *time.Time `bson:"next_attempt_at,omitempty"`
			} `bson:"_pending_events"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo outbox: decode claimable: %w", err)
		}
		for _, entry := range doc.Pending {
			if len(claimed) >= limit {
				break
			}
			if entry.LeasedUntil != nil && entry.LeasedUntil.After(now) {
				continue
			}
			if entry.NextAttempt != nil && entry.NextAttempt.After(now) {
				continue
			}
			// findOneAndUpdate on one document is atomic, so two workers cannot
			// take the same event: the loser's filter no longer matches.
			res := o.coll.coll.FindOneAndUpdate(ctx,
				bson.M{
					"_id": doc.ID,
					PendingField: bson.M{"$elemMatch": bson.M{
						"id": entry.ID,
						"$or": bson.A{
							bson.M{leasedUntilField: bson.M{"$exists": false}},
							bson.M{leasedUntilField: bson.M{"$lte": now}},
						},
					}},
				},
				bson.M{"$set": bson.M{
					PendingField + ".$." + leasedUntilField: now.Add(o.lease),
					PendingField + ".$." + attemptsField:    entry.Attempts + 1,
				}},
			)
			if err := res.Err(); err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					continue // another worker took it
				}
				return nil, fmt.Errorf("mongo outbox: lease %s: %w", entry.ID, err)
			}
			claimed = append(claimed, ClaimedEvent{
				DocumentID: doc.ID, Event: entry.PendingEvent, Attempts: entry.Attempts + 1,
			})
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("mongo outbox: iterate claimable: %w", err)
	}
	return claimed, nil
}

// MarkPublished removes a delivered event. It is safe to call twice: this outbox
// is at-least-once, so a repeat acknowledgement is expected, not an error.
func (o *ClaimableOutbox) MarkPublished(ctx context.Context, documentID any, eventID string) error {
	_, err := o.coll.coll.UpdateOne(ctx,
		bson.M{"_id": documentID},
		bson.M{"$pull": bson.M{PendingField: bson.M{"id": eventID}}},
	)
	if err != nil {
		return fmt.Errorf("mongo outbox: mark %s published: %w", eventID, err)
	}
	return nil
}

// MarkFailed releases the lease and schedules a retry with the same capped
// exponential backoff the PostgreSQL adapters use.
func (o *ClaimableOutbox) MarkFailed(ctx context.Context, documentID any, eventID, reason string) error {
	attempts, err := o.attemptsFor(ctx, documentID, eventID)
	if err != nil {
		return err
	}
	_, err = o.coll.coll.UpdateOne(ctx,
		bson.M{"_id": documentID, PendingField: bson.M{"$elemMatch": bson.M{"id": eventID}}},
		bson.M{
			"$set": bson.M{
				PendingField + ".$." + nextAttemptField: o.now().UTC().Add(backoff(attempts)),
				PendingField + ".$." + lastErrorField:   truncateReason(reason),
			},
			"$unset": bson.M{PendingField + ".$." + leasedUntilField: ""},
		},
	)
	if err != nil {
		return fmt.Errorf("mongo outbox: mark %s failed: %w", eventID, err)
	}
	return nil
}

func (o *ClaimableOutbox) attemptsFor(ctx context.Context, documentID any, eventID string) (int, error) {
	var doc struct {
		Pending []struct {
			ID       string `bson:"id"`
			Attempts int    `bson:"attempts,omitempty"`
		} `bson:"_pending_events"`
	}
	err := o.coll.coll.FindOne(ctx, bson.M{"_id": documentID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		return 0, fmt.Errorf("mongo outbox: read attempts for %s: %w", eventID, err)
	}
	for _, entry := range doc.Pending {
		if entry.ID == eventID {
			return entry.Attempts, nil
		}
	}
	return 0, nil
}

// backoff mirrors LEAST(300, power(2, attempts)) seconds from the SQL adapters,
// so a service behaves the same on either driver.
func backoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 8 {
		return 300 * time.Second
	}
	seconds := 1 << uint(attempts)
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// truncateReason keeps a failure note bounded; it is operator diagnostics, and
// a provider error can carry a large body that has no business being stored.
func truncateReason(reason string) string {
	const max = 500
	if len(reason) <= max {
		return reason
	}
	return reason[:max]
}
