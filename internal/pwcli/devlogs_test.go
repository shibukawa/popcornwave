package pwcli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwenv"
)

func TestDevLogCaptureSelectsOneInvocationFileWithoutCreatingIt(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	capture := newDevLogCapture(root, projectConfig{Logs: devLogsConfig{Enabled: true, Directory: ".log"}}, "0123456789abcdef", &stdout)
	if capture == nil {
		t.Fatal("enabled capture returned nil")
	}
	if !strings.HasPrefix(filepath.Base(capture.path), "pw-dev-") || !strings.HasSuffix(capture.path, "-0123456789ab.jsonl") {
		t.Fatalf("unexpected invocation path %q", capture.path)
	}
	if filepath.Dir(capture.path) != filepath.Join(root, ".log") {
		t.Fatalf("directory = %q", filepath.Dir(capture.path))
	}
	if _, err := os.Stat(capture.path); !os.IsNotExist(err) {
		t.Fatalf("selecting a path created the log file: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(capture.path)); !os.IsNotExist(err) {
		t.Fatalf("selecting a path created the log directory: %v", err)
	}
	if !strings.Contains(stdout.String(), ".log/") {
		t.Fatalf("report does not name the project-relative path: %q", &stdout)
	}
}

func TestDevLogCaptureEnvironmentReplacesAStalePath(t *testing.T) {
	capture := &devLogCapture{path: "/project/.log/current.jsonl"}
	environ := capture.environ([]string{"A=B", pwenv.DevLogFileVar + "=/old.jsonl"})
	if strings.Join(environ, "\n") != "A=B\n"+pwenv.DevLogFileVar+"=/project/.log/current.jsonl" {
		t.Fatalf("environment = %#v", environ)
	}
}

func TestDevLogCaptureUsesADifferentFileForAnotherInvocation(t *testing.T) {
	root := t.TempDir()
	config := projectConfig{Logs: devLogsConfig{Enabled: true, Directory: ".log"}}
	first := newDevLogCapture(root, config, "first-run", &bytes.Buffer{})
	second := newDevLogCapture(root, config, "second-run", &bytes.Buffer{})
	if first.path == second.path {
		t.Fatalf("two invocations selected the same file %q", first.path)
	}
}

func TestDisabledDevLogCaptureChangesNothing(t *testing.T) {
	var stdout bytes.Buffer
	if capture := newDevLogCapture(t.TempDir(), projectConfig{Logs: devLogsConfig{Enabled: false}}, "run", &stdout); capture != nil {
		t.Fatal("disabled capture returned a value")
	}
	if stdout.Len() != 0 {
		t.Fatalf("disabled capture reported output: %q", &stdout)
	}
	environ := (*devLogCapture)(nil).environ([]string{"A=B", pwenv.DevLogFileVar + "=/stale.jsonl"})
	if strings.Join(environ, "\n") != "A=B" {
		t.Fatalf("disabled capture retained its private handoff: %#v", environ)
	}
}
