package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/domain"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Save(ctx context.Context, tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (id, sender_id, receiver_id, amount, currency, status, device_id, ip, lat, lng, payment_method, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := r.db.ExecContext(ctx, query,
		tx.ID, tx.SenderID, tx.ReceiverID,
		tx.Amount.Amount, tx.Amount.Currency,
		string(tx.Status), tx.DeviceID, tx.IP,
		tx.Location.Lat, tx.Location.Lng,
		string(tx.PaymentMethod),
		tx.CreatedAt, tx.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving transaction %s: %w", tx.ID, err)
	}
	return nil
}

func (r *TransactionRepository) FindByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `
		SELECT id, sender_id, receiver_id, amount, currency, status, device_id, ip, lat, lng, payment_method, fraud_decision, fraud_score, created_at, updated_at
		FROM transactions WHERE id = $1`

	var tx domain.Transaction
	var fraudDecision sql.NullString
	var fraudScore sql.NullInt32

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID, &tx.SenderID, &tx.ReceiverID,
		&tx.Amount.Amount, &tx.Amount.Currency,
		&tx.Status, &tx.DeviceID, &tx.IP,
		&tx.Location.Lat, &tx.Location.Lng,
		&tx.PaymentMethod,
		&fraudDecision, &fraudScore,
		&tx.CreatedAt, &tx.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying transaction %s: %w", id, err)
	}

	if fraudDecision.Valid {
		tx.FraudDecision = &fraudDecision.String
	}
	if fraudScore.Valid {
		score := int(fraudScore.Int32)
		tx.FraudScore = &score
	}

	return &tx, nil
}

func (r *TransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	query := `
		UPDATE transactions SET status = $2, fraud_decision = $3, fraud_score = $4, updated_at = $5
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, tx.ID, string(tx.Status), tx.FraudDecision, tx.FraudScore, tx.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updating transaction %s: %w", tx.ID, err)
	}
	return nil
}
