package pwconfig

import (
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/minitoml"
)

// applyRDBConnections drives the generated binding and hands back just the
// connection set. The elements used to be read by hand here; codegen owns that
// now, so the tests below exercise the code that actually runs.
func applyRDBConnections(overlay *configbind.Overlay) ([]RDBConnectionConfig, error) {
	var config MiddlewareConfig
	if err := applyMiddlewareConfigDefinition5(&config, overlay); err != nil {
		return nil, err
	}
	return config.RDB.Connections, nil
}

func TestApplyRDBConnectionsReadsElementsAndDefaults(t *testing.T) {
	element := func(pairs map[string]string) *configbind.Overlay {
		table := configbind.NewOverlay()
		for key, value := range pairs {
			table.Set(key, value, configbind.PlaceFile)
		}
		return table
	}
	overlay := configbind.NewOverlay()
	overlay.SetTables("middleware.rdb.connections", []*configbind.Overlay{
		element(map[string]string{
			"group": "writer", "dsn": "postgres://host/app",
			"max_open_conns": "20", "conn_max_lifetime": "30m",
		}),
		// An element that sets only what it cares about takes the documented
		// per-element defaults for everything else.
		element(map[string]string{"group": "replica", "dsn": "postgres://ro/app", "readonly": "true"}),
	}, configbind.PlaceFile)

	connections, err := applyRDBConnections(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(connections))
	}
	if got := connections[0]; got.Group != "writer" || got.MaxOpenConns != 20 || got.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("writer = %+v", got)
	}
	if got := connections[1]; !got.ReadOnly || got.ConnectTimeout != 5*time.Second || got.MaxOpenConns != 0 {
		t.Fatalf("replica = %+v", got)
	}
	if connections[0].ReadOnly {
		t.Fatal("an element without readonly was marked read-only")
	}
	// A malformed element value names the key and the element it came from, so
	// an operator with several connections is told which one to fix.
	broken := configbind.NewOverlay()
	broken.SetTables("middleware.rdb.connections", []*configbind.Overlay{
		element(map[string]string{"group": "writer", "conn_max_lifetime": "thirty"}),
	}, configbind.PlaceFile)
	_, err = applyRDBConnections(broken)
	if err == nil || !strings.Contains(err.Error(), "middleware.rdb.connections[0].conn_max_lifetime") {
		t.Fatalf("err = %v", err)
	}
}

// overlayFromTOML mirrors what configbind does with a parsed document: an array
// of tables becomes one overlay per element, keyed relative to the array. The
// test builds it from real TOML text so the key names this package reads are
// checked against the parser rather than against an assumption.
func overlayFromTOML(t *testing.T, source string) *configbind.Overlay {
	t.Helper()
	document, err := minitoml.ParseString(source)
	if err != nil {
		t.Fatal(err)
	}
	overlay := configbind.NewOverlay()
	for _, key := range document.Keys() {
		value, ok := document.Get(key)
		if !ok {
			continue
		}
		if value.Kind != minitoml.KindTableArray {
			raw, err := value.AsString()
			if err != nil {
				t.Fatal(err)
			}
			overlay.Set(key, raw, configbind.PlaceFile)
			continue
		}
		tables := make([]*configbind.Overlay, 0, len(value.Tables))
		for _, table := range value.Tables {
			element := configbind.NewOverlay()
			for _, elementKey := range table.Keys() {
				elementValue, ok := table.Get(elementKey)
				if !ok {
					continue
				}
				raw, err := elementValue.AsString()
				if err != nil {
					t.Fatal(err)
				}
				element.Set(elementKey, raw, configbind.PlaceFile)
			}
			tables = append(tables, element)
		}
		overlay.SetTables(key, tables, configbind.PlaceFile)
	}
	return overlay
}

