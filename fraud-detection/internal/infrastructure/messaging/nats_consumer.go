package messaging

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"

	"github.com/nats-io/nats.go"
)

const (
	SubjectCompleted = "fraud.assessment.assessment.completed"
	SubjectDLQ       = "fraud.assessment.dlq"
	QueueGroup       = "fraud-workers"
	MaxRedelivery    = 3
)

type NATSConsumer struct {
	conn   *nats.Conn
	logger *slog.Logger
	sub    *nats.Subscription
}

func NewNATSConsumer(conn *nats.Conn, logger *slog.Logger) *NATSConsumer {
	return &NATSConsumer{conn: conn, logger: logger}
}

type MessageHandler func(event domain.AssessmentCompletedEvent) error

func (c *NATSConsumer) Subscribe(handler MessageHandler) error {
	sub, err := c.conn.QueueSubscribe(SubjectCompleted, QueueGroup, func(msg *nats.Msg) {
		c.handleMessage(msg, handler)
	})
	if err != nil {
		return fmt.Errorf("subscribing to %s: %w", SubjectCompleted, err)
	}
	c.sub = sub
	return nil
}

func (c *NATSConsumer) handleMessage(msg *nats.Msg, handler MessageHandler) {
	var event domain.AssessmentCompletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.logger.Error("failed to unmarshal event",
			slog.String("subject", msg.Subject),
			slog.String("error", err.Error()),
		)
		c.sendToDLQ(msg, "unmarshal_error", err)
		return
	}

	redeliveryCount := getRedeliveryCount(msg)

	if err := handler(event); err != nil {
		c.logger.Warn("message processing failed",
			slog.String("transaction_id", event.TransactionID),
			slog.Int("redelivery_count", redeliveryCount),
			slog.String("error", err.Error()),
		)

		if redeliveryCount >= MaxRedelivery {
			c.sendToDLQ(msg, "max_redelivery", err)
			return
		}

		if msg.Sub != nil {
			msg.Nak()
		}
		return
	}

	msg.Ack()
}

func (c *NATSConsumer) sendToDLQ(msg *nats.Msg, reason string, err error) {
	dlqPayload, _ := json.Marshal(map[string]any{
		"original_subject": msg.Subject,
		"original_data":    string(msg.Data),
		"reason":           reason,
		"error":            err.Error(),
		"timestamp":        time.Now(),
	})

	if pubErr := c.conn.Publish(SubjectDLQ, dlqPayload); pubErr != nil {
		c.logger.Error("failed to publish to DLQ",
			slog.String("error", pubErr.Error()),
		)
	}

	msg.Ack() // ack original to stop redelivery
}

func (c *NATSConsumer) Drain() error {
	if c.sub != nil {
		return c.sub.Drain()
	}
	return nil
}

func getRedeliveryCount(msg *nats.Msg) int {
	if msg.Header == nil {
		return 0
	}
	// NATS JetStream uses Nats-Num-Delivered header
	// For core NATS, we track via header manually
	val := msg.Header.Get("Nats-Num-Delivered")
	if val == "" {
		return 0
	}
	count := 0
	fmt.Sscanf(val, "%d", &count)
	return count
}
