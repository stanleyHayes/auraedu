// Package mocks provides test doubles for the api-gateway.
package mocks

import "context"

type RedisStore struct {
	AllowFunc func(key string) (bool, error)
	calls     []string
}

func (r *RedisStore) Eval(_ context.Context, _ string, keys []string, _ ...interface{}) (interface{}, error) {
	key := ""
	if len(keys) > 0 {
		key = keys[0]
	}
	r.calls = append(r.calls, key)
	if r.AllowFunc != nil {
		return r.AllowFunc(key)
	}
	return true, nil
}

func (r *RedisStore) Calls() []string { return r.calls }
