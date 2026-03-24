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

// LoadAll returns all known devices grouped by sender.
func (r *DeviceRepository) LoadAll(ctx context.Context) (map[string][]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sender_id, device_id FROM known_devices`)
	if err != nil {
		return nil, fmt.Errorf("querying all known_devices: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var senderID, deviceID string
		if err := rows.Scan(&senderID, &deviceID); err != nil {
			return nil, fmt.Errorf("scanning known_devices row: %w", err)
		}
		result[senderID] = append(result[senderID], deviceID)
	}
	return result, rows.Err()
}
