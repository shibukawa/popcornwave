package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinybind-go/sqlbind"
	"github.com/shibukawa/tinygodriver/database/pgx"
)

// WithConn hands the pgx connection behind the effective PostgreSQL connection
// to fn, so pgx capabilities stay reachable without any pgx type entering the
// framework API.
//
// The request-time path runs on a pgxpool rather than database/sql, so
// pw.DB reports no *sql.DB for a PostgreSQL group and sql.Conn.Raw — the
// escape hatch on the database/sql path — cannot be reached from a handler.
// This is that hatch for the native path. It serves batches, CopyFrom, and
// LISTEN/NOTIFY:
//
//	err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
//		batch := &pgx.Batch{}
//		for _, row := range rows {
//			batch.Queue("INSERT INTO t (a) VALUES ($1)", row.A)
//		}
//		results := conn.SendBatch(ctx, batch)
//		defer results.Close()
//		for range rows {
//			if _, err := results.Exec(); err != nil {
//				return err
//			}
//		}
//		return results.Close()
//	})
//
// Inside pw.Transaction the callback receives the connection that transaction
// is executing on, so its work joins the transaction and rolls back with it.
// Outside one, a pooled connection is leased for the call and returned after.
// Either way nothing derived from the connection may outlive fn: rows must be
// read, and results closed, before it returns.
//
// Work done here does not pass through the framework's instrumented executor,
// so it produces no query log entry and no span of its own. Wrap it in one:
//
//	ctx, span := pw.StartSpanKind(ctx, "insert-batch", pw.SpanKindClient)
//	defer span.End()
//
// A group that is not PostgreSQL returns an error naming the dialect it found.
func WithConn(ctx context.Context, fn func(*pgx.Conn) error) error {
	if fn == nil {
		return errors.New("popcornweb/database/postgres: WithConn was given no callback")
	}
	executor, err := pwruntime.SQLExecutor(ctx)
	if err != nil {
		return err
	}
	switch target := unwrapExecutor(executor).(type) {
	case *nativeTx:
		// The transaction is executing on this connection, so statements the
		// callback issues are inside it. Leasing a second connection instead
		// would put its writes outside the transaction the caller opened,
		// where a rollback would leave them standing.
		return fn(target.tx.Conn())
	case *nativeDB:
		conn, err := target.pool.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()
		return fn(conn.Conn())
	default:
		return notPostgres(ctx)
	}
}

// unwrapExecutor peels the query-diagnostics wrapper the framework installs
// when diagnostics are enabled. The assertion is structural so this package
// needs no framework internals to see past it.
func unwrapExecutor(executor sqlbind.SQLExecutor) sqlbind.SQLExecutor {
	for {
		wrapper, unwrappable := executor.(interface {
			Unwrap() sqlbind.SQLExecutor
		})
		if !unwrappable {
			return executor
		}
		inner := wrapper.Unwrap()
		if inner == nil {
			return executor
		}
		executor = inner
	}
}

// notPostgres names the engine that was found, because the likely mistake is a
// handler written against PostgreSQL running on a group configured for another
// database rather than a misuse of this function.
func notPostgres(ctx context.Context) error {
	if dialect, known := pwruntime.DBDriver(ctx); known {
		return fmt.Errorf(
			"popcornweb/database/postgres: WithConn needs a PostgreSQL connection, but the effective group runs on %s",
			dialect)
	}
	return errors.New(
		"popcornweb/database/postgres: WithConn needs a PostgreSQL connection, and the effective group is not one")
}
