package pwcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubDevbox puts a devbox on PATH whose `services stop` fails with the given
// message, and whose `services up` outlives the loop that started it. The
// project it stands in declares the environment, since a project without one
// starts no services at all.
func stubDevbox(t *testing.T, stopMessage string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "devbox.json"), "{}\n")
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(bin, "devbox")
	// Single-quoted, because the message devbox actually prints holds backticks
	// and a double-quoted shell word would run what is between them.
	writeTestFile(t, tool, `#!/bin/sh
case "$2" in
  ls) echo "postgresql  Running" ;;
  up) sleep 30 ;;
  stop)
    echo '`+stopMessage+`' >&2
    exit 1
    ;;
esac
`)
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root
}

// A developer loop that ends before its services finish coming up has nothing
// to stop. devbox reports that as an error, but the run it belongs to failed
// somewhere else — the message right above it — and repeating a shutdown
// complaint under that reads like the cause of it.
func TestStoppingServicesThatNeverCameUpIsSilent(t *testing.T) {
	root := stubDevbox(t, "Error: Process manager is not running. Run `devbox services up` to start it.")
	var stdout, stderr strings.Builder
	startDevboxServices(context.Background(), root, &stdout, &stderr)()

	if reported := stdout.String() + stderr.String(); reported != "" {
		t.Errorf("a shutdown with nothing to stop reported:\n%s", reported)
	}
}

// Every other failure is one: the services are still running, and a developer
// who does not hear about it finds out from the next run.
func TestAFailedServiceShutdownIsReported(t *testing.T) {
	root := stubDevbox(t, "Error: could not reach the process manager")
	var stdout, stderr strings.Builder
	startDevboxServices(context.Background(), root, &stdout, &stderr)()

	if !strings.Contains(stderr.String(), "stop Devbox services") {
		t.Errorf("a failed shutdown went unreported:\n%s", stderr.String())
	}
	if reported := stdout.String() + stderr.String(); !strings.Contains(reported, "could not reach the process manager") {
		t.Errorf("what devbox said about the failure was dropped:\n%s", reported)
	}
}
