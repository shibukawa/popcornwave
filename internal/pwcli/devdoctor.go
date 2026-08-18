package pwcli

import (
	"bytes"
	"context"
	"fmt"

	"github.com/shibukawa/popcornweb/internal/devconsole"
)

// doctorPane runs api:cli-doctor and shows what it said.
//
// It runs the diagnosis rather than reimplementing it, which is what
// policy:dev-console-boundary asks of anything a pane offers. Doctor is also
// the answer to the question a configuration dump would have been asked for:
// it already merges defaults, environment, and the selected file, already masks
// what policy:startup-summary masks, and already names a remedy for what it
// finds. A raw dump would carry the same values with none of that.
func doctorPane(root string) devconsole.Pane {
	return devconsole.Pane{
		Slug:    "doctor",
		Title:   "doctor",
		Summary: "what this environment would run, and what is missing or inadvisable there",
		Handler: devconsole.TextPane(
			"doctor",
			"pw doctor for the environment the loop is running, read from the project rather than from the running application",
			func(ctx context.Context) (string, error) { return runDoctorReport(ctx, root) },
		),
	}
}

// runDoctorReport diagnoses the development environment and renders the same
// text the command prints.
//
// The offline default is kept: api:cli-doctor contacts nothing unless asked,
// and a pane that reached out on every page load would be a different command
// wearing its name.
func runDoctorReport(ctx context.Context, root string) (string, error) {
	options, err := parseDoctorOptions(nil)
	if err != nil {
		return "", err
	}
	report, err := diagnose(ctx, root, options, processEnviron())
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	// No style: the buffer is not a terminal, and the page renders the plain
	// text the command would print into a pipe.
	writeDoctorText(&out, report, doctorStyle{})
	if failed, reason := report.failing(options.Strict); failed {
		return out.String(), fmt.Errorf("%s", reason)
	}
	return out.String(), nil
}
