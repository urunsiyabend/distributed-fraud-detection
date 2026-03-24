package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/application"
	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *application.TransactionService
}

func NewHandler(svc *application.TransactionService) *Handler {
	return &Handler{svc: svc}
}

type CreateTransactionRequest struct {
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	DeviceID      string  `json:"device_id"`
	IP            string  `json:"ip"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	PaymentMethod string  `json:"payment_method"`
}

type TransactionResponse struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	FraudDecision *string `json:"fraud_decision,omitempty"`
	FraudScore    *int    `json:"fraud_score,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	tx, err := domain.NewTransaction(
		uuid.New().String(),
		req.SenderID, req.ReceiverID,
		domain.Money{Amount: req.Amount, Currency: req.Currency},
		req.DeviceID, req.IP,
		domain.Coordinate{Lat: req.Lat, Lng: req.Lng},
		domain.PaymentMethod(req.PaymentMethod),
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.svc.CreateTransaction(r.Context(), tx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "transaction failed"})
		return
	}

	writeJSON(w, http.StatusCreated, toResponse(tx))
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tx, err := h.svc.GetTransaction(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, toResponse(tx))
}

func (h *Handler) CompleteMFA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.svc.CompleteMFA(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tx, _ := h.svc.GetTransaction(r.Context(), id)
	writeJSON(w, http.StatusOK, toResponse(tx))
}

func toResponse(tx *domain.Transaction) TransactionResponse {
	return TransactionResponse{
		ID:            tx.ID,
		Status:        string(tx.Status),
		Amount:        tx.Amount.Amount,
		Currency:      tx.Amount.Currency,
		SenderID:      tx.SenderID,
		ReceiverID:    tx.ReceiverID,
		FraudDecision: tx.FraudDecision,
		FraudScore:    tx.FraudScore,
		CreatedAt:     tx.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
