package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
)

type UnitOfWork struct {
	db *sql.DB
}

func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) Begin(ctx context.Context) (domain.TxHandle, error) {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

func (u *UnitOfWork) Commit(handle domain.TxHandle) error {
	w := handle.(*txWrapper)
	return w.tx.Commit()
}

func (u *UnitOfWork) Rollback(handle domain.TxHandle) error {
	w := handle.(*txWrapper)
	return w.tx.Rollback()
}

// txWrapper adapts *sql.Tx to domain.TxHandle.
type txWrapper struct {
	tx *sql.Tx
}

func (w *txWrapper) ExecContext(ctx context.Context, query string, args ...any) (domain.Result, error) {
	return w.tx.ExecContext(ctx, query, args...)
}

func (w *txWrapper) QueryRowContext(ctx context.Context, query string, args ...any) domain.Row {
	return w.tx.QueryRowContext(ctx, query, args...)
}
