package domain

import "context"

type TransactionRepository interface {
	Save(ctx context.Context, tx *Transaction) error
	FindByID(ctx context.Context, id string) (*Transaction, error)
	Update(ctx context.Context, tx *Transaction) error
}

type FraudChecker interface {
	Check(ctx context.Context, tx *Transaction) (decision string, score int, reasons []string, err error)
}
