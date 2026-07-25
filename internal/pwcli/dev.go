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
	state, err := snapshotWatchFiles(root)
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
		case <-ticker.C:
			next, err := snapshotWatchFiles(root)
			if err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
				continue
			}
			if equalWatchState(state, next) {
				continue
			}
			stopCommand(app)
			if err := runGenerate(ctx, nil, stdout); err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
				state = next
				continue
			}
			state, _ = snapshotWatchFiles(root)
			app, exited, err = startApplication(ctx, root, config.Main, stdout, stderr)
			if err != nil {
				fmt.Fprintln(stderr, "pw dev:", err)
			}
		}
	}
}

func startApplication(ctx context.Context, root, mainPackage string, stdout, stderr io.Writer) (*exec.Cmd, <-chan error, error) {
	command := exec.CommandContext(ctx, "go", "run", mainPackage)
	command.Dir, command.Stdout, command.Stderr, command.Stdin, command.Env = root, stdout, stderr, os.Stdin, os.Environ()
	if err := command.Start(); err != nil {
		return nil, nil, err
	}
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	return command, result, nil
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

func snapshotWatchFiles(root string) (watchState, error) {
	state := watchState{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".devbox", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if name != "popcornwave.toml" && !strings.HasSuffix(name, ".go") &&
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
