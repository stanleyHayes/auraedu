// Package session is a minimal local stub for the Redis session store.
package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	redis    *redis.Client
	prefix   string
	mem      map[string]memEntry
	memMu    sync.RWMutex
	memClock func() time.Time
}

type memEntry struct {
	userID    string
	expiresAt time.Time
}

func NewFromEnv(ctx context.Context) (ports.SessionStore, error) {
	prefix := config.Getenv("SESSION_KEY_PREFIX", "identity")
	if redisURL := config.Getenv("REDIS_URL", ""); redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		client := redis.NewClient(opt)
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("redis ping: %w", err)
		}
		return &Store{redis: client, prefix: prefix}, nil
	}
	return &Store{prefix: prefix, mem: make(map[string]memEntry), memClock: time.Now}, nil
}

func (s *Store) key(tenantID, tokenHash string) string {
	if tenantID == "" {
		tenantID = "platform"
	}
	return fmt.Sprintf("%s:%s:refresh:%s", s.prefix, tenantID, tokenHash)
}

func (s *Store) Save(ctx context.Context, tenantID, userID, tokenHash string, ttl time.Duration) error {
	if s.redis != nil {
		return s.redis.Set(ctx, s.key(tenantID, tokenHash), userID, ttl).Err()
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	s.mem[s.key(tenantID, tokenHash)] = memEntry{userID: userID, expiresAt: s.memClock().Add(ttl)}
	return nil
}

func (s *Store) Find(ctx context.Context, tenantID, tokenHash string) (string, bool, error) {
	if s.redis != nil {
		v, err := s.redis.Get(ctx, s.key(tenantID, tokenHash)).Result()
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return v, true, nil
	}
	s.memMu.RLock()
	defer s.memMu.RUnlock()
	e, ok := s.mem[s.key(tenantID, tokenHash)]
	if !ok || s.memClock().After(e.expiresAt) {
		return "", false, nil
	}
	return e.userID, true, nil
}

func (s *Store) Revoke(ctx context.Context, tenantID, tokenHash string) error {
	if s.redis != nil {
		return s.redis.Del(ctx, s.key(tenantID, tokenHash)).Err()
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	delete(s.mem, s.key(tenantID, tokenHash))
	return nil
}
