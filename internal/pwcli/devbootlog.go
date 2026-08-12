package pwcli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/internal/bootblock"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/minitoml"
)

// bootLogVar is the generated environment binding for observability.boot_log.
// It is derived rather than spelled out, so a change to how bindings are named
// cannot leave the developer loop pinning a variable nothing reads.
var bootLogVar = configbind.EnvName(cliparser.DefaultLongName("observability", "boot_log"))

// bootLogIdle is how long a held summary waits for a line that would end it.
// A summary emitted by Middlewares has no listening line to close it, and an
// application that then says nothing would otherwise hold its own startup
// summary until it exited.
const bootLogIdle = 250 * time.Millisecond

// devBootLog stands between the application's stderr and the terminal, and
// replaces the startup summary of a restarted process with what changed since
// the previous one.
//
// It holds a summary rather than a stream: everything else the application
// writes passes straight through, and a summary that never completes is
// released rather than swallowed. That matters more than the feature does. An
// application that panics halfway up has said something the developer needs,
// and a reload report is worth nothing next to it.
type devBootLog struct {
	out  io.Writer
	dim  func(string) string
	idle time.Duration

	mu       sync.Mutex
	partial  []byte
	scanner  bootblock.Scanner
	timer    *time.Timer
	previous *bootblock.Report
}

func newDevBootLog(out io.Writer) *devBootLog {
	dim := func(text string) string { return text }
	if terminalWriter(out) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		dim = func(text string) string { return "\x1b[2m" + text + "\x1b[0m" }
	}
	return &devBootLog{out: out, dim: dim, idle: bootLogIdle}
}

func (d *devBootLog) Write(data []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.partial = append(d.partial, data...)
	for {
		index := bytes.IndexByte(d.partial, '\n')
		if index < 0 {
			break
		}
		line := string(d.partial[:index])
		d.partial = d.partial[index+1:]
		d.line(line)
	}
	return len(data), nil
}

// Flush releases whatever is held. The developer loop calls it once the
// application process is gone, which is the last moment an unfinished summary
// can still reach the terminal in the place it belongs.
func (d *devBootLog) Flush() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.scanner.Holding() {
		d.report()
	}
	if len(d.partial) > 0 {
		_, _ = d.out.Write(d.partial)
		d.partial = nil
	}
}

func (d *devBootLog) line(text string) {
	taken, complete := d.scanner.Feed(text)
	switch {
	case taken && complete:
		d.report()
	case taken:
		d.hold()
	case d.scanner.Holding():
		// The summary ended before this line, which is how one without a
		// listening line ends.
		d.report()
		d.writeLine(text)
	default:
		d.writeLine(text)
	}
}

// hold arms the wait for a line that would end the summary, and restarts it on
// every line taken, so the wait is for silence rather than for a deadline.
func (d *devBootLog) hold() {
	if d.timer == nil {
		d.timer = time.AfterFunc(d.idle, d.expire)
		return
	}
	d.timer.Reset(d.idle)
}

func (d *devBootLog) expire() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.scanner.Holding() {
		d.report()
	}
}

// report is the whole decision: the first summary of a run is printed as it
// arrived, and every one after it becomes a difference from the one before.
func (d *devBootLog) report() {
	if d.timer != nil {
		d.timer.Stop()
	}
	lines := d.scanner.Take()
	current, ok := bootblock.Parse(lines)
	if !ok {
		// Something about the summary is no longer what this reads. Printing it
		// unchanged is the behavior this replaces, so the developer loses the
		// reload report rather than the summary.
		d.writeLines(lines)
		d.previous = nil
		return
	}
	previous := d.previous
	d.previous = &current
	if previous == nil {
		d.writeLines(lines)
		return
	}
	facts, changes := bootblock.Facts(*previous, current), bootblock.Diff(*previous, current)
	if len(facts) == 0 && len(changes) == 0 {
		d.writeLine("reloaded")
		return
	}
	var out strings.Builder
	bootblock.Render(&out, facts, changes, d.dim)
	_, _ = io.WriteString(d.out, out.String())
}

func (d *devBootLog) writeLine(text string) {
	_, _ = io.WriteString(d.out, text+"\n")
}

func (d *devBootLog) writeLines(lines []string) {
	for _, line := range lines {
		d.writeLine(line)
	}
}

// bootLogEnviron tells the application process what its own stderr can no longer
// tell it. The loop reads that stderr to report requirement:dev-reload-summary,
// so the pipe in between costs the application both halves of its terminal
// check: observability.boot_log auto resolves to record, and the summary renders
// without color.
//
// Both are put back rather than worked around, and neither overrides the
// developer: an explicit boot_log in the development configuration or in the
// shell is left exactly as it is, including off.
func bootLogEnviron(root string, colored bool, environ []string) []string {
	if _, set := os.LookupEnv(bootLogVar); !set && readDevelopmentBootLog(root) == "" {
		environ = append(environ, bootLogVar+"=tree")
	}
	if colored {
		environ = append(environ, "CLICOLOR_FORCE=1")
	}
	return environ
}

// readDevelopmentBootLog reads observability.boot_log out of the development
// configuration, best effort. Best effort is the right amount of effort: getting
// it wrong costs a banner, and the alternative is the loop resolving the
// application's configuration, which is the thing it cannot do.
func readDevelopmentBootLog(root string) string {
	for _, name := range []string{
		pwenv.FileName(pwenv.Development),
		filepath.Join("config", pwenv.FileName(pwenv.Development)),
	} {
		source, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		document, err := minitoml.Parse(source)
		if err != nil {
			continue
		}
		if value := tomlString(document, "observability.boot_log"); value != "" {
			return value
		}
	}
	return ""
}
