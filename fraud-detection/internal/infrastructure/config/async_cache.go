package config

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
)

type AsyncConfigCache struct {
	source  domain.ConfigSource
	metrics domain.ConfigMetrics

	mu    sync.RWMutex
	store map[string]string
	ready bool
}

func NewAsyncConfigCache(
	ctx context.Context,
	source domain.ConfigSource,
	metrics domain.ConfigMetrics,
	refreshInterval time.Duration,
) (*AsyncConfigCache, error) {
	c := &AsyncConfigCache{
		source:  source,
		metrics: metrics,
	}

	data, err := source.LoadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("initial config load: %w", err)
	}

	c.mu.Lock()
	c.store = data
	c.ready = true
	c.mu.Unlock()

	go c.refreshLoop(ctx, refreshInterval)

	return c, nil
}

func (c *AsyncConfigCache) GetFloat(_ context.Context, key string) (float64, error) {
	val, err := c.get(key)
	if err != nil {
		return 0, err
	}

	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("config key %s: cannot parse %q as float: %w", key, val, err)
	}

	return f, nil
}

func (c *AsyncConfigCache) GetInt(_ context.Context, key string) (int, error) {
	val, err := c.get(key)
	if err != nil {
		return 0, err
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("config key %s: cannot parse %q as int: %w", key, val, err)
	}

	return i, nil
}

func (c *AsyncConfigCache) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

func (c *AsyncConfigCache) get(key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.store[key]
	if !ok {
		return "", fmt.Errorf("key %q: %w", key, domain.ErrConfigNotFound)
	}

	return val, nil
}

func (c *AsyncConfigCache) refreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

func (c *AsyncConfigCache) refresh(ctx context.Context) {
	data, err := c.source.LoadAll(ctx)
	if err != nil {
		c.metrics.ConfigRefreshError()
		log.Printf("config refresh failed, keeping stale cache: %v", err)
		return
	}

	c.mu.Lock()
	c.store = data
	c.mu.Unlock()

	c.metrics.ConfigRefreshSuccess()
}
