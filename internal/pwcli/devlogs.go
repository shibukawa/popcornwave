package pwcli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/internal/pwenv"
)

// devLogCapture is the one invocation file selected by pw dev. The file stays
// unopened here: the application creates it on its first structured record and
// reopens the same path after every rebuild.
type devLogCapture struct{ path string }

func newDevLogCapture(root string, config projectConfig, runID string, stdout io.Writer) *devLogCapture {
	if !config.Logs.Enabled {
		return nil
	}
	if len(runID) > 12 {
		runID = runID[:12]
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	name := fmt.Sprintf("pw-dev-%s-%s.jsonl", stamp, runID)
	path := filepath.Join(root, filepath.FromSlash(config.Logs.Directory), name)
	relative, err := filepath.Rel(root, path)
	if err != nil {
		relative = path
	}
	fmt.Fprintf(stdout, "pw dev: structured logs %s\n", filepath.ToSlash(relative))
	return &devLogCapture{path: path}
}

func (capture *devLogCapture) environ(base []string) []string {
	// This is a pw-owned handoff, not an operator override. Remove a stale
	// inherited value so a nested shell cannot send a new run into an old file.
	prefix := pwenv.DevLogFileVar + "="
	filtered := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	if capture == nil {
		return filtered
	}
	return append(filtered, prefix+capture.path)
}
