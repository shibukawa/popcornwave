package pwcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"syscall"
)

var (
	tailwindImport      = regexp.MustCompile(`(?m)@import\s+["']tailwindcss["']`)
	localTailwindPlugin = regexp.MustCompile(`(?m)@plugin\s+["'](\.{1,2}/[^"']+)["']`)
)

// tailwindToolchainHint names the way this project installs its tools. A
// project without the Devbox environment manages the toolchain itself, so
// telling it to enter a shell that does not exist would send the operator
// looking for a file the scaffold never wrote.
func tailwindToolchainHint(root string) string {
	if _, err := os.Stat(filepath.Join(root, "devbox.json")); err == nil {
		return "enter the configured Devbox shell"
	}
	return "install " + tailwindToolchainRequirement + ", or run pw add devbox"
}

func validateTailwind(root string, config tailwindConfig) (string, string, error) {
	if !config.Enabled {
		return "", "", nil
	}
	if _, err := exec.LookPath("tailwindcss"); err != nil {
		return "", "", fmt.Errorf("Tailwind CSS is enabled but tailwindcss is not available in PATH (%s)",
			tailwindToolchainHint(root))
	}
	input := filepath.Clean(filepath.Join(root, filepath.FromSlash(config.Input)))
	output := filepath.Clean(filepath.Join(root, filepath.FromSlash(config.Output)))
	source, err := os.ReadFile(input)
	if err != nil {
		return "", "", fmt.Errorf("read Tailwind input %s: %w", input, err)
	}
	if !tailwindImport.Match(source) {
		return "", "", fmt.Errorf("Tailwind input %s must contain @import \"tailwindcss\"", input)
	}
	for _, match := range localTailwindPlugin.FindAllSubmatch(source, -1) {
		plugin := filepath.Clean(filepath.Join(filepath.Dir(input), filepath.FromSlash(string(match[1]))))
		if _, err := os.Stat(plugin); err != nil {
			return "", "", fmt.Errorf("Tailwind plugin %s: %w", plugin, err)
		}
	}
	return input, output, nil
}

func buildTailwind(ctx context.Context, root string, config tailwindConfig, stdout, stderr io.Writer) error {
	input, output, err := validateTailwind(root, config)
	if err != nil || !config.Enabled {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create Tailwind output directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(output), "."+filepath.Base(output)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary Tailwind output: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	defer os.Remove(tempPath)

	args := []string{"-i", input, "-o", tempPath}
	if config.Minify {
		args = append(args, "--minify")
	}
	command := exec.CommandContext(ctx, "tailwindcss", args...)
	command.Dir = root
	command.Stdout = newPrefixWriter(stdout, "tailwind: ")
	command.Stderr = newPrefixWriter(stderr, "tailwind: ")
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("tailwindcss: %w", explainSignalledTool("tailwindcss", err))
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("prepare Tailwind output: %w", err)
	}
	if err := os.Rename(tempPath, output); err != nil {
		return fmt.Errorf("replace Tailwind output: %w", err)
	}
	return nil
}

// explainSignalledTool says what a death by signal means, because the error
// os/exec produces does not. "signal: killed" names an outcome and no cause,
// and the tool printed nothing to go with it — not because it failed quietly
// but because it never ran. A developer reading that line has nowhere to go.
//
// On macOS the usual cause is the one the exit status cannot carry. Every arm64
// binary is signed, the kernel validates it at exec, and one that does not
// validate is killed rather than refused with a message. A tool whose installed
// build was modified after signing therefore dies here on every invocation,
// having written nothing — and the project it happened in is not at fault, so
// the remedy is to look at the executable rather than at the configuration.
func explainSignalledTool(name string, err error) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return err
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return err
	}
	if runtime.GOOS != "darwin" || status.Signal() != syscall.SIGKILL {
		return fmt.Errorf("%w: it was ended by a signal rather than exiting, so it produced no diagnosis of its own", err)
	}
	path, lookErr := exec.LookPath(name)
	if lookErr != nil {
		path = name
	}
	return fmt.Errorf(`%w: killed before it ran, which on macOS is how the kernel refuses a code signature that does not validate
  the executable is %s
  check it with codesign --verify --verbose, and if that path is a wrapper script, check the binary it execs
  a signature that does not validate is a fault in that build rather than in this project; install %s from elsewhere`,
		err, path, name)
}

func startTailwindWatch(ctx context.Context, root string, config tailwindConfig, stdout, stderr io.Writer) (*exec.Cmd, <-chan error, error) {
	input, output, err := validateTailwind(root, config)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create Tailwind output directory: %w", err)
	}
	// Started without the context for the same reason the application is: the
	// developer loop stops its children itself, through the group, rather than
	// letting cancellation kill a launcher and orphan what it started.
	command := exec.Command("tailwindcss", "-i", input, "-o", output, "--watch")
	command.Dir = root
	command.Stdout = newPrefixWriter(stdout, "tailwind: ")
	command.Stderr = newPrefixWriter(stderr, "tailwind: ")
	command.Env = os.Environ()
	ownProcessGroup(command)
	if err := command.Start(); err != nil {
		return nil, nil, fmt.Errorf("start tailwindcss: %w", err)
	}
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	return command, result, nil
}

type prefixWriter struct {
	mu        sync.Mutex
	dst       io.Writer
	prefix    []byte
	lineStart bool
}

func newPrefixWriter(dst io.Writer, prefix string) io.Writer {
	return &prefixWriter{dst: dst, prefix: []byte(prefix), lineStart: true}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := len(p)
	var output bytes.Buffer
	for len(p) > 0 {
		if w.lineStart {
			output.Write(w.prefix)
			w.lineStart = false
		}
		index := bytes.IndexByte(p, '\n')
		if index < 0 {
			output.Write(p)
			break
		}
		output.Write(p[:index+1])
		p = p[index+1:]
		w.lineStart = true
	}
	if _, err := w.dst.Write(output.Bytes()); err != nil {
		return 0, err
	}
	return written, nil
}
