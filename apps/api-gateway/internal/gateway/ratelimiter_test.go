package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/auraedu/api-gateway/internal/mocks"
)

func TestTokenBucketAllow(t *testing.T) {
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return true, nil }}
	tb := &TokenBucket{Store: store, RPS: 10, Burst: 20}

	allowed, err := tb.Allow(context.Background(), "rate:upshs:GET /api/v1/students")
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed")
	}
	if len(store.Calls()) != 1 {
		t.Fatalf("calls: got %d, want 1", len(store.Calls()))
	}
}

func TestTokenBucketDeny(t *testing.T) {
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return false, nil }}
	tb := &TokenBucket{Store: store, RPS: 1, Burst: 1}

	allowed, err := tb.Allow(context.Background(), "rate:aboom:GET /api/v1/cbt")
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if allowed {
		t.Fatal("expected denied")
	}
}

func TestTokenBucketReturnsStoreError(t *testing.T) {
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return false, errors.New("redis down") }}
	tb := &TokenBucket{Store: store, RPS: 1, Burst: 1}

	_, err := tb.Allow(context.Background(), "rate:upshs:GET /")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenBucketRequiresStore(t *testing.T) {
	tb := &TokenBucket{RPS: 1, Burst: 1}
	_, err := tb.Allow(context.Background(), "rate:upshs:GET /")
	if err == nil {
		t.Fatal("expected error when store missing")
	}
}
