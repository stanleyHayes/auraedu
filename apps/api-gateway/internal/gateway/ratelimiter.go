package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

type TokenBucket struct {
	Store  RedisStore
	RPS    float64
	Burst  int
	Window time.Duration
}

type RedisStore interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) (bool, error) {
	if tb.Store == nil {
		return false, errors.New("rate limiter: no store configured")
	}
	rps := tb.RPS
	if rps <= 0 {
		rps = 20
	}
	burst := tb.Burst
	if burst <= 0 {
		burst = int(rps * 2)
	}
	window := tb.Window
	if window <= 0 {
		window = time.Second
	}

	now := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	if windowMs <= 0 {
		windowMs = 1000
	}

	result, err := tb.Store.Eval(ctx, tokenBucketScript, []string{key},
		fmt.Sprintf("%d", now),
		fmt.Sprintf("%f", rps),
		fmt.Sprintf("%d", burst),
		fmt.Sprintf("%d", windowMs),
	)
	if err != nil {
		return false, fmt.Errorf("rate limiter eval: %w", err)
	}

	return luaResultToBool(result)
}

func luaResultToBool(result interface{}) (bool, error) {
	switch v := result.(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "1", nil
	case bool:
		return v, nil
	case int:
		return v == 1, nil
	default:
		return false, fmt.Errorf("rate limiter: unexpected lua result type %T", result)
	}
}

const tokenBucketScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local window = tonumber(ARGV[4])

local state = redis.call('HMGET', key, 'tokens', 'last')
local tokens = state[1]
local last = state[2]

if tokens == false then
  tokens = burst
  last = now
else
  tokens = tonumber(tokens)
  last = tonumber(last)
  local elapsed = math.max(0, now - last)
  tokens = math.min(burst, tokens + (elapsed * rate / window))
  last = now
end

if tokens >= 1 then
  tokens = tokens - 1
  redis.call('HMSET', key, 'tokens', tokens, 'last', last)
  redis.call('PEXPIRE', key, window)
  return 1
else
  redis.call('HMSET', key, 'tokens', tokens, 'last', last)
  redis.call('PEXPIRE', key, window)
  return 0
end
`
