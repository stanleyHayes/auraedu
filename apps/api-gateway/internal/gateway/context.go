package gateway

import "context"

type contextKey int

const (
	requestIDKey contextKey = iota
	tenantIDKey
	actorKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantIDKey, id)
}

func TenantIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey).(string); ok {
		return v
	}
	return ""
}

func WithActor(ctx context.Context, actor ActorContext) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

func ActorFrom(ctx context.Context) ActorContext {
	if v, ok := ctx.Value(actorKey).(ActorContext); ok {
		return v
	}
	return ActorContext{}
}
