// Package sessionstore keeps Popcorn Wave login sessions in a relational
// database. It owns its own table and never inspects application tables.
//
// The engine is not compiled in here. One of the sibling packages describes it
// and registers itself, so an application blank-imports the engine it runs:
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/postgres"
package sessionstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

const (
	// DefaultTable is the table owned by this package.
	DefaultTable = "popcornwave_session"

	defaultMaxPayloadBytes = 64 << 10
	hardMaxPayloadBytes    = 1 << 20
	defaultMaxPruneBatch   = 256
	hardMaxPruneBatch      = 4096
	keyHashLength          = 64
	maxMethodBytes         = 64
)

// Options bounds record size, selects the owned table, and names the engine
// whose dialect the statements take.
type Options struct {
	// Dialect is the registered engine name, which is the dialect the DSN
	// scheme already resolved to.
	Dialect         string
	Table           string
	Now             func() time.Time
	MaxPayloadBytes int
	MaxPruneBatch   int
}

// Store is a relational session.RawStore. The caller owns db and must
// call EnsureSchema, or carry the migration, before serving requests.
type Store struct {
	db              sqlbind.SQLExecutor
	dialect         Dialect
	table           string
	now             func() time.Time
	maxPayloadBytes int
	maxPruneBatch   int
}

// NewStore constructs a Store over db, which is a *sql.DB or the native
// executor of an engine that bypasses database/sql. db stays owned by the
// caller, because a session store commonly shares the pool of the RDB
// middleware. Wrap the result with session.Typed to give a Manager the payload
// type it stores.
func NewStore(db sqlbind.SQLExecutor, options Options) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil dependency", session.ErrInvalidOptions)
	}
	table := options.Table
	if table == "" {
		table = DefaultTable
	}
	if !validIdentifier(table) {
		return nil, fmt.Errorf("%w: table name", session.ErrInvalidOptions)
	}
	if options.MaxPayloadBytes < 0 || options.MaxPayloadBytes > hardMaxPayloadBytes ||
		options.MaxPruneBatch < 0 || options.MaxPruneBatch > hardMaxPruneBatch {
		return nil, fmt.Errorf("%w: bounds", session.ErrInvalidOptions)
	}
	maxPayloadBytes := options.MaxPayloadBytes
	if maxPayloadBytes == 0 {
		maxPayloadBytes = defaultMaxPayloadBytes
	}
	maxPruneBatch := options.MaxPruneBatch
	if maxPruneBatch == 0 {
		maxPruneBatch = defaultMaxPruneBatch
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	dialect, err := dialectFor(options.Dialect)
	if err != nil {
		return nil, err
	}
	return &Store{
		db:              db,
		dialect:         dialect,
		table:           table,
		now:             now,
		maxPayloadBytes: maxPayloadBytes,
		maxPruneBatch:   maxPruneBatch,
	}, nil
}

// MigrationName is the stable name of the migration a project carries for this
// table, without a version. The version belongs to the project: the file takes
// the next free one when it is written, so adding this capability to a project
// that already applied migrations renumbers nothing. The name is what makes the
// file recognizable at whatever version it ended up with.
const MigrationName = "init_popcornwave_session"

// ErrSchemaMissing reports that the owned table does not exist yet. A project
// creates it from MigrationSQL rather than at startup.
var ErrSchemaMissing = errors.New("sessionstore: session table is missing")

// Columns are the columns of the owned table, in the order every dialect
// declares them. VerifySchema compares what it finds against this list.
var Columns = []string{
	"key_hash", "created_at_ms", "authenticated_at_ms", "last_seen_at_ms",
	"expires_at_ms", "idle_expires_at_ms", "method", "version", "payload",
}

// SchemaSQL returns the deterministic DDL of the owned table under this
// store's engine.
func (s *Store) SchemaSQL() string { return s.dialect.CreateTable(s.table) }

// Dialect reports the engine this store speaks.
func (s *Store) Dialect() string { return s.dialect.Name }

// EnsureSchema creates the owned table when it is missing. A project that
// carries MigrationSQL does not need it; VerifySchema is the startup check.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if !s.ready() || ctx == nil {
		return fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, s.SchemaSQL()); err != nil {
		return unavailable(ctx)
	}
	return nil
}

