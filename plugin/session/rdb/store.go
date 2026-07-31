// Package rdb stores Popcorn Wave login sessions in a database/sql database.
//
// The package owns its own table and never inspects application tables. SQLite
// is the guaranteed dialect; another dialect needs its own DDL until a schema
// provider covers it.
package rdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/session"
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

// Options bounds record size and selects the owned table.
type Options struct {
	Table           string
	Now             func() time.Time
	MaxPayloadBytes int
	MaxPruneBatch   int
}

// Store is a database/sql backed session.RawStore. The caller owns db and must
// call EnsureSchema, or carry the migration, before serving requests.
type Store struct {
	db              *sql.DB
	table           string
	now             func() time.Time
	maxPayloadBytes int
	maxPruneBatch   int
}

// NewStore constructs a Store over db. db stays owned by the caller, because a
// session store commonly shares the pool of the RDB middleware. Wrap the result
// with session.Typed to give a Manager the payload type it stores.
func NewStore(db *sql.DB, options Options) (*Store, error) {
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
	return &Store{
		db:              db,
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
var ErrSchemaMissing = errors.New("rdb: session table is missing")

// SchemaSQL returns the deterministic SQLite DDL of table.
func SchemaSQL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	key_hash TEXT PRIMARY KEY,
	created_at_ms INTEGER NOT NULL,
	authenticated_at_ms INTEGER NOT NULL,
	last_seen_at_ms INTEGER NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	idle_expires_at_ms INTEGER NOT NULL,
	method TEXT NOT NULL,
	version INTEGER NOT NULL,
	payload BLOB NOT NULL
)`
}

// MigrationSQL returns the goose migration that creates table. It is the source
// of the file a project keeps in its migration directory, and later of the file
// api:cli-init scaffolds.
func MigrationSQL(table string) string {
	if table == "" {
		table = DefaultTable
	}
	return `-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/plugin/session/rdb.
-- Login sessions: one row per issued cookie token, keyed by its hash.
` + SchemaSQL(table) + `;

-- +goose Down
DROP TABLE ` + table + `;
`
}

// SchemaSQL returns the deterministic SQLite DDL of the owned table.
func (s *Store) SchemaSQL() string { return SchemaSQL(s.table) }

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
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+s.table+`)`)
	if err != nil {
		return unavailable(ctx)
	}
	defer rows.Close()
	want := []string{
		"key_hash", "created_at_ms", "authenticated_at_ms", "last_seen_at_ms",
		"expires_at_ms", "idle_expires_at_ms", "method", "version", "payload",
	}
	index := 0
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return unavailable(ctx)
		}
		if index >= len(want) || name != want[index] {
			return fmt.Errorf("%w: table %s has an unexpected column layout", session.ErrInvalidOptions, s.table)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return unavailable(ctx)
	}
	if index == 0 {
		return fmt.Errorf("%w: %s", ErrSchemaMissing, s.table)
	}
	if index != len(want) {
		return fmt.Errorf("%w: table %s has an unexpected column layout", session.ErrInvalidOptions, s.table)
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO `+s.table+`(key_hash, created_at_ms, authenticated_at_ms, last_seen_at_ms,
			expires_at_ms, idle_expires_at_ms, method, version, payload)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key_hash) DO UPDATE SET
			created_at_ms = excluded.created_at_ms,
			authenticated_at_ms = excluded.authenticated_at_ms,
			last_seen_at_ms = excluded.last_seen_at_ms,
			expires_at_ms = excluded.expires_at_ms,
			idle_expires_at_ms = excluded.idle_expires_at_ms,
			method = excluded.method,
			version = excluded.version,
			payload = excluded.payload`,
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
	err := s.db.QueryRowContext(ctx, `
		SELECT created_at_ms, authenticated_at_ms, last_seen_at_ms, expires_at_ms,
			idle_expires_at_ms, method, version, payload
		FROM `+s.table+` WHERE key_hash = ?`, keyHash).
		Scan(&createdAt, &authenticatedAt, &lastSeenAt, &expiresAt, &idleExpiresAt, &method, &version, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return zero, session.ErrNotFound
	}
	if err != nil {
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
	result, err := s.db.ExecContext(ctx, `
		UPDATE `+s.table+`
		SET last_seen_at_ms = ?, idle_expires_at_ms = ?
		WHERE key_hash = ? AND expires_at_ms > ?
			AND (idle_expires_at_ms = 0 OR idle_expires_at_ms > ?)
			AND ? <= expires_at_ms`,
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
	if _, err := s.db.ExecContext(ctx, `DELETE FROM `+s.table+` WHERE key_hash = ?`, keyHash); err != nil {
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
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM `+s.table+` WHERE key_hash IN (
			SELECT key_hash FROM `+s.table+`
			WHERE expires_at_ms <= ? OR (idle_expires_at_ms > 0 AND idle_expires_at_ms <= ?)
			ORDER BY expires_at_ms, key_hash LIMIT ?
		)`, cutoff, cutoff, limit)
	if err != nil {
		return 0, unavailable(ctx)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, unavailable(ctx)
	}
	return affected, nil
}

func (s *Store) ready() bool {
	return s != nil && s.db != nil && s.table != "" && s.now != nil &&
		s.maxPayloadBytes > 0 && s.maxPruneBatch > 0
}

// unavailable reports a backend failure without copying driver text, which can
// contain a DSN or query fragment, into the response path.
func unavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: rdb", session.ErrUnavailable)
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
