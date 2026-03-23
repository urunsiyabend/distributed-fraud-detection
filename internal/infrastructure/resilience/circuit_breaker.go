package resilience

import (
	"context"
	"fmt"
	"log"
	"time"

	"distributed-fraud-detection/internal/domain"

	"github.com/sony/gobreaker/v2"
)

func newBreaker(name string, metrics domain.CircuitBreakerMetrics) *gobreaker.CircuitBreaker[any] {
	return gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("circuit breaker %s: %s → %s", name, from.String(), to.String())
			metrics.CircuitBreakerStateChange(name, from.String(), to.String())
		},
	})
}

func wrapError(err error) error {
	if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
		return fmt.Errorf("%w: %v", domain.ErrCircuitOpen, err)
	}
	return err
}

// CircuitBreakerTransactionCounter wraps domain.TransactionCounter with a circuit breaker.
type CircuitBreakerTransactionCounter struct {
	inner   domain.TransactionCounter
	breaker *gobreaker.CircuitBreaker[any]
}

func NewCircuitBreakerTransactionCounter(
	inner domain.TransactionCounter,
	metrics domain.CircuitBreakerMetrics,
) *CircuitBreakerTransactionCounter {
	return &CircuitBreakerTransactionCounter{
		inner:   inner,
		breaker: newBreaker("transaction-counter", metrics),
	}
}

func (c *CircuitBreakerTransactionCounter) CountBySender(ctx context.Context, senderID string, since time.Time) (int, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		return c.inner.CountBySender(ctx, senderID, since)
	})
	if err != nil {
		return 0, wrapError(err)
	}
	return result.(int), nil
}

// CircuitBreakerDeviceRepository wraps domain.DeviceRepository with a circuit breaker.
type CircuitBreakerDeviceRepository struct {
	inner   domain.DeviceRepository
	breaker *gobreaker.CircuitBreaker[any]
}

func NewCircuitBreakerDeviceRepository(
	inner domain.DeviceRepository,
	metrics domain.CircuitBreakerMetrics,
) *CircuitBreakerDeviceRepository {
	return &CircuitBreakerDeviceRepository{
		inner:   inner,
		breaker: newBreaker("device-repository", metrics),
	}
}

func (c *CircuitBreakerDeviceRepository) IsKnownDevice(ctx context.Context, senderID string, deviceID string) (bool, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		return c.inner.IsKnownDevice(ctx, senderID, deviceID)
	})
	if err != nil {
		return false, wrapError(err)
	}
	return result.(bool), nil
}
