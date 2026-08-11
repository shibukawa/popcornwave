package middlewares

import (
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/pwratelimit"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// The rate limiter's vocabulary is the shared leaf's, so a project configures
// one limiter and both builds enforce it. What is here is this transport's two
// frames: reading the path off a net/http request, and writing the refusal.
type (
	// RateLimitConfig bounds how often one caller, and the process as a whole,
	// may arrive within a window.
	RateLimitConfig = pwruntime.RateLimitConfig
	// RateLimitRedisConfig addresses the shared counter server.
	RateLimitRedisConfig = pwruntime.RateLimitRedisConfig
	// RateLimitCounter is the storage a limiter counts in.
	RateLimitCounter = pwratelimit.Counter
	// MemoryRateLimitCounter counts inside this process.
	MemoryRateLimitCounter = pwratelimit.MemoryCounter
)

const (
	// RateLimitBackendMemory counts inside this process.
	RateLimitBackendMemory = pwruntime.RateLimitBackendMemory
	// RateLimitBackendRedis counts in a shared server.
	RateLimitBackendRedis = pwruntime.RateLimitBackendRedis
	// DefaultRateLimitKeyPrefix namespaces the keys this limiter owns.
	DefaultRateLimitKeyPrefix = pwruntime.DefaultRateLimitKeyPrefix
)

// DefaultRateLimit returns the shipped defaults: off, and bounded once on.
func DefaultRateLimit() RateLimitConfig { return pwruntime.DefaultRateLimit() }

// NewMemoryRateLimitCounter returns an empty in-process counter.
func NewMemoryRateLimitCounter() *MemoryRateLimitCounter { return pwratelimit.NewMemoryCounter() }

// RateLimitRejection writes the response for a refused request.
type RateLimitRejection func(w http.ResponseWriter, r *http.Request, limit pwruntime.RateLimit)

// RateLimitDegraded is called when the counter store could not answer.
//
// The request is admitted regardless. A limiter that refuses when its store
// blinks converts a store incident into an outage of every limited route at
// once, including login, while the edge in front still has its own limits.
// This exists so that silently-not-limiting is observable, which is the worst
// of the three states to be in unknowingly.
type RateLimitDegraded func(r *http.Request, err error)

// RateLimitDeps are the runtime pieces the limiter needs beyond its
// configuration.
type RateLimitDeps struct {
	// Counter is the storage. It is required once the limiter is enabled.
	Counter RateLimitCounter
	// Exempt are the paths this limiter never counts. The framework fills it
	// with the endpoints it owns and routes itself — the operational probes
	// and the public asset mount — and it is not a deployment setting.
	Exempt []string
	// Reject writes the refusal. A nil value writes a bare 429.
	Reject RateLimitRejection
	// Degraded observes an admission made without a working store.
	Degraded RateLimitDegraded
}

// RateLimit bounds arrivals per caller.
//
// The identity is the authenticated subject where there is one and the
// resolved client address otherwise, so it must be installed below whatever
// establishes authentication. The process ceiling needs nothing resolved and
// is installed separately, further out.
func RateLimit(config RateLimitConfig, deps RateLimitDeps) (Middleware, error) {
	limiter, err := pwratelimit.NewLimiter(config, deps.Counter, deps.Exempt)
	if err != nil || limiter == nil {
		return passThrough(err)
	}
	frame := rateLimitFrame{limiter: limiter, deps: deps}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if frame.exempt(r) {
				next.ServeHTTP(w, r)
				return
			}
			key, limit := limiter.Identity(r.Context(), pwruntime.ClientAddress(r.Context(), r))
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if !frame.admit(w, r, key, limit) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// RateLimitProcess is the unkeyed total arrival ceiling.
//
// It is a separate middleware from RateLimit because it needs nothing resolved
// and belongs further out, where a refused request has cost less. It is also
// the only layer that sees a flood spread across many addresses, each of them
// staying under the per-address count by construction.
func RateLimitProcess(config RateLimitConfig, deps RateLimitDeps) (Middleware, error) {
	if config.Process <= 0 {
		// Validate still runs, so a configuration mistake is reported whether
		// or not this particular layer is switched on.
		if err := config.Validate(); err != nil {
			return nil, err
		}
		return passThrough(nil)
	}
	limiter, err := pwratelimit.NewLimiter(config, deps.Counter, deps.Exempt)
	if err != nil || limiter == nil {
		return passThrough(err)
	}
	frame := rateLimitFrame{limiter: limiter, deps: deps}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if frame.exempt(r) {
				next.ServeHTTP(w, r)
				return
			}
			if !frame.admit(w, r, pwratelimit.ProcessKey, limiter.Process()) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// rateLimitFrame is the transport half: the two facts read off a net/http
// request, and the refusal written back to it.
type rateLimitFrame struct {
	limiter *pwratelimit.Limiter
	deps    RateLimitDeps
}

func (f rateLimitFrame) exempt(r *http.Request) bool {
	path, ok := pathpattern.CanonicalPath(r)
	if !ok {
		return false
	}
	return f.limiter.Exempt(path)
}

func (f rateLimitFrame) admit(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	refusal, err := f.limiter.Admit(r.Context(), key, limit, time.Now())
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
		reject = writeRateLimitStatus
	}
	reject(w, r, *refusal)
	return false
}

func passThrough(err error) (Middleware, error) {
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler { return next }, nil
}

func writeRateLimitStatus(w http.ResponseWriter, r *http.Request, limit pwruntime.RateLimit) {
	problem := pwruntime.Problem{Status: http.StatusTooManyRequests, RateLimit: &limit}
	if err := pwruntime.ApplyProblemHeaders(w.Header(), problem); err != nil {
		pwruntime.ReadLogger(r.Context()).Log(r.Context(), pwruntime.LevelWarn,
			"rate limit metadata was omitted", pwruntime.String("error", err.Error()))
	}
	if Committed(w) {
		return
	}
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}
