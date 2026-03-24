package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrConfigNotFound = errors.New("config key not found")
	ErrCircuitOpen    = errors.New("circuit breaker is open")
)

// CircuitBreakerMetrics tracks circuit breaker state transitions.
type CircuitBreakerMetrics interface {
	CircuitBreakerStateChange(name string, from string, to string)
}

// TransactionCounter counts recent transactions for velocity checks.
type TransactionCounter interface {
	CountBySender(ctx context.Context, senderID string, since time.Time) (int, error)
}

// DeviceRepository provides device trust information.
type DeviceRepository interface {
	IsKnownDevice(ctx context.Context, senderID string, deviceID string) (bool, error)
}

// ConfigRepository provides rule configuration thresholds.
type ConfigRepository interface {
	GetFloat(ctx context.Context, key string) (float64, error)
	GetInt(ctx context.Context, key string) (int, error)
}

// ConfigSource loads raw config from a persistent store.
type ConfigSource interface {
	LoadAll(ctx context.Context) (map[string]string, error)
}

// IdempotencyStore provides idempotency key deduplication.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte) error
}

// EventPublisher publishes domain events to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, event DomainEvent) error
}

// OutboxMetrics tracks outbox poller health.
type OutboxMetrics interface {
	OutboxPending(count int)
	OutboxPublished()
	OutboxDead()
}

// AssessmentRepository persists fraud assessments.
type AssessmentRepository interface {
	SaveWithTx(ctx context.Context, tx TxHandle, assessment FraudAssessment) error
}

// TxHandle abstracts a database transaction for outbox pattern.
type TxHandle interface {
	ExecContext(ctx context.Context, query string, args ...any) (Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) Row
}

// Result abstracts sql.Result.
type Result interface {
	RowsAffected() (int64, error)
}

// Row abstracts sql.Row.
type Row interface {
	Scan(dest ...any) error
}

// UnitOfWork manages atomic DB transactions.
type UnitOfWork interface {
	Begin(ctx context.Context) (TxHandle, error)
	Commit(tx TxHandle) error
	Rollback(tx TxHandle) error
}

// LocationRepository provides last known location for a sender.
type LocationRepository interface {
	GetLastLocation(ctx context.Context, senderID string) (Coordinate, error)
}

// WebhookNotifier sends webhook notifications for fraud decisions.
type WebhookNotifier interface {
	Notify(ctx context.Context, transactionID string, decision Decision, riskScore RiskScore) error
}

// WorkerMetrics tracks worker pool health.
type WorkerMetrics interface {
	WorkerPanic(workerID int)
	WorkerMessageProcessed(success bool)
	WorkerDLQ(transactionID string)
}

// ConfigMetrics tracks config cache health.
type ConfigMetrics interface {
	ConfigRefreshSuccess()
	ConfigRefreshError()
}
