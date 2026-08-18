package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/shibukawa/popcornweb/database"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// ErrSavepointUnsupported reports that the configured driver cannot nest a
// transaction because it has no known savepoint support.
var ErrSavepointUnsupported = errors.New("popcornweb: driver does not support savepoints")

// ErrTransactionFailed reports that a savepoint operation left the transaction
// in an unknown state, so no further work may be committed on it.
var ErrTransactionFailed = errors.New("popcornweb: transaction is no longer usable")

// ErrCrossGroupTransaction reports a nested transaction naming a different
// connection group than the one already open.
var ErrCrossGroupTransaction = errors.New("popcornweb: transaction cannot span two connection groups")

// TransactionScope owns one open transaction and the savepoint stack nested
// inside it. The transaction is a *sql.Tx on a database/sql connection and a
// native transaction on one that bypasses it; the savepoint stack runs the
// same statements on either. The framework creates a scope; applications only
// observe it indirectly through Transaction.
type TransactionScope struct {
	db     *sql.DB
	native database.NativeDB
	driver string
	// group is the connection group this scope belongs to for its whole life.
	// A nested call cannot move it to another group.
	group string
	// readOnly comes from the selected connection and opens a read-only
	// transaction at depth 0.
	readOnly bool

	mu       sync.Mutex
	tx       *sql.Tx
	nativeTx database.NativeTx
	depth    int
	failed   bool
}

// NewTransactionScope prepares an inactive scope over db. Begin activates it.
func NewTransactionScope(db *sql.DB, driver string) *TransactionScope {
	if db == nil {
		return nil
	}
	return &TransactionScope{db: db, driver: driver}
}

// newConnectionScope prepares an inactive scope over one connection of the set,
// carrying the group and read-only marking that connection was configured with.
func newConnectionScope(connection *Connection) *TransactionScope {
	if connection == nil || (connection.DB == nil && connection.Native == nil) {
		return nil
	}
	return &TransactionScope{
		db:       connection.DB,
		native:   connection.Native,
		driver:   connection.Driver,
		group:    connection.Group,
		readOnly: connection.ReadOnly,
	}
}

// Group is the connection group the scope is bound to.
func (scope *TransactionScope) Group() string {
	if scope == nil {
		return ""
	}
	return scope.group
}

// ReadOnly reports whether the scope runs on a read-only connection.
func (scope *TransactionScope) ReadOnly() bool {
	if scope == nil {
		return false
	}
	return scope.readOnly
}

