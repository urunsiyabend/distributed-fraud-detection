package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/postgres"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxRepository_SaveAndGetPending(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	outbox := postgres.NewOutboxRepository(db)
	uow := postgres.NewUnitOfWork(db)

	event := domain.AssessmentCompletedEvent{
		TransactionID: "tx-outbox-1",
		Decision:      domain.DecisionApproved,
		RiskScore:     domain.RiskScore{Value: 30},
		Timestamp:     time.Now(),
	}

	// Save within transaction
	tx, err := uow.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, outbox.SaveWithinTx(ctx, tx, event))
	require.NoError(t, uow.Commit(tx))

	// Get pending
	entries, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "assessment.completed", entries[0].EventType)
	assert.Equal(t, "pending", entries[0].Status)
	assert.Equal(t, 0, entries[0].RetryCount)
}

func TestOutboxRepository_MarkPublished(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	outbox := postgres.NewOutboxRepository(db)
	uow := postgres.NewUnitOfWork(db)

	event := domain.AssessmentCompletedEvent{
		TransactionID: "tx-outbox-pub",
		Decision:      domain.DecisionBlocked,
		RiskScore:     domain.RiskScore{Value: 85},
		Timestamp:     time.Now(),
	}

	tx, err := uow.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, outbox.SaveWithinTx(ctx, tx, event))
	require.NoError(t, uow.Commit(tx))

	entries, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, outbox.MarkPublished(ctx, entries[0].ID))

	// No more pending
	pending, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)

	// Verify status in DB
	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM outbox WHERE id = $1`, entries[0].ID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "published", status)
}

func TestOutboxRepository_MarkFailed(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	outbox := postgres.NewOutboxRepository(db)
	uow := postgres.NewUnitOfWork(db)

	event := domain.AssessmentCompletedEvent{
		TransactionID: "tx-outbox-fail",
		Decision:      domain.DecisionReview,
		RiskScore:     domain.RiskScore{Value: 50},
		Timestamp:     time.Now(),
	}

	tx, err := uow.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, outbox.SaveWithinTx(ctx, tx, event))
	require.NoError(t, uow.Commit(tx))

	entries, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Fail 3 times
	for i := 0; i < 3; i++ {
		require.NoError(t, outbox.MarkFailed(ctx, entries[0].ID, errors.New("nats down")))
	}

	// Verify retry count
	var retryCount int
	var lastError string
	err = db.QueryRowContext(ctx, `SELECT retry_count, last_error FROM outbox WHERE id = $1`, entries[0].ID).Scan(&retryCount, &lastError)
	require.NoError(t, err)
	assert.Equal(t, 3, retryCount)
	assert.Equal(t, "nats down", lastError)
}

func TestOutboxRepository_MarkDead(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	outbox := postgres.NewOutboxRepository(db)
	uow := postgres.NewUnitOfWork(db)

	event := domain.AssessmentCompletedEvent{
		TransactionID: "tx-outbox-dead",
		Decision:      domain.DecisionApproved,
		RiskScore:     domain.RiskScore{Value: 10},
		Timestamp:     time.Now(),
	}

	tx, err := uow.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, outbox.SaveWithinTx(ctx, tx, event))
	require.NoError(t, uow.Commit(tx))

	entries, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, outbox.MarkDead(ctx, entries[0].ID))

	// Not in pending anymore
	pending, err := outbox.GetPending(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)

	// Status is dead
	var status string
	err = db.QueryRowContext(ctx, `SELECT status FROM outbox WHERE id = $1`, entries[0].ID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "dead", status)
}

func TestOutboxRepository_ForUpdateSkipLocked(t *testing.T) {
	t.Parallel()
	db := testutil.StartPostgres(t)
	ctx := context.Background()

	outbox := postgres.NewOutboxRepository(db)
	uow := postgres.NewUnitOfWork(db)

	// Insert 2 entries
	for _, txID := range []string{"tx-lock-1", "tx-lock-2"} {
		event := domain.AssessmentCompletedEvent{
			TransactionID: txID,
			Decision:      domain.DecisionApproved,
			RiskScore:     domain.RiskScore{Value: 10},
			Timestamp:     time.Now(),
		}
		tx, err := uow.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, outbox.SaveWithinTx(ctx, tx, event))
		require.NoError(t, uow.Commit(tx))
	}

	// Two concurrent GetPending calls — each should get different rows due to SKIP LOCKED.
	// We simulate this by holding a transaction open while the other queries.
	dbTx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer dbTx.Rollback()

	// First reader locks the rows
	rows1, err := dbTx.QueryContext(ctx, `
		SELECT id FROM outbox WHERE status = 'pending'
		ORDER BY created_at ASC LIMIT 10
		FOR UPDATE SKIP LOCKED`)
	require.NoError(t, err)

	var lockedIDs []string
	for rows1.Next() {
		var id string
		require.NoError(t, rows1.Scan(&id))
		lockedIDs = append(lockedIDs, id)
	}
	rows1.Close()
	require.Len(t, lockedIDs, 2)

	// Second reader on a separate connection should get 0 rows (all locked)
	var wg sync.WaitGroup
	var secondCount int
	wg.Add(1)
	go func() {
		defer wg.Done()
		entries, err := outbox.GetPending(ctx, 10)
		if err == nil {
			secondCount = len(entries)
		}
	}()
	wg.Wait()

	assert.Equal(t, 0, secondCount)
}
