package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"distributed-fraud-detection/internal/application"
	"distributed-fraud-detection/internal/domain"
	"distributed-fraud-detection/internal/infrastructure/postgres"
)

type Handler struct {
	assessor     *application.FraudAssessor
	uow          domain.UnitOfWork
	assessments  *postgres.AssessmentRepository
	outbox       *postgres.OutboxRepository
	idempotency  domain.IdempotencyStore
	now          func() time.Time
}

type HandlerDeps struct {
	Assessor    *application.FraudAssessor
	UoW         domain.UnitOfWork
	Assessments *postgres.AssessmentRepository
	Outbox      *postgres.OutboxRepository
	Idempotency domain.IdempotencyStore
	Now         func() time.Time
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		assessor:    deps.Assessor,
		uow:        deps.UoW,
		assessments: deps.Assessments,
		outbox:      deps.Outbox,
		idempotency: deps.Idempotency,
		now:         deps.Now,
	}
}

func (h *Handler) AssessTransaction(w http.ResponseWriter, r *http.Request) {
	var req AssessTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON: " + err.Error()})
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation failed", Details: errs})
		return
	}

	// Idempotency check
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey != "" {
		cached, found, err := h.idempotency.Get(r.Context(), idempotencyKey)
		if err == nil && found {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotent-Replay", "true")
			w.WriteHeader(http.StatusAccepted)
			w.Write(cached)
			return
		}
	}

	// Build domain transaction
	money, err := domain.NewMoney(req.Amount, req.Currency)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	coord, _ := domain.NewCoordinate(req.Location.Lat, req.Location.Lng)

	pm, err := domain.NewPaymentMethod(req.PaymentMethod)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	tx, err := domain.NewTransaction(domain.TransactionInput{
		ID:            req.ID,
		Amount:        money,
		SenderID:      req.SenderID,
		ReceiverID:    req.ReceiverID,
		DeviceID:      req.DeviceID,
		IP:            req.IP,
		Location:      coord,
		Timestamp:     req.Timestamp,
		PaymentMethod: pm,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	// Fast path: 20ms timeout
	assessCtx, cancel := context.WithTimeout(r.Context(), 20*time.Millisecond)
	defer cancel()

	assessment, err := h.assessor.Assess(assessCtx, tx)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			resp := AssessTransactionResponse{
				TransactionID: req.ID,
				Decision:      "pending",
				RiskScore:     0,
				Reasons:       []string{"fast path timeout, queued for async assessment"},
				AssessedAt:    h.now(),
				FastPath:      false,
			}
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "assessment failed"})
		return
	}

	// Atomic: save assessment + outbox events in one DB transaction
	if err := h.saveAtomically(r.Context(), assessment); err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to persist assessment"})
		return
	}

	var reasons []string
	for _, rr := range assessment.RuleResults {
		if rr.Triggered {
			reasons = append(reasons, rr.Reason)
		}
	}

	resp := AssessTransactionResponse{
		TransactionID: assessment.TransactionID,
		Decision:      string(assessment.Decision),
		RiskScore:     assessment.RiskScore.Value,
		Reasons:       reasons,
		AssessedAt:    h.now(),
		FastPath:      true,
	}

	// Cache for idempotency
	if idempotencyKey != "" {
		if body, err := json.Marshal(resp); err == nil {
			h.idempotency.Set(r.Context(), idempotencyKey, body)
		}
	}

	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) saveAtomically(ctx context.Context, assessment domain.FraudAssessment) error {
	dbTx, err := h.uow.Begin(ctx)
	if err != nil {
		return err
	}

	if err := h.assessments.SaveWithTx(ctx, dbTx, assessment); err != nil {
		h.uow.Rollback(dbTx)
		return err
	}

	for _, event := range assessment.Events() {
		if err := h.outbox.SaveWithinTx(ctx, dbTx, event); err != nil {
			h.uow.Rollback(dbTx)
			return err
		}
	}

	return h.uow.Commit(dbTx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
