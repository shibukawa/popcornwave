package pwruntime

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Transaction executes fn with the active transaction stored in its context.
// Nested calls initially reuse the existing transaction.
func Transaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if fn == nil {
		return errors.New("popcornwave: nil transaction callback")
	}
	if executor, lookupErr := sqlbind.SQLExecutorFromContext(ctx); lookupErr == nil {
		if _, nested := executor.(*sql.Tx); nested {
			return fn(ctx)
		}
	}
	db, ok := DB(ctx)
	if !ok {
		return errors.New("popcornwave: database is not available in context")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txctx := sqlbind.WithSQLExecutor(ctx, tx)
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
	}()
	if err := fn(txctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
