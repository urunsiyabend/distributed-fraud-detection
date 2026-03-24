package redis

import (
	"context"
	"fmt"
	"log"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"

	"github.com/redis/go-redis/v9"
)

// DeviceRepository is a read-through cache over a backing Postgres DeviceRepository.
// WarmUp loads all devices at startup. After that, hits are sub-ms from Redis.
// Miss on an uncached sender falls through to Postgres and populates Redis.
type DeviceRepository struct {
	client  *redis.Client
	backing domain.DeviceRepository
}

func NewDeviceRepository(client *redis.Client, backing domain.DeviceRepository) *DeviceRepository {
	return &DeviceRepository{client: client, backing: backing}
}

// WarmUp bulk-loads all known devices from Postgres into Redis.
// Call at startup before accepting traffic.
func (r *DeviceRepository) WarmUp(ctx context.Context, devices map[string][]string) error {
	pipe := r.client.Pipeline()

	for senderID, deviceIDs := range devices {
		key := fmt.Sprintf("devices:%s", senderID)
		members := make([]any, len(deviceIDs))
		for i, d := range deviceIDs {
			members[i] = d
		}
		pipe.SAdd(ctx, key, members...)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("warming up device cache: %w", err)
	}

	log.Printf("device cache warmed: %d senders", len(devices))
	return nil
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

	// Populate cache (best-effort)
	if known {
		r.client.SAdd(ctx, key, deviceID)
	}

	return known, nil
}
