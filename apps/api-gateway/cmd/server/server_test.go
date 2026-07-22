package servercmd

import (
	"testing"

	"github.com/auraedu/api-gateway/internal/gateway"
)

func TestNewRateLimiterRejectsMissingRedis(t *testing.T) {
	cfg := &gateway.Config{}
	health := gateway.NewHealth("api-gateway", "test")

	limiter, client, err := newRateLimiter(cfg, health)
	if err == nil || limiter != nil || client != nil {
		t.Fatalf("limiter=%v client=%v error=%v", limiter, client, err)
	}
}

func TestNewRateLimiterRejectsInvalidRedisURL(t *testing.T) {
	cfg := &gateway.Config{RedisURL: "://invalid"}
	health := gateway.NewHealth("api-gateway", "test")

	limiter, client, err := newRateLimiter(cfg, health)
	if err == nil || limiter != nil || client != nil {
		t.Fatalf("limiter=%v client=%v error=%v", limiter, client, err)
	}
}
