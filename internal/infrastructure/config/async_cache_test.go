package config_test

import (
	"context"
	"testing"
	"time"

	infraConfig "distributed-fraud-detection/internal/infrastructure/config"
	"distributed-fraud-detection/internal/infrastructure/postgres"
	"distributed-fraud-detection/internal/infrastructure/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopConfigMetrics struct{}

func (n *noopConfigMetrics) ConfigRefreshSuccess() {}
func (n *noopConfigMetrics) ConfigRefreshError()   {}

func seedConfig(t *testing.T, db interface{ ExecContext(context.Context, string, ...any) (interface{ RowsAffected() (int64, error) }, error) }, key, value string) {
	// Use raw sql.DB from testutil, cast via helper
}

func TestAsyncConfigCache_LoadAndRead(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `INSERT INTO config (key, value) VALUES ('test.key', '42'), ('test.float', '3.14')`)
	require.NoError(t, err)

	source := postgres.NewConfigRepository(db)
	cache, err := infraConfig.NewAsyncConfigCache(ctx, source, &noopConfigMetrics{}, 1*time.Hour)
	require.NoError(t, err)

	t.Run("IsReady after load", func(t *testing.T) {
		assert.True(t, cache.IsReady())
	})

	t.Run("GetInt returns correct value", func(t *testing.T) {
		val, err := cache.GetInt(ctx, "test.key")
		require.NoError(t, err)
		assert.Equal(t, 42, val)
	})

	t.Run("GetFloat returns correct value", func(t *testing.T) {
		val, err := cache.GetFloat(ctx, "test.float")
		require.NoError(t, err)
		assert.InDelta(t, 3.14, val, 0.001)
	})

	t.Run("missing key returns error", func(t *testing.T) {
		_, err := cache.GetInt(ctx, "nonexistent")
		require.Error(t, err)
	})
}

func TestAsyncConfigCache_Refresh(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := db.ExecContext(ctx, `INSERT INTO config (key, value) VALUES ('refresh.key', '100')`)
	require.NoError(t, err)

	source := postgres.NewConfigRepository(db)
	cache, err := infraConfig.NewAsyncConfigCache(ctx, source, &noopConfigMetrics{}, 1*time.Second)
	require.NoError(t, err)

	// Initial value
	val, err := cache.GetInt(ctx, "refresh.key")
	require.NoError(t, err)
	assert.Equal(t, 100, val)

	// Update DB
	_, err = db.ExecContext(ctx, `UPDATE config SET value = '200' WHERE key = 'refresh.key'`)
	require.NoError(t, err)

	// Wait for refresh cycle (1s interval + margin)
	time.Sleep(2 * time.Second)

	// Should see new value
	val, err = cache.GetInt(ctx, "refresh.key")
	require.NoError(t, err)
	assert.Equal(t, 200, val)
}
