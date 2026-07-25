//go:build tinygo || force_tinygo_logic

package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Delegated reports whether migration work runs in a pw child process.
const Delegated = true

// command is the host tool that carries the migration engine.
const command = "pw"

// materialize gives the child process a readable directory. An embedded tree is
// written to a temporary directory because a child cannot share an fs.FS.
func materialize(resolved settings) (dir string, cleanup func(), err error) {
	if resolved.fsys == nil {
		return resolved.dir, func() {}, nil
	}
	temporary, err := os.MkdirTemp("", "pw-migrate-")
	if err != nil {
		return "", nil, fmt.Errorf("migrate: migration workspace: %w", err)
	}
	entries, err := fs.ReadDir(resolved.fsys, ".")
	if err != nil {
		os.RemoveAll(temporary)
		return "", nil, fmt.Errorf("migrate: read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			continue
		}
		body, err := fs.ReadFile(resolved.fsys, entry.Name())
		if err != nil {
			os.RemoveAll(temporary)
			return "", nil, fmt.Errorf("migrate: read %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(temporary, entry.Name()), body, 0o600); err != nil {
			os.RemoveAll(temporary)
			return "", nil, fmt.Errorf("migrate: stage %s: %w", entry.Name(), err)
		}
	}
	return temporary, func() { os.RemoveAll(temporary) }, nil
}

func run(ctx context.Context, dsn string, resolved settings, args ...string) (string, error) {
	dir, cleanup, err := materialize(resolved)
	if err != nil {
		return "", err
	}
	defer cleanup()
	arguments := append([]string{"migrate"}, args...)
	arguments = append(arguments, "--dir", dir)
	child := exec.CommandContext(ctx, command, arguments...)
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	child.Env = os.Environ()
	if dsn != "" {
		// The DSN travels in the environment, never in a process argument.
		child.Env = append(child.Env, DSNEnv+"="+dsn)
	}
	if err := child.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("migrate: the %s command is required on PATH for a TinyGo build: %w", command, err)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("migrate: %s: %s", strings.Join(arguments, " "), message)
	}
	return stdout.String(), nil
}

// Snapshot applies every migration to a throwaway database and returns SQL that
// reproduces the migrated schema, data, and recorded versions.
func Snapshot(ctx context.Context, options ...Option) (string, error) {
	resolved, err := resolve(options)
	if err != nil {
		return "", err
	}
	return run(ctx, "", resolved, "snapshot")
}

// Up applies every pending migration to the configured database.
func Up(ctx context.Context, dsn string, options ...Option) (Result, error) {
	resolved, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	output, err := run(ctx, dsn, resolved, "up")
	result := parseVersions(output)
	if err != nil {
		return result, err
	}
	return result, nil
}

// Status reports every known migration and whether it is applied.
func Status(ctx context.Context, dsn string, options ...Option) ([]State, error) {
	resolved, err := resolve(options)
	if err != nil {
		return nil, err
	}
	output, err := run(ctx, dsn, resolved, "status")
	if err != nil {
		return nil, err
	}
	var states []State
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) != 3 {
			continue
		}
		version, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		state := State{Version: version, Path: fields[1], Applied: fields[2] != "pending"}
		if state.Applied {
			if at, err := time.Parse("2006-01-02 15:04:05", fields[2]); err == nil {
				state.AppliedAt = at
			}
		}
		states = append(states, state)
	}
	return states, nil
}

// parseVersions reads the trailing "version A -> B" line reported by pw migrate.
func parseVersions(output string) Result {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "version ") {
			continue
		}
		previous, current, ok := strings.Cut(strings.TrimPrefix(line, "version "), " -> ")
		if !ok {
			continue
		}
		from, fromErr := strconv.ParseInt(strings.TrimSpace(previous), 10, 64)
		to, toErr := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
		if fromErr == nil && toErr == nil {
			return Result{Previous: from, Current: to}
		}
	}
	return Result{}
}
