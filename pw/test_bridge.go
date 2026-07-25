package pw

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/shibukawa/popcornwave/internal/pwtestbridge"
	"github.com/shibukawa/popcornwave/pwruntime"
)

func init() {
	pwtestbridge.Register(pwtestbridge.Hooks{
		Snapshot: snapshotTestConfigs,
		Prepare:  prepareTestRuntime,
	})
}

func snapshotTestConfigs() (pwtestbridge.Configs, error) {
	if err := ParseConfig(); err != nil {
		return nil, err
	}
	configState.RLock()
	defer configState.RUnlock()
	configs := make(pwtestbridge.Configs, len(configState.entries))
	for typ, entry := range configState.entries {
		value := reflect.ValueOf(entry.ptr)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			configs[typ] = value.Elem().Interface()
		}
	}
	return configs, nil
}

func prepareTestRuntime(handler http.Handler, configs pwtestbridge.Configs, options pwtestbridge.Options) (pwtestbridge.Prepared, error) {
	if handler == nil {
		return pwtestbridge.Prepared{}, fmt.Errorf("popcornwave: nil test handler")
	}
	server := testConfigValue[ServerConfig](configs)
	security := testConfigValue[SecurityConfig](configs)
	middleware := testConfigValue[MiddlewareConfig](configs)
	observability := testConfigValue[ObservabilityConfig](configs)
	if err := validateRuntimeConfig(server, security, middleware, observability); err != nil {
		return pwtestbridge.Prepared{}, err
	}
	if err := validateOperationalEndpointCollisions(handler, server); err != nil {
		return pwtestbridge.Prepared{}, err
	}
	// Savepoint support decides whether a test transaction can host the
	// application's own transactions, so it is checked before opening a pool.
	if options.Transaction {
		if !middleware.RDB.Enabled {
			return pwtestbridge.Prepared{}, fmt.Errorf("popcornwave: test transaction requires middleware.rdb.enabled")
		}
		configured, _, err := databaseTarget(middleware.RDB.DSN)
		if err != nil {
			return pwtestbridge.Prepared{}, err
		}
		if !pwruntime.SupportsSavepoint(configured) {
			return pwtestbridge.Prepared{}, fmt.Errorf("popcornwave: test transaction requires a driver with savepoint support, got %q", configured)
		}
	}
	var dbClose func() error
	var db = (*sql.DB)(nil)
	var driver string
	if middleware.RDB.Enabled {
		var err error
		db, driver, err = openRuntimeDatabase(middleware.RDB)
		if err != nil {
			return pwtestbridge.Prepared{}, err
		}
		dbClose = db.Close
	}
	var scope *pwruntime.TransactionScope
	if options.Transaction {
		scope = pwruntime.NewTransactionScope(db, driver)
	}
	resources := pwruntime.Resources{
		Configs:  map[reflect.Type]any(configs),
		Logger:   slog.Default(),
		DB:       db,
		DBDriver: driver,
		TxScope:  scope,
	}
	wrapped, err := buildRuntimeHandler(handler, server, security, middleware, resources)
	if err != nil {
		if dbClose != nil {
			_ = dbClose()
		}
		return pwtestbridge.Prepared{}, err
	}
	if dbClose == nil {
		dbClose = func() error { return nil }
	}
	return pwtestbridge.Prepared{
		Handler:   wrapped,
		DB:        db,
		Driver:    driver,
		TxScope:   scope,
		Resources: resources,
		Close:     dbClose,
	}, nil
}

func testConfigValue[T any](configs pwtestbridge.Configs) T {
	if value, ok := configs[reflect.TypeFor[T]()].(T); ok {
		return value
	}
	var zero T
	return zero
}
