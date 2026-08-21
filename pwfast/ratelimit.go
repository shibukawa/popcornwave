package pwfast

import (
	"time"

	"github.com/shibukawa/popcornweb/internal/pathpattern"
	"github.com/shibukawa/popcornweb/pwratelimit"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The limiter's configuration, its storage and its decisions are the shared
// leaf's, so a project configures one limiter and both builds enforce it. What
// is here is this transport's two frames.
type (
	// RateLimitConfig bounds how often one caller, and the process as a whole,
	// may arrive within a window.
	RateLimitConfig = pwratelimit.Config
	// RateLimitCounter is the storage a limiter counts in.
	RateLimitCounter = pwratelimit.Counter
)

// RateLimitRejection writes the response for a refused request.
type RateLimitRejection func(r *fasthttp.RequestCtx, limit pwruntime.RateLimit)

// RateLimitDegraded is called when the counter store could not answer.
//
// The request is admitted regardless, per pwratelimit.Limiter.Admit. This
// exists so that silently-not-limiting is observable, which is the worst of the
// three states to be in unknowingly.
type RateLimitDegraded func(r *fasthttp.RequestCtx, err error)

// RateLimitDeps are the runtime pieces the limiter needs beyond its
// configuration.
type RateLimitDeps struct {
	// Counter is the storage. It is required once the limiter is enabled.
	Counter RateLimitCounter
	// Exempt are the paths this limiter never counts. The framework fills it
	// with the endpoints it owns and routes itself — the operational probes and
	// the public asset mount — and it is not a deployment setting.
	Exempt []string
	// Reject writes the refusal. A nil value writes the framework problem
	// document, which is what a caller on either transport receives.
	Reject RateLimitRejection
	// Degraded observes an admission made without a working store.
	Degraded RateLimitDegraded
}

// RateLimiter bounds arrivals per caller.
//
// It is named for the frame rather than for the limit, because RateLimit here
// is already the response metadata a refusal carries — the same spelling the
// other transport gives it.
//
// The identity is the authenticated subject where there is one and the resolved
// client address otherwise, so it must be installed below whatever establishes
// authentication — which the shared slot ordering does. The process ceiling
// needs nothing resolved and is installed separately, further out.
func RateLimiter(config RateLimitConfig, deps RateLimitDeps) (Middleware, error) {
	limiter, err := pwratelimit.NewLimiter(config, deps.Counter, deps.Exempt)
	if err != nil || limiter == nil {
		return ratePassThrough(err)
	}
	frame := rateLimitFrame{limiter: limiter, deps: deps}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			if frame.exempt(r) {
				next(r)
				return
			}
			key, limit := limiter.Identity(r, pwruntime.ReadClientAddress(r))
			if limit <= 0 {
				next(r)
				return
			}
			if !frame.admit(r, key, limit) {
				return
			}
			next(r)
		}
	}, nil
}

// ProcessRateLimiter is the unkeyed total arrival ceiling.
//
// It is a separate frame from RateLimiter because it needs nothing resolved and
// belongs further out, where a refused request has cost less. It is also the
// only layer that sees a flood spread across many addresses, each of them
// staying under the per-address count by construction.
func ProcessRateLimiter(config RateLimitConfig, deps RateLimitDeps) (Middleware, error) {
	if config.Process <= 0 {
		// Validate still runs, so a configuration mistake is reported whether or
		// not this particular layer is switched on.
		if err := config.Validate(); err != nil {
			return nil, err
		}
		return ratePassThrough(nil)
	}
	limiter, err := pwratelimit.NewLimiter(config, deps.Counter, deps.Exempt)
	if err != nil || limiter == nil {
		return ratePassThrough(err)
	}
	frame := rateLimitFrame{limiter: limiter, deps: deps}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			// The exempt paths are deliberately not consulted here. They keep a
			// page's asset fetches out of the identity buckets, but the ceiling
			// is the one layer that sees a distributed flood, and a fixed,
			// discoverable prefix that nothing counts would be that flood's way
			// past it.
			if !frame.admit(r, pwratelimit.ProcessKey, limiter.Process()) {
				return
			}
			next(r)
		}
	}, nil
}

// rateLimitFrame is the transport half: the two facts read off this request
// value, and the refusal written back to it.
type rateLimitFrame struct {
	limiter *pwratelimit.Limiter
	deps    RateLimitDeps
}

// exempt reads the undecoded path, for the same reason the guard does: this
// transport's RequestURI carries the query string, so a return path encoded in
// a parameter would be read as an ambiguous path and the request treated as
// unexemptable.
func (f rateLimitFrame) exempt(r *fasthttp.RequestCtx) bool {
	if !f.limiter.HasExemptPaths() {
		return false
	}
	path, ok := pathpattern.CanonicalPathOf(string(r.Path()), rawPath(r))
	if !ok {
		// An ambiguous path is counted rather than exempted. The limiter must
		// not have a way out that a caller can spell.
		return false
	}
	return f.limiter.Exempt(path)
}

func (f rateLimitFrame) admit(r *fasthttp.RequestCtx, key string, limit int) bool {
	refusal, err := f.limiter.Admit(r, key, limit, time.Now())
	if err != nil {
		if f.deps.Degraded != nil {
			f.deps.Degraded(r, err)
		}
		return true
	}
	if refusal == nil {
		return true
	}
	reject := f.deps.Reject
	if reject == nil {
		reject = writeRateLimitProblem
	}
	reject(r, *refusal)
	return false
}

func ratePassThrough(err error) (Middleware, error) {
	if err != nil {
		return nil, err
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }, nil
}

// writeRateLimitProblem answers a refused request through the framework error
// path, so a caller gets the same document the other transport writes.
//
// The metadata rides on the problem rather than being applied separately: this
// transport's WriteProblem sets the headers a Problem carries, where the other
// half has to apply them first because its own writer may already have
// committed by then.
//
// The response names no rule and no counter. What a caller needs is when to
// come back, which the metadata already says.
func writeRateLimitProblem(r *fasthttp.RequestCtx, limit pwruntime.RateLimit) {
	WriteProblem(r, RateLimited(limit))
}
