package mongo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	claimed, err := r.outbox.Claim(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ports.OutboxEvent, 0, len(claimed))
	for _, item := range claimed {
		e := item.Event
		at, err := time.Parse(time.RFC3339Nano, e.Time)
		if err != nil {
			return nil, err
		}
		tenant := e.TenantID
		if tenant == platformIdentityScope {
			tenant = ""
		}
		out = append(out, ports.OutboxEvent{ID: e.ID, TenantID: tenant, EventType: e.Type, Payload: e.Data, CreatedAt: at})
	}
	return out, nil
}
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.outbox.MarkPublishedByEventID(ctx, id)
}
func (r *Repository) MarkFailed(ctx context.Context, id, reason string) error {
	return r.outbox.MarkFailedByEventID(ctx, id, reason)
}
func ended(t token, cutoff time.Time) bool {
	return t.Expires.Before(cutoff) && (t.Used == nil || t.Used.Before(cutoff)) && (t.Revoked == nil || t.Revoked.Before(cutoff))
}
func (r *Repository) CleanupAuthArtifacts(ctx context.Context, c ports.AuthRetentionCutoffs) (ports.AuthCleanupResult, error) {
	result := ports.AuthCleanupResult{}
	if c.BatchSize <= 0 || c.BatchSize > 10000 {
		return result, fmt.Errorf("identity cleanup batch size must be between 1 and 10000")
	}
	s, err := r.privileged(ctx)
	if err != nil {
		return result, err
	}
	// Select oldest artifacts, not the first N accounts, so repeated runs make progress.
	q := bson.M{"deleted": bson.M{"$ne": true},
		"$or": bson.A{bson.M{"refresh.expires": bson.M{"$lt": c.RefreshFamiliesBefore}},
			bson.M{"resets.expires": bson.M{"$lt": c.PasswordResetsBefore}},
			bson.M{"invites.token.expires": bson.M{"$lt": c.InvitesBefore}}}}
	cursor, err := s.Find(ctx, q, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return result, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			slog.Error("close identity cleanup cursor", "err", err)
		}
	}()
	processed := 0
	for cursor.Next(ctx) && processed < c.BatchSize {
		var key struct {
			ID string `bson:"_id"`
		}
		if err = cursor.Decode(&key); err != nil {
			return result, err
		}
		var removed ports.AuthCleanupResult
		err = r.mutate(ctx, s, bson.M{"_id": key.ID}, func(a *account) error {
			removed = trimAccountArtifacts(a, c)
			return nil
		})
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return result, err
		}
		if removed.RefreshTokens+removed.PasswordResets+removed.Invites > 0 {
			processed++
		}
		result.RefreshTokens += removed.RefreshTokens
		result.PasswordResets += removed.PasswordResets
		result.Invites += removed.Invites
	}
	// Tombstones retain their outbox until delivery; then deletion may finish.
	if _, err = r.outbox.PurgeSettled(ctx, bson.M{"deleted": true}); err != nil {
		return result, err
	}
	return result, cursor.Err()
}

func trimAccountArtifacts(a *account, c ports.AuthRetentionCutoffs) ports.AuthCleanupResult {
	removed := ports.AuthCleanupResult{}
	live := map[string]bool{}
	for _, t := range a.Refresh {
		if !t.Expires.Before(c.RefreshFamiliesBefore) {
			live[t.Family] = true
		}
	}
	refresh := a.Refresh[:0]
	for _, t := range a.Refresh {
		if live[t.Family] {
			refresh = append(refresh, t)
		} else {
			removed.RefreshTokens++
		}
	}
	a.Refresh = refresh
	resets := a.Resets[:0]
	for _, t := range a.Resets {
		if ended(t, c.PasswordResetsBefore) {
			removed.PasswordResets++
		} else {
			resets = append(resets, t)
		}
	}
	a.Resets = resets
	invites := a.Invites[:0]
	for _, i := range a.Invites {
		if ended(i.Token, c.InvitesBefore) {
			removed.Invites++
		} else {
			invites = append(invites, i)
		}
	}
	a.Invites = invites
	return removed
}
