package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
)

type OutboxEntry struct {
	ID         string
	EventType  string
	Payload    []byte
	Status     string
	CreatedAt  time.Time
	RetryCount int
	LastError  string
}

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) SaveWithinTx(ctx context.Context, tx domain.TxHandle, event domain.DomainEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event %s: %w", event.EventName(), err)
	}

	query := `INSERT INTO outbox (event_type, payload) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, query, event.EventName(), payload)
	if err != nil {
		return fmt.Errorf("inserting outbox entry: %w", err)
	}

	return nil
}

func (r *OutboxRepository) GetPending(ctx context.Context, limit int) ([]OutboxEntry, error) {
	query := `
		SELECT id, event_type, payload, status, created_at, retry_count, COALESCE(last_error, '')
		FROM outbox
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Status, &e.CreatedAt, &e.RetryCount, &e.LastError); err != nil {
			return nil, fmt.Errorf("scanning outbox entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `UPDATE outbox SET status = 'published', published_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry %s as published: %w", id, err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, publishErr error) error {
	query := `UPDATE outbox SET retry_count = retry_count + 1, last_error = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, publishErr.Error())
	if err != nil {
		return fmt.Errorf("marking outbox entry %s as failed: %w", id, err)
	}
	return nil
}

func (r *OutboxRepository) MarkDead(ctx context.Context, id string) error {
	query := `UPDATE outbox SET status = 'dead' WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry %s as dead: %w", id, err)
	}
	return nil
}
