package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
)

// withTransaction creates a new transaction and handles rollback/commit based on the
// error object returned by the `TxFn` or when it panics.
func (db *dB) withTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, beginErr := db.Pool.Begin(ctx)
	if beginErr != nil {
		return fmt.Errorf("tx error: %w", beginErr)
	}

	defer func() {
		if p := recover(); p != nil {
			rollErr := tx.Rollback(ctx)
			if rollErr != nil {
				return
			}
			panic(p)
		}
	}()

	callErr := fn(tx)

	if callErr != nil {
		rollErr := tx.Rollback(ctx)
		if rollErr != nil {
			// return the call (root cause) error and not transaction error
			return fmt.Errorf("tx rollback error: %s, cause: %w", rollErr.Error(), callErr)
		}
		return fmt.Errorf("tx error: %w", callErr)
	}

	commitErr := tx.Commit(ctx)
	if commitErr != nil {
		return fmt.Errorf("db commit error: %w", commitErr)
	}

	return nil
}