// VerifySchema reports whether the owned table exists with the expected
// columns. It never changes the schema.
func (s *Store) VerifySchema(ctx context.Context) error {
	if !s.ready() || ctx == nil {
		return fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	found, err := s.dialect.Columns(ctx, s.db, s.table)
	if err != nil {
		return unavailable(ctx)
	}
	if len(found) == 0 {
		return fmt.Errorf("%w: %s", ErrSchemaMissing, s.table)
	}
	if len(found) != len(Columns) {
		return fmt.Errorf("%w: table %s has an unexpected column layout", session.ErrInvalidOptions, s.table)
	}
	for index, name := range found {
		if !strings.EqualFold(name, Columns[index]) {
			return fmt.Errorf("%w: table %s has an unexpected column layout", session.ErrInvalidOptions, s.table)
		}
	}
	return nil
}

func (s *Store) Put(ctx context.Context, keyHash string, record session.RawRecord) error {
	if !s.ready() || ctx == nil {
		return fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return session.ErrInvalidKey
	}
	if record.ExpiresAt.IsZero() || len(record.Method) > maxMethodBytes {
		return fmt.Errorf("%w: record", session.ErrInvalidOptions)
	}
	payload := record.Payload
	if len(payload) == 0 || len(payload) > s.maxPayloadBytes {
		return fmt.Errorf("%w: payload size", session.ErrCodec)
	}
	_, err := s.db.ExecContext(ctx, s.rebind(s.dialect.Upsert(s.table)),
		keyHash, milli(record.CreatedAt), milli(record.AuthenticatedAt), milli(record.LastSeenAt),
		milli(record.ExpiresAt), milli(record.IdleExpiresAt), record.Method, record.Version, payload)
	if err != nil {
		return unavailable(ctx)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, keyHash string) (session.RawRecord, error) {
	var zero session.RawRecord
	if !s.ready() || ctx == nil {
		return zero, fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if !validKeyHash(keyHash) {
		return zero, session.ErrInvalidKey
	}
	var createdAt, authenticatedAt, lastSeenAt, expiresAt, idleExpiresAt int64
	var version int
	var method string
	var payload []byte
	// sqlbind.Query rather than QueryRowContext, because only database/sql can
	// construct the *sql.Row that method returns and this store also runs on
	// native executors.
	rows, err := sqlbind.Query(ctx, s.db, s.rebind(`
		SELECT created_at_ms, authenticated_at_ms, last_seen_at_ms, expires_at_ms,
			idle_expires_at_ms, method, version, payload
		FROM `+s.table+` WHERE key_hash = ?`), keyHash)
	if err != nil {
		return zero, unavailable(ctx)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, unavailable(ctx)
		}
		return zero, session.ErrNotFound
	}
	if err := rows.Scan(&createdAt, &authenticatedAt, &lastSeenAt, &expiresAt, &idleExpiresAt, &method, &version, &payload); err != nil {
		return zero, unavailable(ctx)
	}
	if expiresAt <= 0 || len(payload) == 0 || len(payload) > s.maxPayloadBytes {
		return zero, fmt.Errorf("%w: malformed record", session.ErrCodec)
	}
	record := session.RawRecord{
		Payload:         payload,
		CreatedAt:       time.UnixMilli(createdAt),
		AuthenticatedAt: time.UnixMilli(authenticatedAt),
		LastSeenAt:      time.UnixMilli(lastSeenAt),
		ExpiresAt:       time.UnixMilli(expiresAt),
		IdleExpiresAt:   timeOrZero(idleExpiresAt),
		Method:          method,
		Version:         version,
	}
	if !record.Deadline().After(s.now()) {
		return zero, session.ErrExpired
	}
	return record, nil
}

func (s *Store) Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	if !s.ready() || ctx == nil {
		return fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return session.ErrInvalidKey
	}
	now := milli(s.now())
	result, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE `+s.table+`
		SET last_seen_at_ms = ?, idle_expires_at_ms = ?
		WHERE key_hash = ? AND expires_at_ms > ?
			AND (idle_expires_at_ms = 0 OR idle_expires_at_ms > ?)
			AND ? <= expires_at_ms`),
		milli(lastSeenAt), milli(idleExpiresAt), keyHash, now, now, milli(idleExpiresAt))
	if err != nil {
		return unavailable(ctx)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return unavailable(ctx)
	}
	if affected == 0 {
		return session.ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, keyHash string) error {
	if !s.ready() || ctx == nil {
		return fmt.Errorf("%w: store", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKeyHash(keyHash) {
		return session.ErrInvalidKey
	}
	if _, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM `+s.table+` WHERE key_hash = ?`), keyHash); err != nil {
		return unavailable(ctx)
	}
	return nil
}

// Prune removes at most limit records that expired before the given time.
func (s *Store) Prune(ctx context.Context, before time.Time, limit int) (int64, error) {
	if !s.ready() || ctx == nil || before.IsZero() || limit <= 0 || limit > s.maxPruneBatch {
		return 0, fmt.Errorf("%w: prune", session.ErrInvalidOptions)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cutoff := milli(before)
	result, err := s.db.ExecContext(ctx, s.rebind(s.dialect.Prune(s.table)), cutoff, cutoff, limit)
	if err != nil {
		return 0, unavailable(ctx)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, unavailable(ctx)
	}
	return affected, nil
}

// rebind adapts a statement written with ? to the engine's own placeholders.
func (s *Store) rebind(statement string) string {
	if s.dialect.Rebind == nil {
		return statement
	}
	return s.dialect.Rebind(statement)
}

func (s *Store) ready() bool {
	return s != nil && s.db != nil && s.dialect.Name != "" && s.table != "" && s.now != nil &&
		s.maxPayloadBytes > 0 && s.maxPruneBatch > 0
}

// unavailable reports a backend failure without copying driver text, which can
// contain a DSN or query fragment, into the response path.
func unavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: %s", session.ErrUnavailable, "database")
}

func milli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func timeOrZero(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func validKeyHash(value string) bool {
	if len(value) != keyHashLength {
		return false
	}
	for index := range len(value) {
		c := value[index]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// validIdentifier keeps a configured table name out of SQL injection range,
// because an identifier cannot be a bound parameter.
func validIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && index > 0:
		default:
			return false
		}
	}
	return !strings.EqualFold(value, "sqlite_master")
}

var _ session.RawStore = (*Store)(nil)
