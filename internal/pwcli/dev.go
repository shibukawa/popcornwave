package pwcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/internal/pwenv"
)

func runDev(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("dev: unexpected arguments")
	}
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopServices := startDevboxServices(ctx, root, stdout, stderr)
	defer stopServices()

	if err := runGenerate(ctx, nil, stdout); err != nil {
		return err
	}
	if err := runDevMigrations(ctx, root, config, stdout, stderr); err != nil {
		return err
	}
	var tailwind *exec.Cmd
	var tailwindExited <-chan error
	if config.Tailwind.Enabled {
		development := config.Tailwind
		development.Minify = false
		if err := buildTailwind(ctx, root, development, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "pw dev:", err)
		}
		tailwind, tailwindExited, err = startTailwindWatch(ctx, root, development, stdout, stderr)
		if err != nil {
			return err
		}
		defer func() { stopCommand(tailwind) }()
	}
	state, err := snapshotWatchFiles(root, configuredWatchPaths(root, config.ExtraWatch,
		append(tailwindWatchPaths(root, config.Tailwind, tailwind == nil),
			migrationWatchPaths(root, config.Migration)...))...)
	if err != nil {
		return err
	}
	app, exited, err := startApplication(ctx, root, config.Main, stdout, stderr)
	if err != nil {
		return err
	}
	defer stopCommand(app)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-exited:
			if err == nil || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("application exited: %w", err)
		case err := <-tailwindExited:
			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintln(stderr, "pw dev: tailwindcss exited:", err)
			}
			tailwind = nil
			tailwindExited = nil
			addWatchFile(state, filepath.Join(root, filepath.FromSlash(config.Tailwind.Input)))
		case <-ticker.C:
			next, err := snapshotWatchFiles(root, configuredWatchPaths(root, config.ExtraWatch,
				append(tailwindWatchPaths(root, config.Tailwind, tailwind == nil),
					migrationWatchPaths(root, config.Migration)...))...)
			if err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
				continue
			}
			if equalWatchState(state, next) {
				continue
			}
			stopCommand(app)
			if config.Tailwind.Enabled && tailwind == nil {
				development := config.Tailwind
				development.Minify = false
				if err := buildTailwind(ctx, root, development, stdout, stderr); err != nil {
					fmt.Fprintln(stderr, "pw dev:", err)
				}
				tailwind, tailwindExited, err = startTailwindWatch(ctx, root, development, stdout, stderr)
				if err != nil {
					fmt.Fprintln(stderr, "pw dev:", err)
					tailwind = nil
					tailwindExited = nil
				}
			}
			if err := runGenerate(ctx, nil, stdout); err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
				state = next
				continue
			}
			if err := runDevMigrations(ctx, root, config, stdout, stderr); err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
				state = next
				continue
			}
			state, _ = snapshotWatchFiles(root, configuredWatchPaths(root, config.ExtraWatch,
				append(tailwindWatchPaths(root, config.Tailwind, tailwind == nil),
					migrationWatchPaths(root, config.Migration)...))...)
			app, exited, err = startApplication(ctx, root, config.Main, stdout, stderr)
			if err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
			}
		}
	}
}

func startApplication(ctx context.Context, root, mainPackage string, stdout, stderr io.Writer) (*exec.Cmd, <-chan error, error) {
	command := exec.CommandContext(ctx, "go", "run", "-tags=pwdev", mainPackage)
	command.Dir, command.Stdout, command.Stderr, command.Stdin, command.Env = root, stdout, stderr, os.Stdin, developmentEnviron()
	if err := command.Start(); err != nil {
		return nil, nil, err
	}
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	return command, result, nil
}

// developmentEnviron runs the application under APP_ENV=dev unless the
// developer already selected another environment.
func developmentEnviron() []string {
	environ := os.Environ()
	if value, ok := os.LookupEnv(pwenv.Var); ok && strings.TrimSpace(value) != "" {
		return environ
	}
	return append(environ, pwenv.Var+"="+pwenv.Development)
}

func stopCommand(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Signal(os.Interrupt)
	select {
	case <-time.After(750 * time.Millisecond):
		_ = command.Process.Kill()
	}
}

func startDevboxServices(ctx context.Context, root string, stdout, stderr io.Writer) func() {
	if _, err := exec.LookPath("devbox"); err != nil {
		fmt.Fprintln(stderr, "pw dev: devbox is not installed; skipping configured services")
		return func() {}
	}
	command := exec.CommandContext(ctx, "devbox", "services", "up")
	command.Dir, command.Stdout, command.Stderr, command.Env = root, stdout, stderr, os.Environ()
	if err := command.Start(); err != nil {
		fmt.Fprintln(stderr, "pw dev: start Devbox services:", err)
		return func() {}
	}
	go func() { _ = command.Wait() }()
	return func() { stopCommand(command) }
}

type watchState map[string]fileState

type fileState struct {
	size    int64
	modTime time.Time
}

func snapshotWatchFiles(root string, extra ...string) (watchState, error) {
	state := watchState{}
	included := make(map[string]bool, len(extra))
	for _, path := range extra {
		if path != "" {
			included[filepath.Clean(path)] = true
		}
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".devbox", "vendor", "node_modules":
				return filepath.SkipDir
			}
			if path == filepath.Join(root, "public") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !included[filepath.Clean(path)] && name != "popcornwave.toml" && !pwenv.IsFileName(name) &&
			!strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, ".pw.html") && !strings.HasSuffix(name, ".pw.sql") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state[path] = fileState{size: info.Size(), modTime: info.ModTime()}
		return nil
	})
	return state, err
}

// migrationWatchPaths adds migration sources to the watch set. They are plain
// .sql files, which the default walk ignores.
func migrationWatchPaths(root string, config migrationConfig) []string {
	directory := config.Dir
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(root, filepath.FromSlash(directory))
	}
	matches, err := filepath.Glob(filepath.Join(directory, "*.sql"))
	if err != nil {
		return nil
	}
	return matches
}

// runDevMigrations applies pending migrations before the application starts. A
// project without a migration directory is not an error.
func runDevMigrations(ctx context.Context, root string, config projectConfig, stdout, stderr io.Writer) error {
	if !config.Migration.Auto {
		return nil
	}
	directory := config.Migration.Dir
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(root, filepath.FromSlash(directory))
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return nil
	}
	return executeMigrate(ctx, project{root: root, config: config}, migrateOptions{action: "up"}, stdout, stderr)
}

func configuredWatchPaths(root string, patterns, additional []string) []string {
	paths := append([]string(nil), additional...)
	for _, pattern := range patterns {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, filepath.FromSlash(pattern))
		}
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			paths = append(paths, matches...)
			continue
		}
		paths = append(paths, filepath.Clean(pattern))
	}
	return paths
}

func tailwindWatchPaths(root string, config tailwindConfig, includeInput bool) []string {
	if !config.Enabled {
		return nil
	}
	var paths []string
	output := filepath.Clean(filepath.Join(root, filepath.FromSlash(config.Output)))
	public := filepath.Join(root, "public")
	if !pathWithin(public, output) {
		paths = append(paths, output)
	}
	if includeInput {
		paths = append(paths, filepath.Join(root, filepath.FromSlash(config.Input)))
	}
	return paths
}

func pathWithin(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func addWatchFile(state watchState, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	state[filepath.Clean(path)] = fileState{size: info.Size(), modTime: info.ModTime()}
}

func equalWatchState(left, right watchState) bool {
	if len(left) != len(right) {
		return false
	}
	for path, value := range left {
		if right[path] != value {
			return false
		}
	}
	return true
}
