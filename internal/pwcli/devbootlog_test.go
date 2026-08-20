package pwcli

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/internal/bootblock"
)

// bootSummary is what an application prints on its way up. Only the rows differ
// between two of them here, which is the case the developer loop exists to make
// quiet.
func bootSummary(rows []string, listening string) string {
	lines := []string{
		bootblock.Art[0],
		bootblock.Art[1] + "   Popcorn Web 0.1.0",
		bootblock.Art[2] + "   started at 2026-08-12 09:00:00 JST",
		bootblock.Art[3] + "   env dev · config.dev.toml",
		bootblock.Art[4],
		"",
		"configuration",
	}
	lines = append(lines, rows...)
	if listening != "" {
		lines = append(lines, "", "listening on "+listening)
	}
	return strings.Join(lines, "\n") + "\n"
}

var bootSummaryRows = []string{
	"├─ html",
	"│  └─ bot_async_timeout      5s              ← file",
	"└─ server",
	"   └─ port                   8080            ← flag",
}

func newTestBootLog(out *syncBuffer) *devBootLog {
	log := newDevBootLog(out)
	log.idle = 20 * time.Millisecond
	return log
}

func TestDevBootLogPrintsTheFirstSummaryUnchanged(t *testing.T) {
	out := &syncBuffer{}
	summary := bootSummary(bootSummaryRows, "http://localhost:8080")
	if _, err := newTestBootLog(out).Write([]byte(summary)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.String() != summary {
		t.Fatalf("first summary came out as\n%s\nwant\n%s", out, summary)
	}
}

func TestDevBootLogReportsARestartThatChangedNothing(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	summary := bootSummary(bootSummaryRows, "http://localhost:8080")
	log.Write([]byte(summary))
	out.Reset()
	log.Write([]byte(summary))
	if out.String() != "reloaded\n" {
		t.Fatalf("restart printed\n%s\nwant just reloaded", out)
	}
}

func TestDevBootLogReportsOnlyTheRowThatChanged(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	log.Write([]byte(bootSummary(bootSummaryRows, "http://localhost:8080")))
	out.Reset()

	changed := append([]string{}, bootSummaryRows...)
	changed[1] = "│  └─ bot_async_timeout      10s             ← file"
	log.Write([]byte(bootSummary(changed, "http://localhost:8080")))

	rendered := out.String()
	for _, want := range []string{"html", "bot_async_timeout", "5s → 10s"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("report missing %q:\n%s", want, rendered)
		}
	}
	for _, unwanted := range []string{"Popcorn Web", "server", "port", "listening"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("report still carries %q:\n%s", unwanted, rendered)
		}
	}
}

// Everything that is not a startup summary is the application talking, and it
// reaches the terminal in the order it was said.
func TestDevBootLogPassesEverythingElseThrough(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	log.Write([]byte("go: downloading\n" + bootSummary(bootSummaryRows, "http://localhost:8080") + "level=INFO msg=ready\n"))
	rendered := out.String()
	if !strings.HasPrefix(rendered, "go: downloading\n") {
		t.Fatalf("output does not start with what came before the summary:\n%s", rendered)
	}
	if !strings.HasSuffix(rendered, "level=INFO msg=ready\n") {
		t.Fatalf("output does not end with what came after the summary:\n%s", rendered)
	}
}

// A summary that never finished is still the only thing the application managed
// to say. It is released rather than held, whatever it cost the report.
func TestDevBootLogReleasesAnUnfinishedSummary(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	log.Write([]byte(bootblock.Art[0] + "\n" + bootblock.Art[1] + "   Popcorn Web 0.1.0\npanic: "))
	log.Flush()
	rendered := out.String()
	if !strings.Contains(rendered, bootblock.Art[1]) || !strings.HasSuffix(rendered, "panic: ") {
		t.Fatalf("flush lost part of what was written:\n%q", rendered)
	}
}

// A summary emitted by Middlewares ends at nothing, so silence has to end it or
// the application's own startup summary would never appear.
func TestDevBootLogReleasesASummaryThatSilenceEnded(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	summary := bootSummary(bootSummaryRows, "")
	log.Write([]byte(summary))
	waitFor(t, func() bool { return strings.Contains(out.String(), "8080") })
	out.Reset()

	log.Write([]byte(summary))
	waitFor(t, func() bool { return out.String() == "reloaded\n" })
}

// A summary this cannot read is printed as it arrived. The developer loses the
// reload report, which is the thing that may be added; the summary is the thing
// that was already there.
func TestDevBootLogPrintsASummaryItCannotRead(t *testing.T) {
	out := &syncBuffer{}
	log := newTestBootLog(out)
	unreadable := strings.Replace(bootSummary(bootSummaryRows, "http://localhost:8080"),
		"env dev · config.dev.toml", "running in dev", 1)
	log.Write([]byte(unreadable))
	log.Write([]byte(unreadable))
	if count := strings.Count(out.String(), "configuration"); count != 2 {
		t.Fatalf("summary printed %d times, want both:\n%s", count, out)
	}
	if strings.Contains(out.String(), "reloaded") {
		t.Fatalf("an unreadable summary was reported as a reload:\n%s", out)
	}
}

// The application writes to a pipe now, so it cannot see the terminal on the
// other side of the loop. It is told what it can no longer work out.
func TestBootLogEnvironPinsTheFormatAndTheColor(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornweb.toml": "[project]\nname = \"app\"\n"})
	environ := bootLogEnviron(root, true, nil)
	if !contains(environ, bootLogVar+"=tree") {
		t.Fatalf("environment does not pin the boot log: %v", environ)
	}
	if !contains(environ, "CLICOLOR_FORCE=1") {
		t.Fatalf("environment does not ask for color: %v", environ)
	}
	if environ := bootLogEnviron(root, false, nil); contains(environ, "CLICOLOR_FORCE=1") {
		t.Fatalf("color was forced onto output that is not a terminal: %v", environ)
	}
}

// Pinning the format is compensation for a pipe, not a decision about what the
// developer wanted. An explicit setting outranks it, including off.
func TestBootLogEnvironLeavesAConfiguredFormatAlone(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornweb.toml":   "[project]\nname = \"app\"\n",
		"config.dev.toml":   "[observability]\nboot_log = \"off\"\n",
		"cmd/app/main.go":   "package main\n\nfunc main() {}\n",
		"config/.gitignore": "",
	})
	if environ := bootLogEnviron(root, false, nil); contains(environ, bootLogVar+"=tree") {
		t.Fatalf("a configured boot log was overridden: %v", environ)
	}
	t.Setenv(bootLogVar, "record")
	root = writeProject(t, map[string]string{"popcornweb.toml": "[project]\nname = \"app\"\n"})
	if environ := bootLogEnviron(root, false, nil); contains(environ, bootLogVar+"=tree") {
		t.Fatalf("a boot log set in the shell was overridden: %v", environ)
	}
}

func contains(environ []string, want string) bool {
	for _, value := range environ {
		if value == want {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if done() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the summary to be released")
}

// syncBuffer is what these tests collect output in. The idle timer releases a
// held summary from its own goroutine, so the buffer is written and read from
// two at once.
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *syncBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer.Reset()
}
