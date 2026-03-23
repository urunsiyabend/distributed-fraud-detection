package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-fraud-detection/internal/api"
	"distributed-fraud-detection/internal/application"
	"distributed-fraud-detection/internal/domain"
	infraConfig "distributed-fraud-detection/internal/infrastructure/config"
	"distributed-fraud-detection/internal/infrastructure/messaging"
	"distributed-fraud-detection/internal/infrastructure/observability"
	"distributed-fraud-detection/internal/infrastructure/postgres"
	infraRedis "distributed-fraud-detection/internal/infrastructure/redis"
	"distributed-fraud-detection/internal/infrastructure/resilience"
	"distributed-fraud-detection/internal/worker"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
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

	// --- Logger ---
	slogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slogLogger := slog.New(slogHandler).With(
		slog.String("service", "fraud-detection"),
		slog.String("env", "production"),
	)

	// --- Tracer (optional — non-fatal if collector unavailable) ---
	otelEndpoint := envOrDefault("OTEL_ENDPOINT", "localhost:4317")
	tracer, err := observability.NewTracer(ctx, "fraud-detection", otelEndpoint)
	if err != nil {
		log.Printf("tracer init failed (continuing without tracing): %v", err)
		tracer = observability.NewNoopTracer()
	}
	defer tracer.Shutdown(ctx)

	// --- Prometheus ---
	reg := prometheus.NewRegistry()
	metrics, err := observability.NewPrometheusMetrics(reg)
	if err != nil {
		log.Fatalf("metrics init: %v", err)
	}

	// --- Postgres ---
	pgDSN := envOrDefault("POSTGRES_DSN", "postgres://user:pass@localhost:5432/fraud?sslmode=disable")
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("postgres open: %v", err)
	}
	defer db.Close()

	// --- Redis ---
	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
	rdb := goRedis.NewClient(&goRedis.Options{Addr: redisAddr})
	defer rdb.Close()

	// --- NATS ---
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	// --- Config: Postgres source → async cache ---
	configSource := postgres.NewConfigRepository(db)
	configCache, err := infraConfig.NewAsyncConfigCache(ctx, configSource, metrics, 60*time.Second)
	if err != nil {
		log.Fatalf("config cache init: %v", err)
	}

	// --- Ports (raw) ---
	rawCounter := infraRedis.NewTransactionCounter(rdb)
	rawDeviceRepo := postgres.NewDeviceRepository(db)

	// --- Circuit breakers ---
	txCounter := resilience.NewCircuitBreakerTransactionCounter(rawCounter, metrics)
	deviceRepo := resilience.NewCircuitBreakerDeviceRepository(rawDeviceRepo, metrics)

	// --- Idempotency ---
	idempotencyStore := infraRedis.NewIdempotencyStore(rdb, 24*time.Hour)

	// --- Messaging ---
	publisher := messaging.NewNATSPublisher(nc, slogLogger)
	consumer := messaging.NewNATSConsumer(nc, slogLogger)

	// --- Postgres repositories ---
	uow := postgres.NewUnitOfWork(db)
	assessmentRepo := postgres.NewAssessmentRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	// --- Application: fast path ---
	factory := application.NewFraudRuleFactory(txCounter, deviceRepo, configCache)
	assessor := application.NewFraudAssessor(factory, metrics, metrics, tracer.TracerForApp(), time.Now)

	// --- Application: slow path ---
	slowAssessor := application.NewSlowPathAssessor(application.SlowPathDeps{
		LocationRepo: &noopLocationRepo{},
		Config:       configCache,
		Publisher:    publisher,
		Notifier:     &noopWebhookNotifier{},
		Idempotency:  idempotencyStore,
		RuleMetrics:  metrics,
		Logger:       slogLogger,
		Now:          time.Now,
	})

	// --- Worker pool ---
	pool := worker.NewPool(worker.PoolDeps{
		NumWorkers: 4,
		Consumer:   consumer,
		Assessor:   slowAssessor,
		Metrics:    metrics,
		Logger:     slogLogger,
	})

	if err := pool.Start(ctx); err != nil {
		log.Fatalf("worker pool start: %v", err)
	}

	// --- Outbox poller ---
	outboxPoller := messaging.NewOutboxPoller(outboxRepo, nc, metrics, slogLogger)
	go outboxPoller.Start(ctx)

	// --- HTTP handler + router ---
	handler := api.NewHandler(api.HandlerDeps{
		Assessor:    assessor,
		UoW:         uow,
		Assessments: assessmentRepo,
		Outbox:      outboxRepo,
		Idempotency: idempotencyStore,
		Now:         time.Now,
	})
	router := api.NewRouter(api.RouterDeps{
		Handler:    handler,
		Tracer:     tracer.TracerForApp(),
		Logger:     slogLogger,
		Registry:   reg,
		ReadyCheck: configCache.IsReady,
	})

	server := &http.Server{Addr: ":8080", Handler: router}

	go func() {
		log.Printf("HTTP server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	log.Printf("fraud detection system ready (config cache ready: %v)", configCache.IsReady())

	// Block until shutdown signal
	<-ctx.Done()

	// Graceful shutdown: HTTP → workers → outbox poller → connections
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.Shutdown(shutdownCtx)
	pool.Shutdown()

	log.Println("shutting down")
}

// Placeholder implementations — replace with real adapters

type noopLocationRepo struct{}

func (n *noopLocationRepo) GetLastLocation(_ context.Context, _ string) (domain.Coordinate, error) {
	return domain.Coordinate{}, nil
}

type noopWebhookNotifier struct{}

func (n *noopWebhookNotifier) Notify(_ context.Context, _ string, _ domain.Decision, _ domain.RiskScore) error {
	return nil
}
