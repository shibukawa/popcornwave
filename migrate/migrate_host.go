//go:build !tinygo && !force_tinygo_logic

package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/shibukawa/popcornwave/internal/pwmigrate"
)

// Delegated reports whether migration work runs in a pw child process.
const Delegated = false

func sourceFS(resolved settings) (fs.FS, error) {
	if resolved.fsys != nil {
		return resolved.fsys, nil
	}
	sources, err := pwmigrate.Sources(resolved.dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return sources, nil
}

// Snapshot applies every migration to a throwaway database and returns SQL that
// reproduces the migrated schema, data, and recorded versions.
func Snapshot(ctx context.Context, options ...Option) (string, error) {
	resolved, err := resolve(options)
	if err != nil {
		return "", err
	}
	sources, err := sourceFS(resolved)
	if err != nil {
		return "", err
	}
	script, err := pwmigrate.Snapshot(ctx, sources)
	if err != nil {
		return "", fmt.Errorf("migrate: %w", err)
	}
	return script, nil
}

// Up applies every pending migration to the configured database.
func Up(ctx context.Context, dsn string, options ...Option) (Result, error) {
	resolved, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	sources, err := sourceFS(resolved)
	if err != nil {
		return Result{}, err
	}
	if dsn == "" {
		dsn = os.Getenv(DSNEnv)
	}
	target, err := pwmigrate.Open(dsn)
	if err != nil {
		return Result{}, fmt.Errorf("migrate: %w", err)
	}
	defer target.Close()
	applied, err := pwmigrate.Apply(ctx, target, sources, pwmigrate.ActionUp, 0)
	result := Result{Previous: applied.Previous, Current: applied.Current}
	for _, item := range applied.Applied {
		result.Applied = append(result.Applied, Applied(item))
	}
	if err != nil {
		return result, fmt.Errorf("migrate: %w", err)
	}
	return result, nil
}

// Status reports every known migration and whether it is applied.
func Status(ctx context.Context, dsn string, options ...Option) ([]State, error) {
	resolved, err := resolve(options)
	if err != nil {
		return nil, err
	}
	sources, err := sourceFS(resolved)
	if err != nil {
		return nil, err
	}
	if dsn == "" {
		dsn = os.Getenv(DSNEnv)
	}
	target, err := pwmigrate.Open(dsn)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	defer target.Close()
	reported, err := pwmigrate.Statuses(ctx, target, sources)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	states := make([]State, 0, len(reported))
	for _, item := range reported {
		states = append(states, State(item))
	}
	return states, nil
}
