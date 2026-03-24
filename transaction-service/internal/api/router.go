package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()

	r.Post("/v1/transactions", handler.CreateTransaction)
	r.Get("/v1/transactions/{id}", handler.GetTransaction)
	r.Post("/v1/transactions/{id}/mfa", handler.CompleteMFA)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	})

	return r
}
