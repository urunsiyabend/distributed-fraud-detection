package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

type ConfigRepository struct {
	db *sql.DB
}

func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

func (r *ConfigRepository) GetFloat(ctx context.Context, key string) (float64, error) {
	val, err := r.getValue(ctx, key)
	if err != nil {
		return 0, err
	}

	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("config key %s: cannot parse %q as float: %w", key, val, err)
	}

	return f, nil
}

func (r *ConfigRepository) GetInt(ctx context.Context, key string) (int, error) {
	val, err := r.getValue(ctx, key)
	if err != nil {
		return 0, err
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("config key %s: cannot parse %q as int: %w", key, val, err)
	}

	return i, nil
}

func (r *ConfigRepository) LoadAll(ctx context.Context) (map[string]string, error) {
	query := `SELECT key, value FROM config`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying all config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scanning config row: %w", err)
		}
		result[k] = v
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating config rows: %w", err)
	}

	return result, nil
}

func (r *ConfigRepository) getValue(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM config WHERE key = $1`

	var val string
	err := r.db.QueryRowContext(ctx, query, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config key %s not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("querying config key %s: %w", key, err)
	}

	return val, nil
}