func TestConnectionsLoadFromRealTOML(t *testing.T) {
	overlay := overlayFromTOML(t, `
[middleware.rdb]
enabled = true
default_group = "replica"

[[middleware.rdb.connections]]
group = "writer"
dsn = "sqlite://writer.db"
max_open_conns = 20

[[middleware.rdb.connections]]
group = "replica"
dsn = "sqlite://replica-1.db"
readonly = true

[[middleware.rdb.connections]]
group = "replica"
dsn = "sqlite://replica-2.db"
readonly = true
`)
	connections, err := applyRDBConnections(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 3 {
		t.Fatalf("connections = %d, want 3", len(connections))
	}
	config := RDBConfig{
		Enabled:      true,
		DefaultGroup: valueOf(overlay, "middleware.rdb.default_group"),
		Connections:  connections,
	}
	// The pool-level checks — drivers, dialect mixing, sqlite memory sharing —
	// belong to whoever opens the pools. What this level owns is that the three
	// groups resolve from what the file said.
	if _, err := ResolveDefaultGroup(config, connections); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveWriteGroup(config, connections); err != nil {
		t.Fatal(err)
	}
	if got := connections[0]; got.Group != "writer" || got.MaxOpenConns != 20 || got.ReadOnly {
		t.Fatalf("writer = %+v", got)
	}
	if got := connections[2]; got.Group != "replica" || !got.ReadOnly || got.DSN != "sqlite://replica-2.db" {
		t.Fatalf("second replica = %+v", got)
	}
	// The writer group is the only writable one, so migrations resolve to it
	// without a migration_group setting.
	dsn, err := config.MigrationDSN()
	if err != nil {
		t.Fatal(err)
	}
	if dsn != "sqlite://writer.db" {
		t.Fatalf("migration dsn = %q", dsn)
	}
}

func writer(dsn string) RDBConnectionConfig {
	return RDBConnectionConfig{Group: "writer", DSN: dsn, ConnectTimeout: 5 * time.Second}
}

func replica(dsn string) RDBConnectionConfig {
	return RDBConnectionConfig{Group: "replica", DSN: dsn, ReadOnly: true, ConnectTimeout: 5 * time.Second}
}

func TestResolveRDBConnectionsCarriesOneConnectionThrough(t *testing.T) {
	connections, err := ResolveConnections(RDBConfig{
		Enabled: true,
		Connections: []RDBConnectionConfig{
			{DSN: "sqlite://app.db", MaxOpenConns: 4, ConnectTimeout: 5 * time.Second},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(connections))
	}
	if connections[0].Group != "default" || connections[0].DSN != "sqlite://app.db" || connections[0].MaxOpenConns != 4 {
		t.Fatalf("connection = %+v", connections[0])
	}
	if connections[0].ReadOnly {
		t.Fatal("the single database was marked read-only")
	}
}

// The single-DSN form was removed, so a file still setting middleware.rdb.dsn
// configures an enabled database with no pool behind it. The error has to name
// the form that replaced it, because the stale key itself is claimed by no
// binding and reaches nothing here.
func TestResolveRDBConnectionsRequiresAConnection(t *testing.T) {
	_, err := ResolveConnections(RDBConfig{Enabled: true})
	if err == nil {
		t.Fatal("an enabled rdb without any connection was accepted")
	}
	if !strings.Contains(err.Error(), "middleware.rdb.connections") || !strings.Contains(err.Error(), "middleware.rdb.dsn") {
		t.Fatalf("err = %v, want the replacement form and the removed key named", err)
	}
}

func TestResolveRDBConnectionsNamesAnUnnamedGroupDefault(t *testing.T) {
	// One connection that names no group is the common single-database case, so
	// it takes the default group rather than failing.
	connections, err := ResolveConnections(RDBConfig{
		Enabled:     true,
		Connections: []RDBConnectionConfig{{DSN: "sqlite://app.db", ConnectTimeout: time.Second}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if connections[0].Group != "default" {
		t.Fatalf("group = %q", connections[0].Group)
	}
}

func TestResolveDefaultGroup(t *testing.T) {
	connections := []RDBConnectionConfig{writer("sqlite://w.db"), replica("sqlite://r.db")}
	if _, err := ResolveDefaultGroup(RDBConfig{Connections: connections}, connections); err == nil {
		t.Fatal("two groups without default_group were accepted")
	}
	group, err := ResolveDefaultGroup(RDBConfig{DefaultGroup: "replica", Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "replica" {
		t.Fatalf("group = %q", group)
	}
	if _, err := ResolveDefaultGroup(RDBConfig{DefaultGroup: "nowhere", Connections: connections}, connections); err == nil {
		t.Fatal("default_group naming no group was accepted")
	}
	// One group needs no pointer at all.
	single := []RDBConnectionConfig{writer("sqlite://w.db")}
	if group, err = ResolveDefaultGroup(RDBConfig{Connections: single}, single); err != nil || group != "writer" {
		t.Fatalf("group = %q, err = %v", group, err)
	}
}

func TestResolveWriteGroup(t *testing.T) {
	connections := []RDBConnectionConfig{writer("sqlite://w.db"), replica("sqlite://r.db")}
	// One writable group is unambiguous, so write_group may stay unset.
	group, err := ResolveWriteGroup(RDBConfig{Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "writer" {
		t.Fatalf("group = %q", group)
	}
	if _, err := ResolveWriteGroup(RDBConfig{WriteGroup: "replica", Connections: connections}, connections); err == nil {
		t.Fatal("a read-only group was accepted as the write group")
	}
	readOnly := []RDBConnectionConfig{replica("sqlite://r.db")}
	if _, err := ResolveWriteGroup(RDBConfig{Connections: readOnly}, readOnly); err == nil {
		t.Fatal("a topology with no writable connection was accepted")
	}
	ambiguous := []RDBConnectionConfig{
		writer("sqlite://w.db"),
		{Group: "reporting", DSN: "sqlite://rep.db", ConnectTimeout: time.Second},
	}
	if _, err := ResolveWriteGroup(RDBConfig{Connections: ambiguous}, ambiguous); err == nil {
		t.Fatal("two writable groups without write_group were accepted")
	}
}

func TestResolveMigrationGroupFallsBackToWriteGroup(t *testing.T) {
	connections := []RDBConnectionConfig{writer("sqlite://w.db"), replica("sqlite://r.db")}
	group, err := ResolveMigrationGroup(RDBConfig{Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "writer" {
		t.Fatalf("group = %q", group)
	}
	if _, err := ResolveMigrationGroup(RDBConfig{MigrationGroup: "replica", Connections: connections}, connections); err == nil {
		t.Fatal("migrations were pointed at a read-only group")
	}
}

// valueOf reads one overlay key as a string, ignoring an absent one.
func valueOf(overlay *configbind.Overlay, key string) string {
	value, _ := overlay.GetString(key)
	return value
}
