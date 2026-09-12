package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func seedEvent(ctx context.Context, t *testing.T, coll *Collection, docID, eventID string) {
	t.Helper()
	scope, err := coll.Scope(tenantCtx("upshs"))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if _, err := scope.InsertWithEvents(ctx, bson.M{"_id": docID}, event(t, "upshs", eventID)); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestClaimLeasesAnEventExactlyOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("orders")
	seedEvent(ctx, t, coll, "d1", "e1")

	outbox := NewClaimableOutbox(coll, time.Minute)
	first, err := outbox.Claim(ctx, 10)
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim returned %d events (err=%v); want 1", len(first), err)
	}
	if first[0].Attempts != 1 {
		t.Fatalf("claim did not count the attempt: %d", first[0].Attempts)
	}

	// A second worker must not get the same leased event.
	second, err := outbox.Claim(ctx, 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("a leased event was claimed twice: %+v", second)
	}
}

func TestAnExpiredLeaseIsReclaimedRatherThanLost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("orders")
	seedEvent(ctx, t, coll, "d1", "e1")

	outbox := NewClaimableOutbox(coll, time.Minute)
	if claimed, err := outbox.Claim(ctx, 10); err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %d events, err=%v", len(claimed), err)
	}

	// A worker that dies mid-publish must not strand its event forever.
	later := NewClaimableOutbox(coll, time.Minute).
		WithClock(func() time.Time { return time.Now().UTC().Add(2 * time.Minute) })
	reclaimed, err := later.Claim(ctx, 10)
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("an expired lease was not reclaimed: %d events, err=%v", len(reclaimed), err)
	}
	if reclaimed[0].Attempts != 2 {
		t.Fatalf("reclaim did not count the second attempt: %d", reclaimed[0].Attempts)
	}
}

func TestMarkPublishedRemovesTheEventAndIsSafeToRepeat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("orders")
	seedEvent(ctx, t, coll, "d1", "e1")

	outbox := NewClaimableOutbox(coll, time.Minute)
	claimed, err := outbox.Claim(ctx, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %d, err=%v", len(claimed), err)
	}
	if err := outbox.MarkPublished(ctx, claimed[0].DocumentID, claimed[0].Event.ID); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	// At-least-once means a repeat acknowledgement is expected, not an error.
	if err := outbox.MarkPublished(ctx, claimed[0].DocumentID, claimed[0].Event.ID); err != nil {
		t.Fatalf("repeat acknowledgement failed: %v", err)
	}

	later := NewClaimableOutbox(coll, time.Minute).
		WithClock(func() time.Time { return time.Now().UTC().Add(time.Hour) })
	if again, err := later.Claim(ctx, 10); err != nil || len(again) != 0 {
		t.Fatalf("a published event came back: %d events, err=%v", len(again), err)
	}
}

func TestMarkFailedBacksOffBeforeRetrying(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MongoDB outbox test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	coll := openTestStore(ctx, t).Collection("orders")
	seedEvent(ctx, t, coll, "d1", "e1")

	outbox := NewClaimableOutbox(coll, time.Minute)
	claimed, err := outbox.Claim(ctx, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %d, err=%v", len(claimed), err)
	}
	if err := outbox.MarkFailed(ctx, claimed[0].DocumentID, claimed[0].Event.ID, "bus unavailable"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Immediately retrying a failing target would hammer it.
	if again, err := outbox.Claim(ctx, 10); err != nil || len(again) != 0 {
		t.Fatalf("a failed event was retried with no backoff: %d events, err=%v", len(again), err)
	}

	// After the backoff it must come back rather than be dropped.
	later := NewClaimableOutbox(coll, time.Minute).
		WithClock(func() time.Time { return time.Now().UTC().Add(10 * time.Minute) })
	retried, err := later.Claim(ctx, 10)
	if err != nil || len(retried) != 1 {
		t.Fatalf("a failed event was dropped instead of retried: %d events, err=%v", len(retried), err)
	}
}

func TestBackoffIsCappedLikeTheSQLAdapters(t *testing.T) {
	if got := backoff(0); got != time.Second {
		t.Fatalf("backoff(0) = %v; want 1s", got)
	}
	if got := backoff(3); got != 8*time.Second {
		t.Fatalf("backoff(3) = %v; want 8s", got)
	}
	// LEAST(300, power(2, attempts)) in SQL.
	if got := backoff(20); got != 300*time.Second {
		t.Fatalf("backoff(20) = %v; want the 300s cap", got)
	}
}
