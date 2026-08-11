package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// presenceReport is the whole of what a browser sends: whether any input
// happened since the last tick.
//
// One bit, and nothing else. No coordinate, no key, no timing, and no interval
// leaves the browser, which makes behavioral analysis impossible rather than
// merely forbidden. What is transmitted is the negation of absence, which is
// the direction the trust asymmetry below already treats as bounded, so the
// untrusted channel carries the least it can.
type presenceReport struct {
	// Active reports that at least one input event fired since the previous
	// tick. False is an absence report, and so is a tick that never arrives.
	Active bool `json:"active"`
	// Gap reports seconds of wall-clock time the browser observed passing
	// between ticks that should have been Interval apart, which is how a
	// machine waking from sleep is inferred. Nothing reports a wake directly.
	Gap int64 `json:"gap,omitempty"`
}

// maxPresenceBodyBytes bounds the request. The document is two small fields, so
// anything larger is not one.
const maxPresenceBodyBytes = 256

// presencePath is where a browser reports. It hangs off the logout path
// because ending a session for absence is what it ultimately does.
func (rt *runtime) presencePath() string { return rt.config.LogoutPath + "/presence" }

// handlePresence records a presence report against the request's session.
//
// The trust here is deliberately asymmetric, because the failure costs are:
//
//   - A claim of absence is acted on immediately. A false positive costs one
//     extra sign-in, and a browser cannot assert not-being-there at all, so a
//     tick that stops arriving is itself an absence report.
//   - A claim of presence is bounded. A script can send it, so it refreshes
//     idle expiry within the bounds the server already owns and can never move
//     the absolute expiry.
//
// The server stays authoritative for every lifetime. This is an input to a
// bound it already enforces, never a lifetime of its own.
func (rt *runtime) handlePresence(x Exchange) {
	if !allowMethod(x, http.MethodPost) {
		return
	}
	if !rt.sameOrigin(x) {
		x.Problem(pwruntime.Forbidden())
		return
	}
	view, ok := Session(x.Context())
	if !ok {
		// Nothing to keep alive and nothing to end. An anonymous browser
		// reporting presence is answered without saying whether a session
		// existed.
		x.Write(http.StatusNoContent, nil)
		return
	}
	body, err := x.Body(maxPresenceBodyBytes)
	if err != nil {
		x.Problem(pwruntime.BadRequest())
		return
	}
	var report presenceReport
	if err := json.Unmarshal(body, &report); err != nil {
		x.Problem(pwruntime.BadRequest())
		return
	}
	config := rt.config.Assurance.Presence
	absent := !report.Active
	if config.AbsentAfter > 0 && report.Gap > 0 &&
		time.Duration(report.Gap)*time.Second >= config.AbsentAfter {
		// A wall-clock gap far larger than the tick interval means the machine
		// slept or the tab was frozen. Either way nobody was there for it.
		absent = true
	}
	if absent {
		rt.endForAbsence(x)
		x.Write(http.StatusNoContent, nil)
		return
	}
	_ = view
	// A present browser needs nothing written: the request itself already
	// refreshed idle expiry through the session middleware, within the absolute
	// bound that middleware also enforces. Reporting presence therefore cannot
	// extend a session further than an ordinary request would.
	x.Write(http.StatusNoContent, nil)
}

// endForAbsence ends the session of a browser that reported nobody at it. The
// hint follows the deployment's own rule: expiry may leave one, and this is an
// expiry rather than a sign-out.
func (rt *runtime) endForAbsence(x Exchange) {
	if err := rt.endSession(x); err != nil {
		logger(x).Log(x.Context(), pwruntime.LevelWarn, "ending an absent session failed", pwruntime.Err(err))
	}
}
