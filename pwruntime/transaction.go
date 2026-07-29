package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// ErrSavepointUnsupported reports that the configured driver cannot nest a
// transaction because it has no known savepoint support.
var ErrSavepointUnsupported = errors.New("popcornwave: driver does not support savepoints")

// ErrTransactionFailed reports that a savepoint operation left the transaction
// in an unknown state, so no further work may be committed on it.
var ErrTransactionFailed = errors.New("popcornwave: transaction is no longer usable")

// TransactionScope owns one *sql.Tx and the savepoint stack nested inside it.
// The framework creates a scope; applications only observe it indirectly
// through Transaction.
type TransactionScope struct {
	db     *sql.DB
	driver string

	mu     sync.Mutex
	tx     *sql.Tx
	depth  int
	failed bool
}

// NewTransactionScope prepares an inactive scope over db. Begin activates it.
func NewTransactionScope(db *sql.DB, driver string) *TransactionScope {
	if db == nil {
		return nil
	}
	return &TransactionScope{db: db, driver: driver}
}

// Begin opens the depth 0 transaction of an inactive scope.
func (scope *TransactionScope) Begin(ctx context.Context, options *sql.TxOptions) error {
	if scope == nil {
		return errors.New("popcornwave: nil transaction scope")
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.tx != nil {
		return errors.New("popcornwave: transaction scope is already active")
	}
	tx, err := scope.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	scope.tx = tx
	return nil
}

// Active reports whether a depth 0 transaction is open.
func (scope *TransactionScope) Active() bool {
	if scope == nil {
		return false
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.tx != nil
}

// Commit commits the depth 0 transaction. A scope marked failed by a savepoint
// operation is rolled back instead.
func (scope *TransactionScope) Commit() error {
	if scope == nil {
		return errors.New("popcornwave: nil transaction scope")
	}
	scope.mu.Lock()
	tx, failed := scope.tx, scope.failed
	scope.tx = nil
	scope.mu.Unlock()
	if tx == nil {
		return errors.New("popcornwave: transaction scope is not active")
	}
	if failed {
		return errors.Join(ErrTransactionFailed, tx.Rollback())
	}
	return tx.Commit()
}

// Rollback rolls back the depth 0 transaction. Rolling back an inactive scope
// is not an error, so owners can defer it unconditionally.
func (scope *TransactionScope) Rollback() error {
	if scope == nil {
		return nil
	}
	scope.mu.Lock()
	tx := scope.tx
	scope.tx = nil
	scope.mu.Unlock()
	if tx == nil {
		return nil
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// Tx returns the depth 0 transaction, or nil when the scope is inactive.
//
// It exists for test tooling that must run statements inside the same
// transaction as the requests under test, such as dataset seeding and database
// assertions. Application code uses Transaction instead.
func (scope *TransactionScope) Tx() *sql.Tx {
	if scope == nil {
		return nil
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.tx
}

// state reports whether a transaction is open and how deep its savepoint stack
// is, which is what a query record needs to place a statement.
func (scope *TransactionScope) state() (bool, int) {
	if scope == nil {
		return false, 0
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.tx != nil, scope.depth
}

func (scope *TransactionScope) executor() sqlbind.SQLExecutor {
	if scope == nil {
		return nil
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.tx == nil {
		return nil
	}
	return scope.tx
}

// savepointDrivers lists drivers whose SAVEPOINT, RELEASE SAVEPOINT, and
// ROLLBACK TO SAVEPOINT statements are known to behave as required.
var savepointDrivers = map[string]bool{
	"sqlite":     true,
	"sqlite3":    true,
	"postgres":   true,
	"postgresql": true,
	"pgx":        true,
	"mysql":      true,
}

// SupportsSavepoint reports whether driver may nest transactions.
func SupportsSavepoint(driver string) bool {
	return savepointDrivers[driver]
}

// Transaction executes fn with the active transaction stored in its context.
// The outermost call begins a real transaction; a nested call opens a
// savepoint, so an inner failure rolls back only the inner work and leaves the
// outer transaction usable.
func Transaction(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("popcornwave: nil transaction callback")
	}
	if scope := activeScope(ctx); scope != nil {
		return scope.nested(ctx, fn)
	}
	if scope := adoptExecutorTx(ctx); scope != nil {
		return scope.nested(withScope(ctx, scope), fn)
	}
	db, ok := DB(ctx)
	if !ok {
		return errors.New("popcornwave: database is not available in context")
	}
	scope := NewTransactionScope(db, resources(ctx).DBDriver)
	if err := scope.Begin(ctx, nil); err != nil {
		return err
	}
	txctx := withScope(ctx, scope)
	committed := false
	defer func() {
		if !committed {
			_ = scope.Rollback()
		}
	}()
	if err := fn(txctx); err != nil {
		return err
	}
	committed = true
	return scope.Commit()
}

// adoptExecutorTx wraps a transaction installed as a bare context executor so
// nesting still uses savepoints instead of joining it silently.
func adoptExecutorTx(ctx context.Context) *TransactionScope {
	executor, err := sqlbind.SQLExecutorFromContext(ctx)
	if err != nil {
		return nil
	}
	tx, ok := unwrapExecutor(executor).(*sql.Tx)
	if !ok {
		return nil
	}
	return &TransactionScope{tx: tx, driver: resources(ctx).DBDriver}
}

// nested wraps fn in a savepoint of an already active scope.
func (scope *TransactionScope) nested(ctx context.Context, fn func(context.Context) error) error {
	name, tx, err := scope.push(ctx)
	if err != nil {
		return err
	}
	released := false
	defer func() {
		if !released {
			scope.rollbackTo(ctx, tx, name)
		}
	}()
	if err := fn(ctx); err != nil {
		return err
	}
	released = true
	return scope.release(ctx, tx, name)
}

func (scope *TransactionScope) push(ctx context.Context) (string, *sql.Tx, error) {
	scope.mu.Lock()
	tx, failed, driver := scope.tx, scope.failed, scope.driver
	if tx == nil {
		scope.mu.Unlock()
		return "", nil, errors.New("popcornwave: transaction scope is not active")
	}
	if failed {
		scope.mu.Unlock()
		return "", nil, ErrTransactionFailed
	}
	if !SupportsSavepoint(driver) {
		scope.mu.Unlock()
		return "", nil, fmt.Errorf("%w: %s", ErrSavepointUnsupported, driver)
	}
	scope.depth++
	name := "pw_sp_" + strconv.Itoa(scope.depth)
	scope.mu.Unlock()

	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		scope.pop()
		return "", nil, err
	}
	return name, tx, nil
}

func (scope *TransactionScope) pop() {
	scope.mu.Lock()
	if scope.depth > 0 {
		scope.depth--
	}
	scope.mu.Unlock()
}

func (scope *TransactionScope) release(ctx context.Context, tx *sql.Tx, name string) error {
	defer scope.pop()
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+name); err != nil {
		scope.markFailed()
		return err
	}
	return nil
}

// rollbackTo undoes the savepoint work. A failure here means the transaction
// state is unknown, so the scope is marked failed and can no longer commit.
func (scope *TransactionScope) rollbackTo(ctx context.Context, tx *sql.Tx, name string) {
	defer scope.pop()
	rollbackCtx := ctx
	if ctx.Err() != nil {
		rollbackCtx = context.WithoutCancel(ctx)
	}
	if _, err := tx.ExecContext(rollbackCtx, "ROLLBACK TO SAVEPOINT "+name); err != nil {
		scope.markFailed()
		ReadLogger(ctx).Log(ctx, LevelError, "rollback to savepoint failed", String("savepoint", name), String("error", err.Error()))
		return
	}
	if _, err := tx.ExecContext(rollbackCtx, "RELEASE SAVEPOINT "+name); err != nil {
		scope.markFailed()
		ReadLogger(ctx).Log(ctx, LevelError, "release savepoint failed", String("savepoint", name), String("error", err.Error()))
	}
}

func (scope *TransactionScope) markFailed() {
	scope.mu.Lock()
	scope.failed = true
	scope.mu.Unlock()
}
