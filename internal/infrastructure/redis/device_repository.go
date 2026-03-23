package redis

import (
	"context"
	"fmt"

	"distributed-fraud-detection/internal/domain"

	"github.com/redis/go-redis/v9"
)

// DeviceRepository is a read-through cache over a backing Postgres DeviceRepository.
// Miss → query Postgres → populate Redis SET → return.
// Hit  → return from Redis (sub-ms).
type DeviceRepository struct {
	client  *redis.Client
	backing domain.DeviceRepository
}

func NewDeviceRepository(client *redis.Client, backing domain.DeviceRepository) *DeviceRepository {
	return &DeviceRepository{client: client, backing: backing}
}

func (r *DeviceRepository) IsKnownDevice(ctx context.Context, senderID string, deviceID string) (bool, error) {
	key := fmt.Sprintf("devices:%s", senderID)

	// Check Redis first
	exists, err := r.client.SIsMember(ctx, key, deviceID).Result()
	if err == nil && exists {
		return true, nil
	}

	// Check if set exists at all (avoid false negatives on empty cache)
	setExists, err := r.client.Exists(ctx, key).Result()
	if err == nil && setExists > 0 {
		// Set is cached but device not in it → not known
		return false, nil
	}

	// Cache miss — fall through to Postgres
	known, err := r.backing.IsKnownDevice(ctx, senderID, deviceID)
	if err != nil {
		return false, err
	}

	// Warm the cache for this sender (best-effort, don't fail on cache write error)
	if known {
		r.client.SAdd(ctx, key, deviceID)
	}

	return known, nil
}
