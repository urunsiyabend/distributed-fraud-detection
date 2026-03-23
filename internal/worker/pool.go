package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"distributed-fraud-detection/internal/application"
	"distributed-fraud-detection/internal/domain"
	"distributed-fraud-detection/internal/infrastructure/messaging"
)

type Pool struct {
	numWorkers int
	consumer   *messaging.NATSConsumer
	assessor   *application.SlowPathAssessor
	metrics    domain.WorkerMetrics
	logger     *slog.Logger
	wg         sync.WaitGroup
}

type PoolDeps struct {
	NumWorkers int
	Consumer   *messaging.NATSConsumer
	Assessor   *application.SlowPathAssessor
	Metrics    domain.WorkerMetrics
	Logger     *slog.Logger
}

func NewPool(deps PoolDeps) *Pool {
	return &Pool{
		numWorkers: deps.NumWorkers,
		consumer:   deps.Consumer,
		assessor:   deps.Assessor,
		metrics:    deps.Metrics,
		logger:     deps.Logger,
	}
}

func (p *Pool) Start(ctx context.Context) error {
	err := p.consumer.Subscribe(func(event domain.AssessmentCompletedEvent) error {
		return p.processWithRecovery(ctx, event)
	})
	if err != nil {
		return fmt.Errorf("subscribing consumer: %w", err)
	}

	p.logger.Info("worker pool started",
		slog.Int("num_workers", p.numWorkers),
	)

	// NATS handles concurrency via the subscription's internal goroutine pool.
	// We use WaitGroup to track our own processing goroutines for graceful shutdown.
	p.wg.Add(p.numWorkers)
	for i := 0; i < p.numWorkers; i++ {
		go p.runWorker(ctx, i)
	}

	return nil
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()

	p.logger.Info("worker started", slog.Int("worker_id", id))

	<-ctx.Done()

	p.logger.Info("worker stopping", slog.Int("worker_id", id))
}

func (p *Pool) processWithRecovery(ctx context.Context, event domain.AssessmentCompletedEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("worker panic recovered",
				slog.String("transaction_id", event.TransactionID),
				slog.String("panic", fmt.Sprintf("%v", r)),
			)
			p.metrics.WorkerPanic(0)
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	if err := p.assessor.Process(ctx, event); err != nil {
		p.metrics.WorkerMessageProcessed(false)
		return err
	}

	p.metrics.WorkerMessageProcessed(true)
	return nil
}

func (p *Pool) Shutdown() {
	p.logger.Info("draining worker pool")

	if err := p.consumer.Drain(); err != nil {
		p.logger.Error("consumer drain failed", slog.String("error", err.Error()))
	}

	p.wg.Wait()
	p.logger.Info("worker pool stopped")
}
