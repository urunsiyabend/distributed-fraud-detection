package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

type RouterDeps struct {
	Handler     *Handler
	Tracer      trace.Tracer
	Logger      *slog.Logger
	Registry    *prometheus.Registry
	ReadyCheck  func() bool
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Middleware chain: Tracing → RequestID → Logging → Recovery
	r.Use(Tracing(deps.Tracer))
	r.Use(RequestID)
	r.Use(Logging(deps.Logger))
	r.Use(Recovery(deps.Logger))

	// Application routes
	r.Post("/v1/transactions/assess", deps.Handler.AssessTransaction)

	// Operational routes
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	})

	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if deps.ReadyCheck() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
	})

	r.Get("/metrics", promhttp.HandlerFor(deps.Registry, promhttp.HandlerOpts{}).ServeHTTP)

	return r
}
