package pw

import (
	"path/filepath"
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
	if err := applyMiddlewareConfigDefinition4(&config, overlay); err != nil {
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
	// A malformed element value names the key. The generated binding does not
	// carry the element index the hand-written reader used to include, so an
	// operator with several connections still has to find which one it is.
	broken := configbind.NewOverlay()
	broken.SetTables("middleware.rdb.connections", []*configbind.Overlay{
		element(map[string]string{"group": "writer", "conn_max_lifetime": "thirty"}),
	}, configbind.PlaceFile)
	_, err = applyRDBConnections(broken)
	if err == nil || !strings.Contains(err.Error(), "middleware.rdb.connections.conn_max_lifetime") {
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
	if err := validateRDBConfig(config); err != nil {
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

func TestConnectionBootEntriesRedactEveryElementDSN(t *testing.T) {
	// configbind expands ${VAR} while merging the file, so what reaches the
	// startup summary is the resolved secret. Redaction has to survive the
	// indexed key an array element produces.
	element := func(pairs map[string]string) *configbind.Overlay {
		table := configbind.NewOverlay()
		for key, value := range pairs {
			table.Set(key, value, configbind.PlaceFile)
		}
		return table
	}
	overlay := configbind.NewOverlay()
	overlay.SetTables("middleware.rdb.connections", []*configbind.Overlay{
		element(map[string]string{"group": "writer", "dsn": "postgres://app:s3cret@writer/app"}),
		element(map[string]string{"group": "replica", "dsn": "postgres://app:s3cret@replica/app", "readonly": "true"}),
	}, configbind.PlaceFile)

	// bootEntries takes a LoadResult now and hands array elements to this, which
	// is the step the indexed key and its redaction belong to.
	arrayEntry, ok := overlay.Get("middleware.rdb.connections")
	if !ok {
		t.Fatal("the connections array is missing from the overlay")
	}
	entries := tableArrayEntries("middleware.rdb.connections", arrayEntry)
	found := map[string]string{}
	for _, entry := range entries {
		found[entry.key] = entry.value
	}
	for _, key := range []string{
		"middleware.rdb.connections[0].dsn",
		"middleware.rdb.connections[1].dsn",
	} {
		value, ok := found[key]
		if !ok {
			t.Fatalf("%s missing from the startup summary: %v", key, found)
		}
		if value != redactedValue {
			t.Fatalf("%s = %q, want it redacted", key, value)
		}
	}
	// Everything else about a connection is operational detail worth reporting.
	if found["middleware.rdb.connections[1].readonly"] != "true" {
		t.Fatalf("readonly was not reported: %v", found)
	}
	if found["middleware.rdb.connections[0].group"] != "writer" {
		t.Fatalf("group was not reported: %v", found)
	}
}

func TestScaffoldTOMLRendersTheConnectionsBlock(t *testing.T) {
	toml, err := ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"[[middleware.rdb.connections]]", "group = \"\"", "readonly = false",
		"default_group", "write_group", "migration_group",
	} {
		if !strings.Contains(toml, fragment) {
			t.Fatalf("TOML scaffold missing %q:\n%s", fragment, toml)
		}
	}
}

func writer(dsn string) RDBConnectionConfig {
	return RDBConnectionConfig{Group: "writer", DSN: dsn, ConnectTimeout: 5 * time.Second}
}

func replica(dsn string) RDBConnectionConfig {
	return RDBConnectionConfig{Group: "replica", DSN: dsn, ReadOnly: true, ConnectTimeout: 5 * time.Second}
}

func TestResolveRDBConnectionsExpandsLegacyForm(t *testing.T) {
	connections, err := resolveRDBConnections(RDBConfig{
		Enabled:      true,
		DSN:          "sqlite://app.db",
		MaxOpenConns: 4,
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
		t.Fatal("the legacy single database was marked read-only")
	}
}

func TestResolveRDBConnectionsRejectsBothForms(t *testing.T) {
	_, err := resolveRDBConnections(RDBConfig{
		Enabled:     true,
		DSN:         "sqlite://app.db",
		Connections: []RDBConnectionConfig{writer("sqlite://writer.db")},
	})
	if err == nil {
		t.Fatal("both configuration forms were accepted")
	}
	if !strings.Contains(err.Error(), "middleware.rdb.dsn") || !strings.Contains(err.Error(), "middleware.rdb.connections") {
		t.Fatalf("err = %v, want both key paths named", err)
	}
}

func TestResolveRDBConnectionsRequiresAConnection(t *testing.T) {
	if _, err := resolveRDBConnections(RDBConfig{Enabled: true}); err == nil {
		t.Fatal("an enabled rdb without any connection was accepted")
	}
}

func TestResolveRDBConnectionsNamesAnUnnamedGroupDefault(t *testing.T) {
	// One connection that names no group is the common single-database case, so
	// it takes the default group rather than failing.
	connections, err := resolveRDBConnections(RDBConfig{
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
	if _, err := resolveDefaultGroup(RDBConfig{Connections: connections}, connections); err == nil {
		t.Fatal("two groups without default_group were accepted")
	}
	group, err := resolveDefaultGroup(RDBConfig{DefaultGroup: "replica", Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "replica" {
		t.Fatalf("group = %q", group)
	}
	if _, err := resolveDefaultGroup(RDBConfig{DefaultGroup: "nowhere", Connections: connections}, connections); err == nil {
		t.Fatal("default_group naming no group was accepted")
	}
	// One group needs no pointer at all.
	single := []RDBConnectionConfig{writer("sqlite://w.db")}
	if group, err = resolveDefaultGroup(RDBConfig{Connections: single}, single); err != nil || group != "writer" {
		t.Fatalf("group = %q, err = %v", group, err)
	}
}

func TestResolveWriteGroup(t *testing.T) {
	connections := []RDBConnectionConfig{writer("sqlite://w.db"), replica("sqlite://r.db")}
	// One writable group is unambiguous, so write_group may stay unset.
	group, err := resolveWriteGroup(RDBConfig{Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "writer" {
		t.Fatalf("group = %q", group)
	}
	if _, err := resolveWriteGroup(RDBConfig{WriteGroup: "replica", Connections: connections}, connections); err == nil {
		t.Fatal("a read-only group was accepted as the write group")
	}
	readOnly := []RDBConnectionConfig{replica("sqlite://r.db")}
	if _, err := resolveWriteGroup(RDBConfig{Connections: readOnly}, readOnly); err == nil {
		t.Fatal("a topology with no writable connection was accepted")
	}
	ambiguous := []RDBConnectionConfig{
		writer("sqlite://w.db"),
		{Group: "reporting", DSN: "sqlite://rep.db", ConnectTimeout: time.Second},
	}
	if _, err := resolveWriteGroup(RDBConfig{Connections: ambiguous}, ambiguous); err == nil {
		t.Fatal("two writable groups without write_group were accepted")
	}
}

func TestResolveMigrationGroupFallsBackToWriteGroup(t *testing.T) {
	connections := []RDBConnectionConfig{writer("sqlite://w.db"), replica("sqlite://r.db")}
	group, err := resolveMigrationGroup(RDBConfig{Connections: connections}, connections)
	if err != nil {
		t.Fatal(err)
	}
	if group != "writer" {
		t.Fatalf("group = %q", group)
	}
	if _, err := resolveMigrationGroup(RDBConfig{MigrationGroup: "replica", Connections: connections}, connections); err == nil {
		t.Fatal("migrations were pointed at a read-only group")
	}
}

func TestValidateRDBConfigRejectsBadConnectionSets(t *testing.T) {
	base := func(connections ...RDBConnectionConfig) RDBConfig {
		return RDBConfig{Enabled: true, DefaultGroup: "replica", Connections: connections}
	}
	for name, config := range map[string]RDBConfig{
		"upper-case group": base(RDBConnectionConfig{Group: "Writer", DSN: "sqlite://w.db", ConnectTimeout: time.Second}),
		"mixed drivers": base(
			RDBConnectionConfig{Group: "replica", DSN: "sqlite://r.db", ConnectTimeout: time.Second, ReadOnly: true},
			RDBConnectionConfig{Group: "replica", DSN: "postgres://host/db", ConnectTimeout: time.Second, ReadOnly: true},
			writer("sqlite://w.db"),
		),
		"shared memory database": base(
			RDBConnectionConfig{Group: "replica", DSN: "sqlite://:memory:", ConnectTimeout: time.Second, MaxOpenConns: 1, ReadOnly: true},
			RDBConnectionConfig{Group: "replica", DSN: "sqlite://:memory:", ConnectTimeout: time.Second, MaxOpenConns: 1, ReadOnly: true},
			writer("sqlite://w.db"),
		),
		"idle above open": base(RDBConnectionConfig{
			Group: "replica", DSN: "sqlite://r.db", ConnectTimeout: time.Second,
			MaxOpenConns: 2, MaxIdleConns: 3, ReadOnly: true,
		}, writer("sqlite://w.db")),
		"zero connect timeout": base(RDBConnectionConfig{Group: "replica", DSN: "sqlite://r.db", ReadOnly: true}),
		"malformed dsn":        base(RDBConnectionConfig{Group: "replica", DSN: "r.db", ConnectTimeout: time.Second, ReadOnly: true}),
	} {
		if err := validateRDBConfig(config); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestValidateRDBConfigAcceptsAReaderWriterTopology(t *testing.T) {
	config := RDBConfig{
		Enabled:      true,
		DefaultGroup: "replica",
		Connections: []RDBConnectionConfig{
			writer("sqlite://writer.db"),
			replica("sqlite://replica-1.db"),
			replica("sqlite://replica-2.db"),
		},
	}
	if err := validateRDBConfig(config); err != nil {
		t.Fatal(err)
	}
	// The legacy single-database form keeps validating unchanged.
	legacy := RDBConfig{Enabled: true, DSN: "sqlite://app.db", ConnectTimeout: 5 * time.Second}
	if err := validateRDBConfig(legacy); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRuntimeConnectionsOpensEveryPool(t *testing.T) {
	dir := t.TempDir()
	config := RDBConfig{
		Enabled:      true,
		DefaultGroup: "replica",
		Connections: []RDBConnectionConfig{
			writer("sqlite://" + filepath.Join(dir, "writer.db")),
			replica("sqlite://" + filepath.Join(dir, "replica-1.db")),
			replica("sqlite://" + filepath.Join(dir, "replica-2.db")),
		},
	}
	set, err := openRuntimeConnections(config)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	if set.DefaultGroup() != "replica" {
		t.Fatalf("default group = %q", set.DefaultGroup())
	}
	labels := make([]string, 0, 3)
	for _, connection := range set.Connections() {
		labels = append(labels, connection.Label)
	}
	want := []string{"writer#1", "replica#1", "replica#2"}
	for index := range want {
		if labels[index] != want[index] {
			t.Fatalf("labels = %v, want %v", labels, want)
		}
	}
	if set.Collapsed() {
		t.Fatal("a three-connection set reported itself collapsed")
	}
	// A failure closes what was already opened rather than leaving a partial
	// set behind, so the error is the only thing the caller has to handle.
	broken := config
	broken.Connections = append(append([]RDBConnectionConfig{}, config.Connections...),
		RDBConnectionConfig{Group: "writer", DSN: "nosuchdriver://host/db", ConnectTimeout: time.Second})
	if _, err := openRuntimeConnections(broken); err == nil {
		t.Fatal("an unregistered driver was accepted")
	}
}

func TestConfiguredDatabaseDSNUsesTheMigrationGroup(t *testing.T) {
	replaceMiddlewareConfig(t, RDBConfig{
		Enabled:      true,
		DefaultGroup: "replica",
		Connections: []RDBConnectionConfig{
			replica("sqlite://replica.db"),
			writer("sqlite://writer.db"),
		},
	})
	dsn, err := configuredDatabaseDSN()
	if err != nil {
		t.Fatal(err)
	}
	if dsn != "sqlite://writer.db" {
		t.Fatalf("dsn = %q, want the writer group", dsn)
	}
}
