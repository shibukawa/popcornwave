package pw

import (
	"context"

	"github.com/shibukawa/popcornwave/pwdatabase"
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

// initializeRuntimeDatabase opens the configured pools and registers their
// release. Opening is pwdatabase's; what stays here is that a pool this
// process opened is closed by the same shutdown that closes an extension.
func initializeRuntimeDatabase() error {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return nil
	}
	if pwdatabase.Connections() != nil {
		return nil
	}
	if err := pwdatabase.Start(config); err != nil {
		return err
	}
	registerCleanup("database", func(context.Context) error { return pwdatabase.Close() })
	return nil
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
	return pwdatabase.SelectWriteDB(ctx)
}

// SelectSessionDB pins the connection group holding the session table:
// session.rdb.group, falling back to the framework write group.
func SelectSessionDB(ctx context.Context) (context.Context, error) {
	return pwdatabase.SelectSessionDB(ctx)
}

// configuredDatabaseDSN reports the DSN of the migration group so system:pw-cli
// can migrate and seed without reimplementing configuration precedence.
func configuredDatabaseDSN() (string, error) {
	return Config[MiddlewareConfig](nil).RDB.MigrationDSN()
}
