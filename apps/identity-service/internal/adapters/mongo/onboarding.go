package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
)

type onboardingLeaseKey struct{}

// WithOnboardingLease binds one delivery attempt to its persistent lease. It
// fences a slow worker from completing or releasing a successor's claim.
func WithOnboardingLease(ctx context.Context) context.Context {
	return context.WithValue(ctx, onboardingLeaseKey{}, uuid.NewString())
}
func leaseOwner(ctx context.Context) (string, error) {
	owner, ok := ctx.Value(onboardingLeaseKey{}).(string)
	if !ok || owner == "" {
		return "", errors.New("identity onboarding: lease owner required")
	}
	return owner, nil
}

// ClaimOnboarding uses an expiring lease. In-flight claims return an error so
// JetStream retries; only already-completed events may safely be acknowledged.
func (r *Repository) ClaimOnboarding(ctx context.Context, id, eventType, tenant string) (bool, error) {
	owner, err := leaseOwner(ctx)
	if err != nil {
		return false, err
	}
	s, err := r.store.Collection("identity_processed_events").ScopeTo(tenant)
	if err != nil {
		return false, err
	}
	now := r.now()
	_, err = s.InsertOne(ctx, bson.M{"_id": id, "event_type": eventType, "owner": owner, "leased_until": now.Add(5 * time.Minute), "complete": false})
	if err == nil {
		return true, nil
	}
	if !driver.IsDuplicateKeyError(err) {
		return false, err
	}
	result,
		err := s.UpdateOne(ctx,
		bson.M{"_id": id,
			"complete":     false,
			"leased_until": bson.M{"$lte": now}},
		bson.M{"$set": bson.M{"owner": owner,
			"leased_until": now.Add(5 * time.Minute)}})
	if err != nil {
		return false, err
	}
	if result.MatchedCount == 1 {
		return true, nil
	}
	var existing struct {
		Complete bool `bson:"complete"`
	}
	if err = s.FindOne(ctx, bson.M{"_id": id}).Decode(&existing); err != nil {
		return false, err
	}
	if existing.Complete {
		return false, nil
	}
	return false, errors.New("identity onboarding: event is leased; retry later")
}
func (r *Repository) ReleaseOnboarding(ctx context.Context, id, tenant string) error {
	owner, err := leaseOwner(ctx)
	if err != nil {
		return err
	}
	s, err := r.store.Collection("identity_processed_events").ScopeTo(tenant)
	if err != nil {
		return err
	}
	_, err = s.UpdateOne(ctx, bson.M{"_id": id, "complete": false, "owner": owner}, bson.M{"$set": bson.M{"leased_until": r.now()}})
	return err
}
func (r *Repository) CompleteOnboarding(ctx context.Context, id, tenant string) error {
	owner, err := leaseOwner(ctx)
	if err != nil {
		return err
	}
	s, err := r.store.Collection("identity_processed_events").ScopeTo(tenant)
	if err != nil {
		return err
	}
	result, err := s.UpdateOne(ctx, bson.M{"_id": id, "owner": owner, "leased_until": bson.M{"$gt": r.now()}}, bson.M{"$set": bson.M{"complete": true}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.New("identity onboarding: lease lost before completion")
	}
	return nil
}
