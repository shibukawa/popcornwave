package pw

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/database"
	"github.com/shibukawa/popcornwave/pwruntime"
)

func validateConfiguredRuntime() error {
	if err := validateRuntimeConfig(
		Config[ServerConfig](nil),
		Config[SecurityConfig](nil),
		Config[MiddlewareConfig](nil),
		Config[ObservabilityConfig](nil),
	); err != nil {
		return err
	}
	// The cookie policy is the one rule here that reads the environment: the
	// same value is deliberate on a loopback development machine and a defect
	// anywhere else.
	if err := validateSessionConfig(Config[SessionConfig](nil), Env(), Development()); err != nil {
		return err
	}
	return validateHTMLConfig(Config[HTMLConfig](nil))
}

// resolveRDBConnections expands the configuration into the effective connection
// list, in configuration order.
//
// An element naming no group joins the default group, so the single-database
// project writes one element and names nothing.
func resolveRDBConnections(config RDBConfig) ([]RDBConnectionConfig, error) {
	if len(config.Connections) == 0 {
		// A file written against the removed single-DSN form lands here: the
		// key it sets is claimed by no binding, so the section reads as an
		// enabled database with no pool behind it.
		return nil, errors.New("middleware.rdb.enabled needs at least one [[middleware.rdb.connections]] element; the middleware.rdb.dsn form was removed, so move that DSN into an element")
	}
	connections := make([]RDBConnectionConfig, 0, len(config.Connections))
	for _, connection := range config.Connections {
		if connection.Group == "" {
			connection.Group = pwruntime.DefaultConnectionGroup
		}
		connections = append(connections, connection)
	}
	return connections, nil
}

// connectionGroups lists the configured group names in configuration order.
func connectionGroups(connections []RDBConnectionConfig) []string {
	var groups []string
	seen := make(map[string]bool, len(connections))
	for _, connection := range connections {
		if !seen[connection.Group] {
			seen[connection.Group] = true
			groups = append(groups, connection.Group)
		}
	}
	return groups
}

// writableGroups lists the groups holding at least one writable connection.
func writableGroups(connections []RDBConnectionConfig) []string {
	var groups []string
	seen := make(map[string]bool, len(connections))
	for _, connection := range connections {
		if connection.ReadOnly || seen[connection.Group] {
			continue
		}
		seen[connection.Group] = true
		groups = append(groups, connection.Group)
	}
	return groups
}

// resolveDefaultGroup names the group serving statements that pin no group.
func resolveDefaultGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	groups := connectionGroups(connections)
	if configured := strings.TrimSpace(config.DefaultGroup); configured != "" {
		if !containsString(groups, configured) {
			return "", fmt.Errorf("middleware.rdb.default_group %q names no configured connection group", configured)
		}
		return configured, nil
	}
	if len(groups) != 1 {
		return "", errors.New("middleware.rdb.default_group is required when more than one connection group is configured")
	}
	return groups[0], nil
}

// resolveWriteGroup names the group serving framework-owned writes.
//
// An ambiguous topology is a startup error rather than a silent pick, because
// picking the wrong one writes to a replica.
func resolveWriteGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	return resolveWritableGroup(config, connections, strings.TrimSpace(config.WriteGroup), "middleware.rdb.write_group")
}

// resolveMigrationGroup names the group receiving migrations and seed data.
func resolveMigrationGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	if configured := strings.TrimSpace(config.MigrationGroup); configured != "" {
		return resolveWritableGroup(config, connections, configured, "middleware.rdb.migration_group")
	}
	return resolveWriteGroup(config, connections)
}

