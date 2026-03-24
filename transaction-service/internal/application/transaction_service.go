package application

import (
	"context"
	"fmt"

	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/domain"
)

type TransactionService struct {
	repo    domain.TransactionRepository
	fraud   domain.FraudChecker
}

func NewTransactionService(repo domain.TransactionRepository, fraud domain.FraudChecker) *TransactionService {
	return &TransactionService{repo: repo, fraud: fraud}
}

func (s *TransactionService) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	if err := s.repo.Save(ctx, tx); err != nil {
		return fmt.Errorf("saving transaction: %w", err)
	}

	if err := tx.TransitionTo(domain.StatusPendingFraudCheck); err != nil {
		return fmt.Errorf("transitioning to pending: %w", err)
	}
	if err := s.repo.Update(ctx, tx); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	decision, score, _, err := s.fraud.Check(ctx, tx)
	if err != nil {
		return fmt.Errorf("fraud check: %w", err)
	}

	if err := tx.ApplyFraudDecision(decision, score); err != nil {
		return fmt.Errorf("applying fraud decision: %w", err)
	}

	if err := s.repo.Update(ctx, tx); err != nil {
		return fmt.Errorf("updating after fraud: %w", err)
	}

	return nil
}

func (s *TransactionService) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *TransactionService) CompleteMFA(ctx context.Context, id string) error {
	tx, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if tx.Status != domain.StatusPendingMFA && tx.Status != domain.StatusReview {
		return fmt.Errorf("transaction %s not in MFA state (current: %s)", id, tx.Status)
	}

	if err := tx.TransitionTo(domain.StatusApproved); err != nil {
		return fmt.Errorf("approving after MFA: %w", err)
	}

	return s.repo.Update(ctx, tx)
}
