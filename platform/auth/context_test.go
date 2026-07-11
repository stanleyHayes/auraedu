package auth

import (
	"context"
	"testing"
)

func TestWithActorAndActorFromContext(t *testing.T) {
	ctx := context.Background()
	if _, ok := ActorFromContext(ctx); ok {
		t.Fatal("expected no actor in empty context")
	}

	want := Actor{
		UserID:        "u1",
		TenantID:      "upshs",
		Role:          "teacher",
		Permissions:   []string{"attendance.mark"},
		PlatformAdmin: false,
	}
	ctx = WithActor(ctx, want)

	got, ok := ActorFromContext(ctx)
	if !ok {
		t.Fatal("expected actor in context")
	}
	if got.UserID != want.UserID || got.TenantID != want.TenantID || got.Role != want.Role || got.PlatformAdmin != want.PlatformAdmin {
		t.Fatalf("actor mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Permissions) != len(want.Permissions) || got.Permissions[0] != want.Permissions[0] {
		t.Fatalf("actor permissions mismatch: got %v, want %v", got.Permissions, want.Permissions)
	}
}
