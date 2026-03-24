package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"context"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/api"
	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/application"
	fraudGRPC "github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/infrastructure/grpc"
	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/infrastructure/postgres"

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

	// --- Postgres ---
	dsn := envOrDefault("POSTGRES_DSN", "postgres://txn:txn@localhost:5433/transactions?sslmode=disable")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("postgres open: %v", err)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(20)
	defer db.Close()

	// Run migrations
	schema, err := os.ReadFile("internal/infrastructure/postgres/migrations/001_create_transactions.sql")
	if err != nil {
		log.Printf("warning: could not read migration file: %v", err)
	} else {
		if _, err := db.ExecContext(ctx, string(schema)); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
	}

	// --- Fraud gRPC client ---
	fraudAddr := envOrDefault("FRAUD_GRPC_ADDR", "localhost:50051")
	fraudClient, err := fraudGRPC.NewFraudClient(fraudAddr)
	if err != nil {
		log.Fatalf("fraud client: %v", err)
	}

	// --- Wire up ---
	repo := postgres.NewTransactionRepository(db)
	svc := application.NewTransactionService(repo, fraudClient)
	handler := api.NewHandler(svc)
	router := api.NewRouter(handler)

	server := &http.Server{Addr: ":8081", Handler: router}

	go func() {
		log.Printf("transaction-service listening on :8081")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
	log.Println("transaction-service stopped")
}
