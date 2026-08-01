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
	// maxAge resolves the window for one request. The bool reports whether the
	// requirement could be resolved at all.
	maxAge(*http.Request, Config) (time.Duration, bool)
}

// MaxAge states the window literally, for a route whose window is a property
// of the code rather than of the deployment.
//
// A zero duration is meaningful: it is the per-operation level, satisfied only
// by a proof completed for this attempt. See Ensure for why that cannot be a
// timestamp comparison.
func MaxAge(d time.Duration) Requirement { return literalRequirement(d) }

// Policy reads the window from auth.assurance.policy, so the same handler code
// serves a consumer deployment with a long window and an internal one with a
// short window. An undefined name is refused at startup by Config.validate
// rather than at the request that needed it.
func Policy(name string) Requirement { return namedRequirement(name) }

// Dynamic computes the window per request, for a deadline that no fixed
// duration expresses. An internal system re-confirming after every midnight
// returns the time elapsed since the most recent midnight, so a proof older
// than that boundary no longer counts.
func Dynamic(resolve func(*http.Request) time.Duration) Requirement {
	return dynamicRequirement{resolve: resolve}
}

// Default is the window of auth.recent_auth_max_age, which is also what the
// passkey enrollment guard has always used.
func Default() Requirement { return defaultRequirement{} }

type literalRequirement time.Duration

func (r literalRequirement) maxAge(*http.Request, Config) (time.Duration, bool) {
	return time.Duration(r), true
}

type namedRequirement string

func (r namedRequirement) maxAge(_ *http.Request, config Config) (time.Duration, bool) {
	for _, policy := range config.Assurance.Policy {
		if policy.Name == string(r) {
			return policy.MaxAge, true
		}
	}
	return 0, false
}

type dynamicRequirement struct {
	resolve func(*http.Request) time.Duration
}

func (r dynamicRequirement) maxAge(request *http.Request, _ Config) (time.Duration, bool) {
	if r.resolve == nil {
		return 0, false
	}
	value := r.resolve(request)
	if value < 0 {
		return 0, false
	}
	return value, true
}

type defaultRequirement struct{}

func (defaultRequirement) maxAge(_ *http.Request, config Config) (time.Duration, bool) {
	return config.RecentAuthMaxAge, true
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
	window, resolved := requirement.maxAge(r, instance.config)
	if !resolved {
		return false, fmt.Errorf("%w: requirement could not be resolved", ErrNoAssurance)
	}
	if window == 0 {
		// A zero window is never satisfied by elapsed time: the redirect to the
		// provider and back always consumes more than zero seconds, so a
		// timestamp comparison would challenge again immediately after a
		// successful re-proof and never converge. It is satisfied only by the
		// single-use admission a completed step-up leaves behind.
		return stepUpAdmitted(view.Data), nil
	}
	return time.Since(view.Data.provenAt(view.AuthenticatedAt)) <= window, nil
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
	window, resolved := requirement.maxAge(r, instance.config)
	if !resolved {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "assurance requirement could not be resolved")
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	if api {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	instance.redirect(w, r, instance.stepUpPath(r, window))
}

