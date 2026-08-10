package pw

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/database"
	"github.com/shibukawa/popcornwave/pwconfig"
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

func initializeRuntimeDatabase() error {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return nil
	}
	runtimeState.RLock()
	alreadyOpen := runtimeState.connections != nil
	runtimeState.RUnlock()
	if alreadyOpen {
		return nil
	}
	set, err := openRuntimeConnections(config)
	if err != nil {
		return err
	}

	runtimeState.Lock()
	defer runtimeState.Unlock()
	if runtimeState.connections != nil {
		_ = set.Close()
		return nil
	}
	runtimeState.connections = set
	// The default group's connection stays the single handle for callers that
	// predate the connection set, such as the readiness probe and test bridges.
	if primary, ok := defaultConnection(set); ok {
		runtimeState.db = primary.DB
		runtimeState.dbDriver = primary.Driver
	}
	runtimeState.cleanups = append(runtimeState.cleanups, &runtimeCleanup{
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
	connections, err := pwconfig.ResolveConnections(config)
	if err != nil {
		return nil, err
	}
	defaultGroup, err := pwconfig.ResolveDefaultGroup(config, connections)
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
	target, err := pwconfig.Target(config.DSN)
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
	connections, err := pwconfig.ResolveConnections(config)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	group, err := pwconfig.ResolveWritableGroup(config, connections, strings.TrimSpace(configured), key)
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
