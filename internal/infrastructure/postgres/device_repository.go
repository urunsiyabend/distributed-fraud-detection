package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type DeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) IsKnownDevice(ctx context.Context, senderID string, deviceID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM known_devices WHERE sender_id = $1 AND device_id = $2)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, senderID, deviceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("querying known_devices: %w", err)
	}

	return exists, nil
}
