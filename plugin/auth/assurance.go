package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	pw "github.com/shibukawa/popcornwave/pw"
)

// A Requirement states how recently the identity of a request must have been
// proved. It is a type rather than a time.Duration because the sources of a
// window are plural and Go has no overloading: a literal in the code, a name a
// deployment tunes, or a wall-clock deadline computed per request.
//
// There is deliberately no minimum authentication strength. The framework
// cannot rank the methods it mounts: in a mode with one method an ordering has
// nothing to order, and in a mode with two neither is stronger in general, so
// any default would be a claim the framework is not entitled to make.
type Requirement interface {
	// resolve returns the window for one request. The bool reports whether the
	// requirement could be resolved at all.
	resolve(*http.Request, Config) (window, bool)
}

// window is a resolved requirement: how old a proof may be, and whether a
// login counts as one.
type window struct {
	maxAge time.Duration
	// confirmed demands a re-proof the guard asked for, so an ordinary login
	// never satisfies it however recent it is.
	//
	// Signing in and confirming an operation are different acts. A login
	// happened for its own reasons and says nothing about the person now
	// asking to move money; a step-up happened because this operation
	// demanded it. Freshness alone conflates them, and a window wide enough
	// to be usable lets a sign-in stand in for the confirmation.
	confirmed bool
}

// MaxAge admits any proof no older than d, including the login that started
// the session. Use it where recency is the point: a session that has been
// sitting open all afternoon should not reach an administration area, but the
// person who just signed in already proved who they are.
func MaxAge(d time.Duration) Requirement {
	return literalRequirement{window: window{maxAge: d}}
}

// Confirmed admits only a re-proof this guard asked for, within d. A login,
// however recent, never satisfies it.
//
// Use it where the act matters rather than the clock: moving money, deleting a
// tenant, exporting a customer list. Somebody who signed in a minute ago to
// read their dashboard has not agreed to any of those.
//
// A zero duration means confirm for this attempt, and two consecutive
// operations then require two confirmations.
func Confirmed(d time.Duration) Requirement {
	return literalRequirement{window: window{maxAge: d, confirmed: true}}
}

// Policy reads the window from auth.assurance.policy, so the same handler code
// serves a consumer deployment with a long window and an internal one with a
// short window. An undefined name is refused at startup by Config.validate
// rather than at the request that needed it.
func Policy(name string) Requirement { return namedRequirement(name) }

// Dynamic computes the window per request, for a deadline that no fixed
// duration expresses. An internal system re-confirming after every midnight
// returns the time elapsed since the most recent midnight, so a proof older
// than that boundary no longer counts.
func Dynamic(compute func(*http.Request) time.Duration) Requirement {
	return dynamicRequirement{compute: compute}
}

// Default is the window of auth.recent_auth_max_age, which is also what the
// passkey enrollment guard has always used.
func Default() Requirement { return defaultRequirement{} }

type literalRequirement struct{ window window }

func (r literalRequirement) resolve(*http.Request, Config) (window, bool) {
	return r.window, true
}

type namedRequirement string

func (r namedRequirement) resolve(_ *http.Request, config Config) (window, bool) {
	for _, policy := range config.Assurance.Policy {
		if policy.Name == string(r) {
			return window{maxAge: policy.MaxAge, confirmed: policy.Confirm}, true
		}
	}
	return window{}, false
}

type dynamicRequirement struct {
	compute func(*http.Request) time.Duration
}

func (r dynamicRequirement) resolve(request *http.Request, _ Config) (window, bool) {
	if r.compute == nil {
		return window{}, false
	}
	value := r.compute(request)
	if value < 0 {
		return window{}, false
	}
	return window{maxAge: value}, true
}

type defaultRequirement struct{}

func (defaultRequirement) resolve(_ *http.Request, config Config) (window, bool) {
	return window{maxAge: config.RecentAuthMaxAge}, true
}

// ErrNoAssurance reports that assurance could not be evaluated because the auth
// plugin is not running. It is returned rather than assumed, because assuming
// either answer would be wrong: assuming satisfied opens the guard, and
// assuming unsatisfied sends an anonymous deployment into a login it has no
// endpoint for.
var ErrNoAssurance = errors.New("auth: assurance is unavailable")

// Ensure wraps a page route: a request whose proof is older than the
// requirement is redirected into re-proof and returns to this operation.
//
// Guarding a read route is user experience rather than a boundary, because a
// client can post directly to the write route. Guard both, and give the write
// the more generous window so a user who reads, fills a long form, and submits
// does not lose the input to a guard the read had just satisfied.
func Ensure(handler http.HandlerFunc, requirement Requirement) http.HandlerFunc {
	return guard(handler, requirement, false)
}

