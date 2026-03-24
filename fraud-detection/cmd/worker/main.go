package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/application"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
	infraConfig "github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/config"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/messaging"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/observability"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/postgres"
	infraRedis "github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/redis"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/worker"

	"github.com/nats-io/nats.go"
	goRedis "github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slogLogger := slog.New(slogHandler).With(
		slog.String("service", "fraud-worker"),
		slog.String("env", "production"),
	)

	otelEndpoint := envOrDefault("OTEL_ENDPOINT", "localhost:4317")
	tracer, err := observability.NewTracer(ctx, "fraud-worker", otelEndpoint)
	if err != nil {
		log.Printf("tracer init failed: %v", err)
		tracer = observability.NewNoopTracer()
	}
	defer tracer.Shutdown(ctx)
	_ = tracer

	db, err := sql.Open("postgres", envOrDefault("POSTGRES_DSN", "postgres://user:pass@localhost:5432/fraud?sslmode=disable"))
	if err != nil {
		log.Fatalf("postgres open: %v", err)
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	defer db.Close()

	rdb := goRedis.NewClient(&goRedis.Options{
		Addr:         envOrDefault("REDIS_ADDR", "localhost:6379"),
		PoolSize:     20,
		MinIdleConns: 5,
	})
	defer rdb.Close()

	nc, err := nats.Connect(envOrDefault("NATS_URL", "nats://localhost:4222"))
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	configSource := postgres.NewConfigRepository(db)
	configCache, err := infraConfig.NewAsyncConfigCache(ctx, configSource, &noopConfigMetrics{}, 60*time.Second)
	if err != nil {
		log.Fatalf("config cache init: %v", err)
	}

	publisher := messaging.NewNATSPublisher(nc, slogLogger)
	consumer := messaging.NewNATSConsumer(nc, slogLogger)
	idempotencyStore := infraRedis.NewIdempotencyStore(rdb, 24*time.Hour)

	slowAssessor := application.NewSlowPathAssessor(application.SlowPathDeps{
		LocationRepo: &noopLocationRepo{},
		Config:       configCache,
		Publisher:    publisher,
		Notifier:     &noopWebhookNotifier{},
		Idempotency:  idempotencyStore,
		RuleMetrics:  &noopRuleMetrics{},
		Logger:       slogLogger,
		Now:          time.Now,
	})

	pool := worker.NewPool(worker.PoolDeps{
		NumWorkers: 10,
		Consumer:   consumer,
		Assessor:   slowAssessor,
		Metrics:    &noopWorkerMetrics{},
		Logger:     slogLogger,
	})

	if err := pool.Start(ctx); err != nil {
		log.Fatalf("worker pool start: %v", err)
	}

	outboxPoller := messaging.NewOutboxPoller(postgres.NewOutboxRepository(db), nc, &noopOutboxMetrics{}, slogLogger)
	go outboxPoller.Start(ctx)

	log.Println("fraud worker started")
	<-ctx.Done()

	pool.Shutdown()
	log.Println("fraud worker stopped")
}

type noopLocationRepo struct{}

func (n *noopLocationRepo) GetLastLocation(_ context.Context, _ string) (domain.Coordinate, error) {
	return domain.Coordinate{}, nil
}

type noopWebhookNotifier struct{}

func (n *noopWebhookNotifier) Notify(_ context.Context, _ string, _ domain.Decision, _ domain.RiskScore) error {
	return nil
}

type noopConfigMetrics struct{}

func (n *noopConfigMetrics) ConfigRefreshSuccess() {}
func (n *noopConfigMetrics) ConfigRefreshError()   {}

type noopRuleMetrics struct{}

func (n *noopRuleMetrics) RuleFallback(_ string)  {}
func (n *noopRuleMetrics) RuleTriggered(_ string) {}

type noopWorkerMetrics struct{}

func (n *noopWorkerMetrics) WorkerPanic(_ int)              {}
func (n *noopWorkerMetrics) WorkerMessageProcessed(_ bool)  {}
func (n *noopWorkerMetrics) WorkerDLQ(_ string)             {}

type noopOutboxMetrics struct{}

func (n *noopOutboxMetrics) OutboxPending(_ int) {}
func (n *noopOutboxMetrics) OutboxPublished()    {}
func (n *noopOutboxMetrics) OutboxDead()         {}
