// Package sqlite provides a SQLite backed authstate.Store.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/petitweb-go/contrib/authstate"
)

const (
	tableName            = "petitweb_authstate"
	defaultMaxKeyBytes   = 256
	defaultMaxValueBytes = 64 << 10
	defaultMaxPruneBatch = 256
	hardMaxKeyBytes      = 4096
	hardMaxValueBytes    = 1 << 20
	hardMaxPruneBatch    = 4096
	maxNamespaceBytes    = 128
)

const createSchemaSQL = `CREATE TABLE IF NOT EXISTS petitweb_authstate (
	namespace TEXT NOT NULL,
	"key" TEXT NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY (namespace, "key")
) WITHOUT ROWID`

// Options controls record isolation, resource bounds, and expiry behavior.
type Options struct {
	Namespace     string
	Now           func() time.Time
	MaxKeyBytes   int
	MaxValueBytes int
	MaxPruneBatch int
}

// Store persists expiring, single-use authentication state in SQLite.
type Store[T any] struct {
	db            *sql.DB
	codec         authstate.Codec[T]
	namespace     string
	now           func() time.Time
	maxKeyBytes   int
	maxValueBytes int
	maxPruneBatch int
}

// NewStore constructs a SQLite backed Store. The caller retains ownership of
// db and must call EnsureSchema before serving requests.
func NewStore[T any](db *sql.DB, codec authstate.Codec[T], options Options) (*Store[T], error) {
	if db == nil || codec == nil || !validNamespace(options.Namespace) ||
		options.MaxKeyBytes < 0 || options.MaxKeyBytes > hardMaxKeyBytes ||
		options.MaxValueBytes < 0 || options.MaxValueBytes > hardMaxValueBytes ||
		options.MaxPruneBatch < 0 || options.MaxPruneBatch > hardMaxPruneBatch {
		return nil, authstate.ErrInvalidOptions
	}
	maxKeyBytes := options.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	maxValueBytes := options.MaxValueBytes
	if maxValueBytes == 0 {
		maxValueBytes = defaultMaxValueBytes
	}
	maxPruneBatch := options.MaxPruneBatch
	if maxPruneBatch == 0 {
		maxPruneBatch = defaultMaxPruneBatch
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store[T]{
		db: db, codec: codec, namespace: options.Namespace, now: now,
		maxKeyBytes: maxKeyBytes, maxValueBytes: maxValueBytes, maxPruneBatch: maxPruneBatch,
	}, nil
}

// EnsureSchema creates the package-owned table or rejects an incompatible
// table with the same name.
func (s *Store[T]) EnsureSchema(ctx context.Context) error {
	if !s.ready() || ctx == nil {
		return authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, createSchemaSQL); err != nil {
		return unavailable(ctx)
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(petitweb_authstate)`)
	if err != nil {
		return unavailable(ctx)
	}
	defer rows.Close()
	want := []schemaColumn{
		{name: "namespace", dataType: "TEXT", notNull: 1, primaryKey: 1},
		{name: "key", dataType: "TEXT", notNull: 1, primaryKey: 2},
		{name: "expires_at_ms", dataType: "INTEGER", notNull: 1},
		{name: "payload", dataType: "BLOB", notNull: 1},
	}
	index := 0
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return unavailable(ctx)
		}
		if index >= len(want) || cid != index || name != want[index].name ||
			strings.ToUpper(dataType) != want[index].dataType || notNull != want[index].notNull ||
			primaryKey != want[index].primaryKey || defaultValue != nil {
			return authstate.ErrInvalidOptions
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return unavailable(ctx)
	}
	if index != len(want) {
		return authstate.ErrInvalidOptions
	}
	return nil
}

type schemaColumn struct {
	name       string
	dataType   string
	notNull    int
	primaryKey int
}

func (s *Store[T]) Put(ctx context.Context, key string, value T, expiresAt time.Time) error {
	if !s.ready() || ctx == nil {
		return authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return authstate.ErrInvalidKey
	}
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) || expiresAt.UnixMilli() <= 0 {
		return authstate.ErrInvalidExpiry
	}
	payload, err := s.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("%w: encode", authstate.ErrCodec)
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: encode", authstate.ErrCodec)
	}
	if len(payload) > s.maxValueBytes {
		return authstate.ErrLimitExceeded
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO petitweb_authstate(namespace, "key", expires_at_ms, payload)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(namespace, "key") DO UPDATE SET
			expires_at_ms = excluded.expires_at_ms,
			payload = excluded.payload
		WHERE petitweb_authstate.expires_at_ms <= ?`,
		s.namespace, key, expiresAt.UnixMilli(), payload, now.UnixMilli())
	if err != nil {
		return unavailable(ctx)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return unavailable(ctx)
	}
	if affected == 0 {
		return authstate.ErrAlreadyExists
	}
	return nil
}

func (s *Store[T]) Take(ctx context.Context, key string) (T, error) {
	var zero T
	if !s.ready() || ctx == nil {
		return zero, authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return zero, authstate.ErrInvalidKey
	}
	var expiresAtMS int64
	var payload []byte
	err := s.db.QueryRowContext(ctx, `
		DELETE FROM petitweb_authstate
		WHERE namespace = ? AND "key" = ?
		RETURNING expires_at_ms, payload`, s.namespace, key).Scan(&expiresAtMS, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return zero, authstate.ErrNotFound
	}
	if err != nil {
		return zero, unavailable(ctx)
	}
	if expiresAtMS <= 0 || len(payload) > s.maxValueBytes {
		return zero, fmt.Errorf("%w: malformed record", authstate.ErrCodec)
	}
	if !time.UnixMilli(expiresAtMS).After(s.now()) {
		return zero, authstate.ErrExpired
	}
	value, err := s.codec.Decode(payload)
	if err != nil {
		return zero, fmt.Errorf("%w: decode", authstate.ErrCodec)
	}
	return value, nil
}

// Prune removes at most limit expired records from this Store's namespace.
func (s *Store[T]) Prune(ctx context.Context, before time.Time, limit int) (int64, error) {
	if !s.ready() || ctx == nil || before.IsZero() || limit <= 0 || limit > s.maxPruneBatch {
		return 0, authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM petitweb_authstate
		WHERE namespace = ? AND "key" IN (
			SELECT "key" FROM petitweb_authstate
			WHERE namespace = ? AND expires_at_ms <= ?
			ORDER BY expires_at_ms, "key" LIMIT ?
		) AND expires_at_ms <= ?`,
		s.namespace, s.namespace, before.UnixMilli(), limit, before.UnixMilli())
	if err != nil {
		return 0, unavailable(ctx)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, unavailable(ctx)
	}
	return affected, nil
}

func (s *Store[T]) ready() bool {
	return s != nil && s.db != nil && s.codec != nil && s.namespace != "" && s.now != nil &&
		s.maxKeyBytes > 0 && s.maxValueBytes > 0 && s.maxPruneBatch > 0
}

func validNamespace(value string) bool {
	if value == "" || len(value) > maxNamespaceBytes {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func unavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: sqlite", authstate.ErrUnavailable)
}

var _ authstate.Store[string] = (*Store[string])(nil)
