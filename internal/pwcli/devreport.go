package pwcli

import (
	"strings"

	"github.com/shibukawa/popcornwave/internal/devconsole"
)

// devReporter says what phase the developer loop is in, to the terminal region
// and to the console, from one call.
//
// They are paired rather than called separately because the two must not
// disagree: a terminal showing a failure over a console reporting a healthy
// loop is worse than either alone, and every phase in the loop has both an
// operator watching the scrollback and possibly a browser open.
//
// A nil console is ordinary. It means the console was disabled or could not
// listen, and every method here already tolerates it.
type devReporter struct {
	progress *progressRegion
	console  *devconsole.Console
	// phase is the last phase named, so a failure does not have to repeat it
	// at the call site where the error is handled.
	phase string
}

// Phase names the work now in progress. It is not a success report: the phase
// is published as starting, and only Healthy says the loop reached its steady
// state.
func (r *devReporter) Phase(name string) {
	r.phase = name
	r.progress.Phase(name)
	r.console.Publish(name, devconsole.StatusStarting, nil)
}

// Failed attaches a diagnostic to the current phase. The text is passed through
// unchanged: a rewrapped compiler error is harder to read than the original,
// and the developer is comparing it with the one in the terminal.
func (r *devReporter) Failed(err error) {
	if err == nil {
		return
	}
	r.console.Failed(r.phase, strings.TrimSpace(err.Error()))
}

// Healthy reports that the application process is running. It does not promise
// that a request to it would succeed, only that there is something to send one
// to.
func (r *devReporter) Healthy() {
	r.console.Publish(r.phase, devconsole.StatusHealthy, nil)
}

// Done gives the terminal region back to the application and service streams.
// The console keeps whatever was last published, because a page opened later
// still wants to know how the loop got where it is.
func (r *devReporter) Done() {
	r.progress.Done()
}
