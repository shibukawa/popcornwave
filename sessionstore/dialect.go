package sessionstore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/session"
)

// Dialect is everything one database engine has to say differently about the
// table this package owns. The statements that only differ in placeholder
// style are written once in the store and rewritten by Rebind, so an engine
// package describes the schema and the two statements no dialect shares.
type Dialect struct {
	// Name is the dialect identifier rule:rdb-dsn-resolution resolves a DSN
	// scheme to, which is what a configured DSN and this store agree on.
	Name string
	// CreateTable is the deterministic DDL of the owned table.
	CreateTable func(table string) string
	// Upsert replaces one row by primary key. Every engine spells the
	// conflict clause its own way.
	Upsert func(table string) string
	// Prune deletes at most a limited number of expired rows. The subquery
	// form that reads well elsewhere is not accepted by every engine.
	Prune func(table string) string
	// Columns lists the columns of table in declaration order, or none at all
	// when the table does not exist.
	Columns func(ctx context.Context, db *sql.DB, table string) ([]string, error)
	// Rebind adapts ? placeholders to the engine's own numbering. A nil
	// Rebind leaves a statement as written.
	Rebind func(statement string) string
}

var dialects struct {
	sync.RWMutex
	byName map[string]Dialect
}

// Register adds an engine's dialect. An engine package calls it from init, so
// a blank import is what makes session.backend = "rdb" work against that
// engine:
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/postgres"
//
// A duplicate or incomplete dialect panics: two descriptions of one engine is
// a build mistake, not a runtime condition.
func Register(dialect Dialect) {
	if dialect.Name == "" || dialect.CreateTable == nil || dialect.Upsert == nil ||
		dialect.Prune == nil || dialect.Columns == nil {
		panic("sessionstore: incomplete dialect")
	}
	dialects.Lock()
	defer dialects.Unlock()
	if dialects.byName == nil {
		dialects.byName = make(map[string]Dialect)
	}
	if _, taken := dialects.byName[dialect.Name]; taken {
		panic(fmt.Sprintf("sessionstore: dialect %q is already registered", dialect.Name))
	}
	dialects.byName[dialect.Name] = dialect
}

// Dialects lists the registered dialect names in order, which is what an error
// reports when the configured engine is not among them.
func Dialects() []string {
	dialects.RLock()
	defer dialects.RUnlock()
	names := make([]string, 0, len(dialects.byName))
	for name := range dialects.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// dialectFor resolves one engine, explaining the missing import rather than
// the missing map entry.
func dialectFor(name string) (Dialect, error) {
	dialects.RLock()
	dialect, ok := dialects.byName[name]
	dialects.RUnlock()
	if ok {
		return dialect, nil
	}
	if name == "" {
		return Dialect{}, fmt.Errorf("%w: no database dialect", session.ErrInvalidOptions)
	}
	return Dialect{}, fmt.Errorf(
		"session storage for %q needs its engine; add to the application: import _ %q",
		name, "github.com/shibukawa/popcornwave/sessionstore/"+name)
}

// MigrationSQL returns the goose migration that creates table under one
// engine. It is the source of the file a project keeps in its migration
// directory, and later of the file api:cli-init scaffolds.
func MigrationSQL(dialect, table string) (string, error) {
	if table == "" {
		table = DefaultTable
	}
	engine, err := dialectFor(dialect)
	if err != nil {
		return "", err
	}
	if !validIdentifier(table) {
		return "", fmt.Errorf("%w: table name", session.ErrInvalidOptions)
	}
	return `-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/sessionstore.
-- Login sessions: one row per issued cookie token, keyed by its hash.
` + engine.CreateTable(table) + `;

-- +goose Down
DROP TABLE ` + table + `;
`, nil
}

// NumberedPlaceholders rewrites ? into $1, $2, and so on. An engine whose
// driver numbers its placeholders points Rebind here instead of restating
// every statement.
func NumberedPlaceholders(statement string) string {
	var builder strings.Builder
	builder.Grow(len(statement) + 8)
	index := 0
	for _, r := range statement {
		if r != '?' {
			builder.WriteRune(r)
			continue
		}
		index++
		builder.WriteByte('$')
		builder.WriteString(strconv.Itoa(index))
	}
	return builder.String()
}

// ScanColumns reads one column name per row, which is the shape every engine's
// catalog query is written to return.
func ScanColumns(rows *sql.Rows) ([]string, error) {
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
