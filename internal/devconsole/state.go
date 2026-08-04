// Package devconsole serves the pw dev web console: one loopback listener
// holding an index and every pane.
//
// The console is host-side tooling. It reads the project tree and what pw
// itself started, and it never asks the application process for anything, so a
// pane keeps answering while the application is stopped — which is most of the
// time between two working states.
package devconsole

import (
	"sync"
	"time"
)

// Status is where the developer loop currently stands.
type Status string

const (
	// StatusStarting covers every phase before the application is up. It is
	// not a failure, and a page waiting on one should say so rather than
	// showing an error.
	StatusStarting Status = "starting"
	// StatusHealthy means the application process is running. It does not
	// promise that a request to it would succeed.
	StatusHealthy Status = "healthy"
	StatusFailed  Status = "failed"
)

// Diagnostic carries a failure exactly as the terminal received it. The text is
// never reformatted: a rewrapped compiler error is harder to read than the
// original, and the developer is comparing the two.
type Diagnostic struct {
	Text string `json:"text"`
	// File, Line, and Column are set only when the diagnostic named a
	// location. Zero values mean it did not, not that it pointed at line zero.
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// State is the one record the loop publishes on every phase transition. There
// is exactly one current State and no history: what the loop did earlier is the
// terminal scrollback and the telemetry pane, and a second copy of it here
// would be a second thing to keep true.
//
// The record holds no configuration value, no environment variable, and no path
// from outside the project.
type State struct {
	// Build changes whenever the served application changes, so a page can
	// tell whether it is looking at a stale one. It is opaque; nothing reads
	// it for meaning.
	Build string `json:"build"`
	// Phase names the loop phase in progress or last completed, using the
	// words the terminal progress region already prints.
	Phase      string      `json:"phase"`
	Status     Status      `json:"status"`
	Diagnostic *Diagnostic `json:"diagnostic,omitempty"`
	Since      time.Time   `json:"since"`
}

// stateHolder keeps the current State. It is deliberately not a channel or a
// log: readers want what is true now, and every one of them is willing to have
// missed the transitions that got there.
type stateHolder struct {
	mutex   sync.RWMutex
	current State
	// build counts the application starts this run. A restart is what makes a
	// page stale, so counting them is enough and a hash of anything would say
	// no more.
	build int
}

func (h *stateHolder) get() State {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.current
}

// publish records a transition. A failed phase carries its diagnostic; every
// other transition clears whatever the previous failure left behind, so a
// cleared error never lingers on a page.
func (h *stateHolder) publish(phase string, status Status, diagnostic *Diagnostic, now time.Time) State {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if status == StatusHealthy && h.current.Status != StatusHealthy {
		h.build++
	}
	h.current = State{
		Build:      buildIdentity(h.build),
		Phase:      phase,
		Status:     status,
		Diagnostic: diagnostic,
		Since:      now,
	}
	return h.current
}

func buildIdentity(count int) string {
	if count == 0 {
		return ""
	}
	return "b" + itoa(count)
}

// itoa avoids pulling strconv into a file that needs one number formatted.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