// Begin opens the depth 0 transaction of an inactive scope.
//
// A nil options on a read-only connection begins a read-only transaction, which
// is the one enforcement the database itself can apply today.
func (scope *TransactionScope) Begin(ctx context.Context, options *sql.TxOptions) error {
	if scope == nil {
		return errors.New("popcornweb: nil transaction scope")
	}
	if options == nil && scope.readOnly {
		options = &sql.TxOptions{ReadOnly: true}
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.tx != nil || scope.nativeTx != nil {
		return errors.New("popcornweb: transaction scope is already active")
	}
	if scope.native != nil {
		nativeOptions := database.NativeTxOptions{}
		if options != nil {
			if options.Isolation != sql.LevelDefault {
				return fmt.Errorf("popcornweb: the native %s path supports no isolation override yet", scope.driver)
			}
			nativeOptions.ReadOnly = options.ReadOnly
		}
		tx, err := scope.native.BeginTx(ctx, nativeOptions)
		if err != nil {
			return err
		}
		scope.nativeTx = tx
		return nil
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
	return scope.tx != nil || scope.nativeTx != nil
}

// Commit commits the depth 0 transaction. A scope marked failed by a savepoint
// operation is rolled back instead.
//
// The native commit is sent on the wire without a caller context, because the
// owner that could carry one may already hold a cancelled context, and a
// commit must not be abandoned by the cancellation that ended the request.
func (scope *TransactionScope) Commit() error {
	if scope == nil {
		return errors.New("popcornweb: nil transaction scope")
	}
	scope.mu.Lock()
	tx, nativeTx, failed := scope.tx, scope.nativeTx, scope.failed
	scope.tx, scope.nativeTx = nil, nil
	scope.mu.Unlock()
	if tx == nil && nativeTx == nil {
		return errors.New("popcornweb: transaction scope is not active")
	}
	if failed {
		if nativeTx != nil {
			return errors.Join(ErrTransactionFailed, nativeTx.Rollback(context.Background()))
		}
		return errors.Join(ErrTransactionFailed, tx.Rollback())
	}
	if nativeTx != nil {
		return nativeTx.Commit(context.Background())
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
	tx, nativeTx := scope.tx, scope.nativeTx
	scope.tx, scope.nativeTx = nil, nil
	scope.mu.Unlock()
	if nativeTx != nil {
		return nativeTx.Rollback(context.Background())
	}
	if tx == nil {
		return nil
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// Tx returns the depth 0 transaction, or nil when the scope is inactive or
// runs on a native connection, whose transaction is no *sql.Tx.
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

// ActiveExecutor returns the statement surface of the open transaction — a
// *sql.Tx or the native transaction — or nil when the scope is inactive.
//
// It exists beside Tx for test tooling that must run statements inside the
// same transaction as the requests under test, whichever kind of pool backs
// the connection. Application code uses Transaction instead.
func (scope *TransactionScope) ActiveExecutor() sqlbind.SQLExecutor {
	if scope == nil {
		return nil
	}
	return scope.executor()
}

// state reports whether a transaction is open and how deep its savepoint stack
// is, which is what a query record needs to place a statement.
func (scope *TransactionScope) state() (bool, int) {
	if scope == nil {
		return false, 0
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.tx != nil || scope.nativeTx != nil, scope.depth
}

func (scope *TransactionScope) executor() sqlbind.SQLExecutor {
	if scope == nil {
		return nil
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.nativeTx != nil {
		return scope.nativeTx
	}
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
//
// The transaction runs on the effective group of ctx, so SelectDB names a
// connection group for a whole transaction exactly as it does for a single
// statement, and nothing here can move a transaction to another group.
func Transaction(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("popcornweb: nil transaction callback")
	}
	current := resources(ctx)
	group := current.effectiveGroup()
	if scope := activeScope(ctx); scope != nil {
		// One transaction never spans two groups: two pools are two
		// connections, and the framework adds no two-phase commit. A collapsed
		// set has one database, so every name names the open transaction.
		if !current.Connections.Collapsed() && scope.Group() != group {
			return fmt.Errorf(
				"%w: cannot open a transaction on group %q inside a transaction on group %q",
				ErrCrossGroupTransaction, group, scope.Group())
		}
		return scope.nested(ctx, fn)
	}
	if scope := adoptExecutorTx(ctx); scope != nil {
		return scope.nested(withScope(ctx, scope), fn)
	}
	connection, err := current.connection()
	if err != nil {
		return err
	}
	scope := newConnectionScope(connection)
	if scope == nil {
		return errors.New("popcornweb: database is not available in context")
	}
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
	current := resources(ctx)
	group := current.effectiveGroup()
	driver := current.DBDriver
	if connection, err := current.connection(); err == nil {
		driver = connection.Driver
	}
	switch tx := unwrapExecutor(executor).(type) {
	case *sql.Tx:
		return &TransactionScope{tx: tx, driver: driver, group: group}
	case database.NativeTx:
		return &TransactionScope{nativeTx: tx, driver: driver, group: group}
	default:
		return nil
	}
}

// savepointExecer is what the savepoint statements need from either
// transaction kind: ExecContext, which both *sql.Tx and a native transaction
// provide.
type savepointExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
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

func (scope *TransactionScope) push(ctx context.Context) (string, savepointExecer, error) {
	scope.mu.Lock()
	failed, driver := scope.failed, scope.driver
	var tx savepointExecer
	switch {
	case scope.nativeTx != nil:
		tx = scope.nativeTx
	case scope.tx != nil:
		tx = scope.tx
	default:
		scope.mu.Unlock()
		return "", nil, errors.New("popcornweb: transaction scope is not active")
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

func (scope *TransactionScope) release(ctx context.Context, tx savepointExecer, name string) error {
	defer scope.pop()
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+name); err != nil {
		scope.markFailed()
		return err
	}
	return nil
}

// rollbackTo undoes the savepoint work. A failure here means the transaction
// state is unknown, so the scope is marked failed and can no longer commit.
func (scope *TransactionScope) rollbackTo(ctx context.Context, tx savepointExecer, name string) {
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
