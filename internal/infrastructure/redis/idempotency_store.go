package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewIdempotencyStore(client *redis.Client, ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{client: client, ttl: ttl}
}

func (s *IdempotencyStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, idempotencyKey(key)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get idempotency %s: %w", key, err)
	}
	return val, true, nil
}

func (s *IdempotencyStore) Set(ctx context.Context, key string, value []byte) error {
	ok, err := s.client.SetNX(ctx, idempotencyKey(key), value, s.ttl).Result()
	if err != nil {
		return fmt.Errorf("redis setnx idempotency %s: %w", key, err)
	}
	if !ok {
		return nil // already exists, no-op
	}
	return nil
}

func idempotencyKey(key string) string {
	return fmt.Sprintf("idempotency:%s", key)
}
