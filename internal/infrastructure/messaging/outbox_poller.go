package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"distributed-fraud-detection/internal/domain"
	"distributed-fraud-detection/internal/infrastructure/postgres"

	"github.com/nats-io/nats.go"
)

const (
	outboxBatchSize  = 100
	outboxMaxRetries = 3
)

type OutboxPoller struct {
	outbox  *postgres.OutboxRepository
	conn    *nats.Conn
	metrics domain.OutboxMetrics
	logger  *slog.Logger
}

func NewOutboxPoller(
	outbox *postgres.OutboxRepository,
	conn *nats.Conn,
	metrics domain.OutboxMetrics,
	logger *slog.Logger,
) *OutboxPoller {
	return &OutboxPoller{
		outbox:  outbox,
		conn:    conn,
		metrics: metrics,
		logger:  logger,
	}
}

func (p *OutboxPoller) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	p.logger.Info("outbox poller started")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox poller stopping")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *OutboxPoller) poll(ctx context.Context) {
	entries, err := p.outbox.GetPending(ctx, outboxBatchSize)
	if err != nil {
		p.logger.ErrorContext(ctx, "outbox poll failed", slog.String("error", err.Error()))
		return
	}

	if len(entries) == 0 {
		return
	}

	p.metrics.OutboxPending(len(entries))

	for _, entry := range entries {
		p.processEntry(ctx, entry)
	}
}

func (p *OutboxPoller) processEntry(ctx context.Context, entry postgres.OutboxEntry) {
	if entry.RetryCount >= outboxMaxRetries {
		p.handleDead(ctx, entry)
		return
	}

	subject := fmt.Sprintf("fraud.assessment.%s", entry.EventType)

	if err := p.conn.Publish(subject, entry.Payload); err != nil {
		p.logger.WarnContext(ctx, "outbox publish failed",
			slog.String("outbox_id", entry.ID),
			slog.String("event_type", entry.EventType),
			slog.String("error", err.Error()),
		)
		p.outbox.MarkFailed(ctx, entry.ID, err)
		return
	}

	if err := p.outbox.MarkPublished(ctx, entry.ID); err != nil {
		p.logger.ErrorContext(ctx, "outbox mark published failed",
			slog.String("outbox_id", entry.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	p.metrics.OutboxPublished()
}

func (p *OutboxPoller) handleDead(ctx context.Context, entry postgres.OutboxEntry) {
	// Publish to DLQ before marking dead
	dlqSubject := "fraud.assessment.dlq"
	p.conn.Publish(dlqSubject, entry.Payload)

	if err := p.outbox.MarkDead(ctx, entry.ID); err != nil {
		p.logger.ErrorContext(ctx, "outbox mark dead failed",
			slog.String("outbox_id", entry.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	p.logger.WarnContext(ctx, "outbox entry moved to DLQ",
		slog.String("outbox_id", entry.ID),
		slog.String("event_type", entry.EventType),
		slog.Int("retry_count", entry.RetryCount),
	)

	p.metrics.OutboxDead()
}
