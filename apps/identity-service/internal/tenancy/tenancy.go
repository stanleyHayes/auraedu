// Package tenancy is a minimal local stub for platform/tenancy (AURA-2.3).
package tenancy

import (
	"context"

	"github.com/auraedu/platform/auth"
)

type ctxKey struct{}

func WithActor(ctx context.Context, actor auth.Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, actor)
}

func ActorFromContext(ctx context.Context) auth.Actor {
	if a, ok := ctx.Value(ctxKey{}).(auth.Actor); ok {
		return a
	}
	return auth.Actor{}
}