func resolveWritableGroup(config RDBConfig, connections []RDBConnectionConfig, configured, key string) (string, error) {
	writable := writableGroups(connections)
	if configured != "" {
		if !containsString(connectionGroups(connections), configured) {
			return "", fmt.Errorf("%s %q names no configured connection group", key, configured)
		}
		if !containsString(writable, configured) {
			return "", fmt.Errorf("%s %q holds no writable connection", key, configured)
		}
		return configured, nil
	}
	switch len(writable) {
	case 0:
		return "", errors.New("middleware.rdb has no writable connection; framework-owned writes have nowhere to go")
	case 1:
		return writable[0], nil
	default:
		return "", fmt.Errorf("%s is required when more than one group holds a writable connection", key)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func initializeRuntimeDatabase() error {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return nil
	}
	configState.RLock()
	alreadyOpen := configState.connections != nil
	configState.RUnlock()
	if alreadyOpen {
		return nil
	}
	set, err := openRuntimeConnections(config)
	if err != nil {
		return err
	}

	configState.Lock()
	defer configState.Unlock()
	if configState.connections != nil {
		_ = set.Close()
		return nil
	}
	configState.connections = set
	// The default group's connection stays the single handle for callers that
	// predate the connection set, such as the readiness probe and test bridges.
	if primary, ok := defaultConnection(set); ok {
		configState.db = primary.DB
		configState.dbDriver = primary.Driver
	}
	configState.cleanups = append(configState.cleanups, &runtimeCleanup{
		name: "database",
		fn: func(context.Context) error {
			return set.Close()
		},
	})
	return nil
}

func defaultConnection(set *pwruntime.ConnectionSet) (*pwruntime.Connection, bool) {
	for _, connection := range set.Connections() {
		if connection.Group == set.DefaultGroup() {
			return connection, true
		}
	}
	return nil, false
}

// openRuntimeConnections opens every configured pool and pings it. A failure
// closes what was already opened, so a partial set never reaches a request.
func openRuntimeConnections(config RDBConfig) (*pwruntime.ConnectionSet, error) {
	connections, err := resolveRDBConnections(config)
	if err != nil {
		return nil, err
	}
	defaultGroup, err := resolveDefaultGroup(config, connections)
	if err != nil {
		return nil, err
	}
	opened := make([]pwruntime.Connection, 0, len(connections))
	closeOpened := func() {
		for _, connection := range opened {
			_ = connection.Close()
		}
	}
	ordinals := make(map[string]int, len(connections))
	for _, connection := range connections {
		ordinals[connection.Group]++
		label := connection.Group + "#" + strconv.Itoa(ordinals[connection.Group])
		runtimeConnection, openErr := openRuntimeDatabase(connection, label)
		if openErr != nil {
			closeOpened()
			return nil, openErr
		}
		opened = append(opened, runtimeConnection)
	}
	set, err := pwruntime.NewConnectionSet(defaultGroup, opened)
	if err != nil {
		closeOpened()
		return nil, err
	}
	return set, nil
}

func openRuntimeDatabase(config RDBConnectionConfig, label string) (pwruntime.Connection, error) {
	connection := pwruntime.Connection{
		Group:    config.Group,
		Label:    label,
		ReadOnly: config.ReadOnly,
	}
	target, err := databaseTarget(config.DSN)
	if err != nil {
		return connection, fmt.Errorf("popcornwave: connection %s: %w", label, err)
	}
	connection.Driver = target.Dialect
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()
	if target.Native() {
		// The bounds travel with the open call, because a native pool is
		// configured at construction rather than adjusted afterwards.
		native, err := target.OpenNative(ctx, database.PoolBounds{
			MaxOpenConns:    config.MaxOpenConns,
			MaxIdleConns:    config.MaxIdleConns,
			ConnMaxLifetime: config.ConnMaxLifetime,
			ConnMaxIdleTime: config.ConnMaxIdleTime,
		})
		if err != nil {
			return connection, fmt.Errorf("popcornwave: open database %s: %w", label, err)
		}
		if err := native.Ping(ctx); err != nil {
			_ = native.Close()
			return connection, fmt.Errorf("popcornwave: connect database %s: %w", label, err)
		}
		connection.Native = native
		return connection, nil
	}
	db, err := target.Open()
	if err != nil {
		return connection, fmt.Errorf("popcornwave: open database %s: %w", label, err)
	}
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return connection, fmt.Errorf("popcornwave: connect database %s: %w", label, err)
	}
	connection.DB = db
	return connection, nil
}

// reportDatabaseConnections names, once at startup, the execution path each
// connection took. The choice is automatic — an engine with a native opener
// always bypasses database/sql — so the log line is what makes the active
// path observable rather than inferred.
func reportDatabaseConnections(connections *pwruntime.ConnectionSet) {
	for _, connection := range connections.Connections() {
		path := "database/sql"
		if connection.Native != nil {
			path = "native"
		}
		processLogger().Info("popcornwave database connection",
			String("connection", connection.Label),
			String("driver", connection.Driver),
			String("path", path),
			Bool("read_only", connection.ReadOnly),
		)
	}
}

// databaseTarget resolves the configured DSN onto the engine that opens it.
// The scheme selects an opener rather than a database/sql driver name, because
// the PostgreSQL engine registers no name to open by.
func databaseTarget(configured string) (database.Target, error) {
	target, err := database.Resolve(configured)
	if err != nil {
		return database.Target{}, fmt.Errorf("popcornwave: %w", err)
	}
	return target, nil
}

// MigrationDSN reports the DSN of the group receiving migrations and seed data.
//
// It is exported because migration and seeding tooling has to reach the schema
// without reimplementing group resolution.
func (config RDBConfig) MigrationDSN() (string, error) {
	if !config.Enabled {
		return "", errors.New("popcornwave: middleware.rdb.enabled is false")
	}
	connections, err := resolveRDBConnections(config)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	group, err := resolveMigrationGroup(config, connections)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	for _, connection := range connections {
		if connection.Group != group {
			continue
		}
		if _, err := databaseTarget(connection.DSN); err != nil {
			return "", fmt.Errorf("popcornwave: %w", err)
		}
		return connection.DSN, nil
	}
	return "", fmt.Errorf("popcornwave: migration group %q has no connection", group)
}

// SelectWriteDB pins the connection group that framework-owned writes use:
// middleware.rdb.write_group, or the only group holding a writable connection.
//
// A replica can never be selected this way, so a caller that must write does
// not have to know the deployment topology.
func SelectWriteDB(ctx context.Context) (context.Context, error) {
	group, err := writableGroupFor(ctx, "", "middleware.rdb.write_group")
	if err != nil {
		return ctx, err
	}
	return pwruntime.SelectDB(ctx, group), nil
}

// SelectSessionDB pins the connection group holding the session table:
// session.rdb.group, falling back to the framework write group.
func SelectSessionDB(ctx context.Context) (context.Context, error) {
	group, err := writableGroupFor(ctx, Config[SessionConfig](ctx).RDB.Group, "session.rdb.group")
	if err != nil {
		return ctx, err
	}
	return pwruntime.SelectDB(ctx, group), nil
}

func writableGroupFor(ctx context.Context, configured, key string) (string, error) {
	config := Config[MiddlewareConfig](ctx).RDB
	if !config.Enabled {
		return "", errors.New("popcornwave: middleware.rdb.enabled is false")
	}
	connections, err := resolveRDBConnections(config)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	group, err := resolveWritableGroup(config, connections, strings.TrimSpace(configured), key)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	return group, nil
}

// configuredDatabaseDSN reports the DSN of the migration group so system:pw-cli
// can migrate and seed without reimplementing configuration precedence.
func configuredDatabaseDSN() (string, error) {
	return Config[MiddlewareConfig](nil).RDB.MigrationDSN()
}
