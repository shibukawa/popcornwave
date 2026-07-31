package rdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// Importing this package registers the rdb session backend:
//
//	import _ "github.com/shibukawa/popcornwave/plugin/session/rdb"
//
// Registration opens nothing. The database is borrowed from the RDB middleware
// when session.backend selects this backend at startup.
func init() {
	pw.RegisterSessionBackend(pw.SessionBackendRDB, open)
}

// schemaTimeout bounds the startup schema check.
const schemaTimeout = 10 * time.Second

// open builds the store and verifies the table it owns before the application
// serves a request. A deployment that skipped the migration learns it here,
// with the migration named, instead of at the first login.
func open(ctx context.Context, config pw.SessionConfig, resources pw.SessionResources) (session.Backend, error) {
	if config.RDB.Source != "" && config.RDB.Source != "middleware" {
		return session.Backend{}, fmt.Errorf("session.rdb.source %q is not implemented; use \"middleware\"", config.RDB.Source)
	}
	if resources.DB == nil {
		return session.Backend{}, errors.New(`session.backend = "rdb" requires middleware.rdb.enabled = true`)
	}
	store, err := NewStore(resources.DB, Options{Table: config.RDB.Table})
	if err != nil {
		return session.Backend{}, err
	}
	schemaCtx, cancel := context.WithTimeout(ctx, schemaTimeout)
	defer cancel()
	if err := store.VerifySchema(schemaCtx); err != nil {
		if errors.Is(err, ErrSchemaMissing) {
			return session.Backend{}, fmt.Errorf(
				"%w: apply the migration named %s with pw migrate up", err, MigrationName)
		}
		return session.Backend{}, fmt.Errorf("session schema: %w", err)
	}
	// The pool belongs to the RDB middleware, so this backend closes nothing
	// and only hands back the sweep its table needs.
	return session.Backend{Store: store, Prune: store.Prune}, nil
}
