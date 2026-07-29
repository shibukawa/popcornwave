package pw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shibukawa/popcornwave/database"
	// SQLite is the scaffold default and the only engine that needs no server,
	// so it stays linked for every project. A project on another engine adds
	// its own blank import, which is what api:cli-init scaffolds.
	_ "github.com/shibukawa/popcornwave/database/sqlite"
)

func validateConfiguredRuntime() error {
	return validateRuntimeConfig(
		Config[ServerConfig](nil),
		Config[SecurityConfig](nil),
		Config[MiddlewareConfig](nil),
		Config[ObservabilityConfig](nil),
	)
}

func initializeRuntimeDatabase() error {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return nil
	}
	configState.RLock()
	alreadyOpen := configState.db != nil
	configState.RUnlock()
	if alreadyOpen {
		return nil
	}
	db, dialect, err := openRuntimeDatabase(config)
	if err != nil {
		return err
	}

	configState.Lock()
	defer configState.Unlock()
	if configState.db != nil {
		_ = db.Close()
		return nil
	}
	configState.db = db
	configState.dbDriver = dialect
	configState.cleanups = append(configState.cleanups, &runtimeCleanup{
		name: "database",
		fn: func(context.Context) error {
			return db.Close()
		},
	})
	return nil
}

func openRuntimeDatabase(config RDBConfig) (*sql.DB, string, error) {
	target, err := databaseTarget(config.DSN)
	if err != nil {
		return nil, "", err
	}
	db, err := target.Open()
	if err != nil {
		return nil, "", fmt.Errorf("popcornwave: open database: %w", err)
	}
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("popcornwave: connect database: %w", err)
	}
	return db, target.Dialect, nil
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

// configuredDatabaseDSN reports the effective middleware.rdb DSN so system:pw-cli
// can migrate and seed without reimplementing configuration precedence.
func configuredDatabaseDSN() (string, error) {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return "", errors.New("popcornwave: middleware.rdb.enabled is false")
	}
	if _, err := databaseTarget(config.DSN); err != nil {
		return "", err
	}
	return config.DSN, nil
}
