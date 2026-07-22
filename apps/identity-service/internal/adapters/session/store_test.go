package session

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewFromEnvRejectsInMemorySessionsInProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("REDIS_URL", "")
	store, err := NewFromEnv(context.Background())
	if err == nil || store != nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("production session fallback: store=%T err=%v", store, err)
	}
}

func TestNewFromEnvAllowsInMemorySessionsInDevelopment(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("REDIS_URL", "")
	store, err := NewFromEnv(context.Background())
	if err != nil || store == nil {
		t.Fatalf("development session fallback: store=%T err=%v", store, err)
	}
}

func TestMemoryStoreIsTenantScopedAndExpiresSessions(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	store := &Store{prefix: "test", mem: make(map[string]memEntry), memClock: func() time.Time { return now }}
	ctx := context.Background()
	if err := store.Save(ctx, "school-a", "user-a", "same-hash", time.Hour); err != nil {
		t.Fatalf("save school A: %v", err)
	}
	if err := store.Save(ctx, "school-b", "user-b", "same-hash", 2*time.Hour); err != nil {
		t.Fatalf("save school B: %v", err)
	}
	if got, ok, err := store.Find(ctx, "school-a", "same-hash"); err != nil || !ok || got != "user-a" {
		t.Fatalf("school A lookup: user=%q ok=%v err=%v", got, ok, err)
	}
	if err := store.Revoke(ctx, "school-a", "same-hash"); err != nil {
		t.Fatalf("revoke school A: %v", err)
	}
	if _, ok, err := store.Find(ctx, "school-a", "same-hash"); err != nil || ok {
		t.Fatalf("revoked school A remained: ok=%v err=%v", ok, err)
	}
	if got, ok, err := store.Find(ctx, "school-b", "same-hash"); err != nil || !ok || got != "user-b" {
		t.Fatalf("school A revoke crossed tenant: user=%q ok=%v err=%v", got, ok, err)
	}
	now = now.Add(3 * time.Hour)
	if _, ok, err := store.Find(ctx, "school-b", "same-hash"); err != nil || ok {
		t.Fatalf("expired school B remained: ok=%v err=%v", ok, err)
	}
}
