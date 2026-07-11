package auth

import "context"

type actorKey struct{}

func WithActor(parent context.Context, actor Actor) context.Context {
	return context.WithValue(parent, actorKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	v, ok := ctx.Value(actorKey{}).(Actor)
	return v, ok
}
