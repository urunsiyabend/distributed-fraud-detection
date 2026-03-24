package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)
	
type TransactionCounter struct {
	client *redis.Client
}

func NewTransactionCounter(client *redis.Client) *TransactionCounter {
	return &TransactionCounter{client: client}
}

func (r *TransactionCounter) CountBySender(ctx context.Context, senderID string, since time.Time) (int, error) {
	key := fmt.Sprintf("tx:count:%s", senderID)

	count, err := r.client.ZCount(ctx, key, fmt.Sprintf("%d", since.UnixMilli()), "+inf").Result()
	if err != nil {
		return 0, fmt.Errorf("redis zcount for sender %s: %w", senderID, err)
	}

	return int(count), nil
}

// Record adds a transaction timestamp to the sender's sorted set with a TTL for automatic cleanup.
func (r *TransactionCounter) Record(ctx context.Context, senderID string, txTime time.Time, ttl time.Duration) error {
	key := fmt.Sprintf("tx:count:%s", senderID)

	pipe := r.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(txTime.UnixMilli()),
		Member: txTime.UnixMilli(),
	})
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline for sender %s: %w", senderID, err)
	}

	return nil
}
