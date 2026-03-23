package redis_test

import (
	"context"
	"testing"
	"time"

	infraRedis "distributed-fraud-detection/internal/infrastructure/redis"
	"distributed-fraud-detection/internal/infrastructure/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionCounter_CountBySender(t *testing.T) {
	t.Parallel()
	client := testutil.StartRedis(t)
	counter := infraRedis.NewTransactionCounter(client)
	ctx := context.Background()

	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	ttl := 10 * time.Minute

	t.Run("counts multiple transactions within window", func(t *testing.T) {
		t.Parallel()
		sender := "sender-count-multi"

		require.NoError(t, counter.Record(ctx, sender, now.Add(-1*time.Minute), ttl))
		require.NoError(t, counter.Record(ctx, sender, now.Add(-2*time.Minute), ttl))
		require.NoError(t, counter.Record(ctx, sender, now.Add(-3*time.Minute), ttl))

		count, err := counter.CountBySender(ctx, sender, now.Add(-5*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("excludes transactions outside window", func(t *testing.T) {
		t.Parallel()
		sender := "sender-window"

		// 2 inside window, 1 outside
		require.NoError(t, counter.Record(ctx, sender, now.Add(-1*time.Minute), ttl))
		require.NoError(t, counter.Record(ctx, sender, now.Add(-2*time.Minute), ttl))
		require.NoError(t, counter.Record(ctx, sender, now.Add(-8*time.Minute), ttl))

		// Window: last 5 minutes
		count, err := counter.CountBySender(ctx, sender, now.Add(-5*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("different senders are isolated", func(t *testing.T) {
		t.Parallel()
		senderA := "sender-iso-a"
		senderB := "sender-iso-b"

		require.NoError(t, counter.Record(ctx, senderA, now, ttl))
		require.NoError(t, counter.Record(ctx, senderA, now.Add(-1*time.Second), ttl))
		require.NoError(t, counter.Record(ctx, senderB, now, ttl))

		countA, err := counter.CountBySender(ctx, senderA, now.Add(-5*time.Minute))
		require.NoError(t, err)

		countB, err := counter.CountBySender(ctx, senderB, now.Add(-5*time.Minute))
		require.NoError(t, err)

		assert.Equal(t, 2, countA)
		assert.Equal(t, 1, countB)
	})

	t.Run("returns zero for unknown sender", func(t *testing.T) {
		t.Parallel()

		count, err := counter.CountBySender(ctx, "sender-unknown", now.Add(-5*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