// EnsureAPI wraps an API route: an unmet requirement answers 401 with a problem
// document naming the window, so a client can start the step-up itself.
//
// This is a separate function rather than an option with a default because the
// two failure modes are not symmetric. Answering an API call with a redirect is
// followed transparently by an XHR, so the client receives 200 and an HTML
// login page instead of an error; answering a page with 401 is merely ugly. A
// route that forgot to pass an option would take the silent failure.
func EnsureAPI(handler http.HandlerFunc, requirement Requirement) http.HandlerFunc {
	return guard(handler, requirement, true)
}

func guard(handler http.HandlerFunc, requirement Requirement, api bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, err := satisfied(r, requirement)
		if err != nil {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "assurance check failed", pw.Err(err))
			pw.WriteProblem(w, r, pw.ServiceUnavailable())
			return
		}
		if ok {
			handler(w, r)
			return
		}
		challenge(w, r, requirement, api)
	}
}

// IsRecent reports whether the requirement is met and writes nothing. Use it
// with Challenge when the window depends on something only the handler knows,
// such as a payment amount. It is split from Challenge because a function that
// both returns a decision and writes a response hides control flow.
func IsRecent(r *http.Request, requirement Requirement) bool {
	ok, err := satisfied(r, requirement)
	return err == nil && ok
}

// Challenge writes the response an unmet requirement produces and starts the
// step-up. Pass api = true from a route whose caller is not a browser.
func Challenge(w http.ResponseWriter, r *http.Request, requirement Requirement, api bool) {
	challenge(w, r, requirement, api)
}

// LastProvedAt reports when the identity of the request was last actually
// proved, so a page can warn before the user commits to a long form. The bool
// is false for a request carrying no session.
func LastProvedAt(ctx context.Context) (time.Time, bool) {
	view, ok := Session(ctx)
	if !ok {
		return time.Time{}, false
	}
	return view.Data.provenAt(view.AuthenticatedAt), true
}

// satisfied evaluates the requirement against the request's session.
func satisfied(r *http.Request, requirement Requirement) (bool, error) {
	instance := activeRuntime()
	if instance == nil {
		return false, ErrNoAssurance
	}
	view, ok := Session(r.Context())
	if !ok {
		// An anonymous request is a login problem rather than an assurance
		// problem, and the path guard answers it first. Reaching here means the
		// route is unguarded, so the challenge sends the user to log in.
		return false, nil
	}
	want, resolved := requirement.resolve(r, instance.config)
	if !resolved {
		return false, fmt.Errorf("%w: requirement could not be resolved", ErrNoAssurance)
	}
	if want.confirmed {
		return confirmedWithin(view.Data, want.maxAge), nil
	}
	if want.maxAge == 0 {
		// Elapsed time can never satisfy a zero window: the redirect to the
		// provider and back always consumes more than zero seconds, so a
		// timestamp comparison would challenge again immediately after a
		// successful re-proof and never converge. Read it as a confirmation
		// for this attempt, which is what it was always meant to say.
		return stepUpAdmitted(view.Data), nil
	}
	return time.Since(view.Data.provenAt(view.AuthenticatedAt)) <= want.maxAge, nil
}

// confirmedWithin reports whether a re-proof the guard asked for happened
// inside the window. The login that started the session does not count, however
// recent it is: it proved an identity, not an intention.
func confirmedWithin(data SessionData, maxAge time.Duration) bool {
	if maxAge == 0 {
		return stepUpAdmitted(data)
	}
	if data.StepUpAt <= 0 {
		return false
	}
	return time.Since(time.Unix(data.StepUpAt, 0)) <= maxAge
}

// stepUpAdmissionWindow bounds how long a completed zero-window proof admits an
// operation. It covers the redirect back and the click that follows, and
// nothing longer.
const stepUpAdmissionWindow = 30 * time.Second

func stepUpAdmitted(data SessionData) bool {
	if data.StepUpAt <= 0 {
		return false
	}
	return time.Since(time.Unix(data.StepUpAt, 0)) <= stepUpAdmissionWindow
}

// challenge answers an unmet requirement. A page route is sent into re-proof
// and comes back to this operation; an API route is told what was missing and
// starts the step-up itself.
//
// The status is 401 rather than 403 because the remedy exists and the client is
// meant to prove again and retry, which is also what RFC 9470 returns for
// insufficient_user_authentication. The Bearer challenge header of that
// specification is not emitted here: these routes authenticate with a cookie
// and are not in the Bearer scheme.
func challenge(w http.ResponseWriter, r *http.Request, requirement Requirement, api bool) {
	instance := activeRuntime()
	if instance == nil {
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	want, resolved := requirement.resolve(r, instance.config)
	if !resolved {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "assurance requirement could not be resolved")
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	if api {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	instance.redirect(w, r, instance.stepUpPath(r, want.maxAge))
}

