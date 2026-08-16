package pwdata

import (
	"context"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Connection is one configured pool the pane can address.
//
// A project may declare several, and requirement:read-write-splitting means a
// group can hold replicas. The pane addresses one at a time rather than the
// group, because selection inside a group is round robin and a page that could
// not say which replica answered would be unable to show the one question
// replicas raise: whether this one has caught up.
type Connection struct {
	// Label identifies the connection, spelled group#ordinal by the runtime.
	Label string
	Group string
	// Driver is per connection rather than per project, so the dialect is
	// resolved here. Nothing forbids two groups on two engines.
	Driver string
	// ReadOnly marks a replica. The pane refuses to write through one, which is
	// not a policy it applies but a fact about the connection.
	ReadOnly bool

	db      sqlbind.SQLExecutor
	dialect dialect
}

// Engine names the dialect this connection speaks.
func (c *Connection) Engine() string { return c.dialect.name }

// queryRows runs a row-returning statement on the pool, dispatching through
// sqlbind.Query so a native executor and a *sql.DB serve the pane alike.
func (c *Connection) queryRows(ctx context.Context, statement string, args ...any) (sqlbind.Rows, error) {
	return sqlbind.Query(ctx, c.db, statement, args...)
}

// NewConnection describes one pool for the pane. db is a *sql.DB or the
// native executor of an engine that bypasses database/sql.
func NewConnection(label, group, driver string, readOnly bool, db sqlbind.SQLExecutor) Connection {
	return Connection{
		Label: label, Group: group, Driver: driver, ReadOnly: readOnly,
		db: db, dialect: dialectFor(driver),
	}
}

// Server is the pane over one or more connections.
type Server struct {
	connections []Connection
	// environment is the runtime environment the application is serving, shown
	// so the page can never be mistaken for one pointed at something else.
	environment string
}

// New builds a server over the connections the application opened.
//
// The order is preserved, because it is the order the project declared and a
// developer reads the list expecting to recognise it.
func New(connections []Connection, environment string) *Server {
	return &Server{connections: connections, environment: environment}
}

// NewSingle is the ordinary case: a configuration that declares one database
// and never names a group.
func NewSingle(db sqlbind.SQLExecutor, driver, environment string) *Server {
	return New([]Connection{NewConnection("default", "default", driver, false, db)}, environment)
}

// Connections lists what the pane can address.
func (s *Server) Connections() []Connection { return s.connections }

// Default is the connection a page selects when none was asked for.
//
// A writable one, because the pane edits and a page that opened on a replica
// would refuse the first edit attempted on it for a reason the developer did
// not choose. Falling back to the first connection keeps a project whose
// connections are all replicas usable for reading.
func (s *Server) Default() *Connection {
	for index := range s.connections {
		if !s.connections[index].ReadOnly {
			return &s.connections[index]
		}
	}
	if len(s.connections) == 0 {
		return nil
	}
	return &s.connections[0]
}

// Lookup finds a connection by label, or returns the default when the label is
// empty or unknown.
func (s *Server) Lookup(label string) *Connection {
	for index := range s.connections {
		if s.connections[index].Label == label {
			return &s.connections[index]
		}
	}
	return s.Default()
}

// gooseVersionTable is where system:goose records what it applied. The pane
// reads it rather than data:migration-source, because the question is what this
// database has, not what the project would apply to it.
const gooseVersionTable = "goose_db_version"

// Migration is what the connected database says about its own schema version.
type Migration struct {
	// Present is false when the table is absent, which means nothing has been
	// applied here rather than that something went wrong.
	Present bool
	Version int64
	Applied int
}

// MigrationState reads the applied version.
//
// Display only. api:cli-migrate owns applying and rolling back, and api:cli-dev
// already rolls back and reseeds on its own when a migration source changes —
// a second actor deciding the same thing from a page would be one too many.
func (c *Connection) MigrationState(ctx context.Context) (Migration, error) {
	tables, err := c.Tables(ctx)
	if err != nil {
		return Migration{}, err
	}
	return c.migrationState(ctx, tables)
}

// migrationState is MigrationState over an already-fetched catalog.
func (c *Connection) migrationState(ctx context.Context, tables []Table) (Migration, error) {
	present := false
	for _, table := range tables {
		if table.Name == gooseVersionTable {
			present = true
			break
		}
	}
	if !present {
		return Migration{}, nil
	}
	state := Migration{Present: true}
	rows, err := c.queryRows(ctx,
		"SELECT COALESCE(MAX(version_id), 0), COUNT(*) FROM "+c.dialect.quote(gooseVersionTable)+
			" WHERE is_applied = "+c.dialect.trueLiteral())
	if err != nil {
		return Migration{Present: true}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Migration{Present: true}, err
		}
		return state, nil
	}
	if err := rows.Scan(&state.Version, &state.Applied); err != nil {
		return Migration{Present: true}, err
	}
	return state, nil
}

