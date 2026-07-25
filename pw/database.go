package pw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/internal/dbschema"
	"github.com/shibukawa/popcornwave/internal/dbseed"
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
	db, driver, err := openRuntimeDatabase(config)
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
	configState.dbDriver = driver
	configState.cleanups = append(configState.cleanups, &runtimeCleanup{
		name: "database",
		fn: func(context.Context) error {
			return db.Close()
		},
	})
	return nil
}

func openRuntimeDatabase(config RDBConfig) (*sql.DB, string, error) {
	driver, dsn, err := databaseTarget(config.DSN)
	if err != nil {
		return nil, "", err
	}
	db, err := sql.Open(driver, dsn)
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
	return db, driver, nil
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

func seedDatabase(paths []string) error {
	configState.RLock()
	db := configState.db
	configState.RUnlock()
	if db == nil {
		return errors.New("popcornwave: seed requires middleware.rdb.enabled")
	}
	dialect, err := dbseed.ResolveDialect(Config[MiddlewareConfig](nil).RDB.DSN)
	if err != nil {
		return fmt.Errorf("popcornwave: seed: %w", err)
	}
	if err := dbseed.Apply(context.Background(), db, dialect, paths); err != nil {
		return fmt.Errorf("popcornwave: seed: %w", err)
	}
	return nil
}

func initializeSchema(directory string) error {
	configState.RLock()
	db := configState.db
	configState.RUnlock()
	if db == nil {
		return errors.New("popcornwave: schema-init requires middleware.rdb.enabled")
	}
	if err := dbschema.Apply(context.Background(), db, directory); err != nil {
		return fmt.Errorf("popcornwave: initialize schema: %w", err)
	}
	return nil
}
