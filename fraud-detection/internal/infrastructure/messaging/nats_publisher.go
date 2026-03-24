package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	conn   *nats.Conn
	logger *slog.Logger
}

func NewNATSPublisher(conn *nats.Conn, logger *slog.Logger) *NATSPublisher {
	return &NATSPublisher{conn: conn, logger: logger}
}

func (p *NATSPublisher) Publish(ctx context.Context, event domain.DomainEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event %s: %w", event.EventName(), err)
	}

	subject := fmt.Sprintf("fraud.assessment.%s", event.EventName())

	if err := p.conn.Publish(subject, data); err != nil {
		p.logger.ErrorContext(ctx, "NATS publish failed",
			slog.String("subject", subject),
			slog.String("event", event.EventName()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("publishing to %s: %w", subject, err)
	}

	p.logger.DebugContext(ctx, "event published",
		slog.String("subject", subject),
		slog.String("event", event.EventName()),
	)

	return nil
}
