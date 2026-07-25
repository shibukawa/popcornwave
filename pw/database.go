package pw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/shibukawa/tinygodriver/database/sqlite"
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
	db, err := openRuntimeDatabase(config)
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
	configState.cleanups = append(configState.cleanups, &runtimeCleanup{
		name: "database",
		fn: func(context.Context) error {
			return db.Close()
		},
	})
	return nil
}

func openRuntimeDatabase(config RDBConfig) (*sql.DB, error) {
	driver, dsn, err := databaseTarget(config.DSN)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("popcornwave: open database: %w", err)
	}
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("popcornwave: connect database: %w", err)
	}
	return db, nil
}

func databaseTarget(configured string) (driver, dsn string, err error) {
	configured = strings.TrimSpace(configured)
	driver, remainder, ok := strings.Cut(configured, "://")
	if !ok || driver == "" || remainder == "" {
		return "", "", fmt.Errorf("middleware.rdb.dsn must use driver://dsn syntax")
	}
	if driver == "sqlite" {
		return driver, remainder, nil
	}
	return driver, configured, nil
}

// configuredDatabaseDSN reports the effective middleware.rdb DSN so system:pw-cli
// can migrate without reimplementing configuration precedence.
func configuredDatabaseDSN() (string, error) {
	config := Config[MiddlewareConfig](nil).RDB
	if !config.Enabled {
		return "", errors.New("popcornwave: middleware.rdb.enabled is false")
	}
	if _, _, err := databaseTarget(config.DSN); err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	return config.DSN, nil
}
