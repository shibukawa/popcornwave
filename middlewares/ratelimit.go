package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// Rate limit backend names. A deployment names one of these in
// ratelimit.backend.
const (
	// RateLimitBackendMemory counts inside this process. It is correct on one
	// replica and enforces N times the configured limit on N of them.
	RateLimitBackendMemory = "memory"
	// RateLimitBackendRedis counts in a shared server, which is what a
	// deployment running more than one replica needs.
	RateLimitBackendRedis = "redis"
)

// RateLimitConfig bounds how often one caller, and the process as a whole, may
// arrive within a window.
//
// It deliberately declares no per-route rules. A per-operation quota belongs to
// an API gateway, and a pattern grammar with a precedence order would be most
// of the cost of this middleware for a capability the boundary in front of a
// normal deployment already sells.
type RateLimitConfig struct {
	Enabled bool   `default:"false"`
	Backend string `default:"memory" dependon:".enabled" help:"counter storage: memory or redis"`
	// Window is the period every count below is measured over. It is also the
	// burst granularity, because the algorithm is a fixed window, and it is
	// what X-RateLimit-Reset reports.
	Window time.Duration `default:"1m" dependon:".enabled" help:"period every count is measured over"`
	// PerSubject bounds one authenticated caller. Zero disables this bucket,
	// because such a caller is accountable and revocable by other means.
	PerSubject int `default:"600" dependon:".enabled" help:"requests one authenticated subject may make in a window; zero disables"`
	// PerAddress bounds one caller with no session, and has no off position.
	// It is the only bucket an unauthenticated flood meets, so an unlimited
	// value here is an absent control rather than a permissive one.
	PerAddress int `default:"300" dependon:".enabled" help:"requests one caller with no session may make in a window"`
	// Process is the total arrival ceiling, unkeyed. It is the only layer that
	// sees a distributed flood, since such a flood keeps every source under
	// PerAddress by construction.
	//
	// It defaults to zero rather than to a guess: the right value follows from
	// what a deployment can serve, and one set below real capacity refuses
	// legitimate traffic globally.
	Process int                  `default:"0" dependon:".enabled" help:"total arrivals allowed in a window, unkeyed; zero leaves only the identity buckets"`
	Redis   RateLimitRedisConfig `dependon:".enabled"`
}

// RateLimitRedisConfig addresses the shared counter server.
type RateLimitRedisConfig struct {
	DSN            string        `secret:"mask" env:"RATELIMIT_REDIS_DSN" help:"redis:// or rediss:// counter server"`
	KeyPrefix      string        `default:"pw:ratelimit:" help:"key space this limiter owns"`
	ConnectTimeout time.Duration `default:"5s" help:"bounds the startup ping and per-command deadlines"`
}

// DefaultRateLimit returns the shipped defaults: off, and bounded once on.
func DefaultRateLimit() RateLimitConfig {
	return RateLimitConfig{
		Backend:    RateLimitBackendMemory,
		Window:     time.Minute,
		PerSubject: 600,
		PerAddress: 300,
		Redis: RateLimitRedisConfig{
			KeyPrefix:      DefaultRateLimitKeyPrefix,
			ConnectTimeout: 5 * time.Second,
		},
	}
}

// DefaultRateLimitKeyPrefix namespaces the keys this limiter owns.
const DefaultRateLimitKeyPrefix = "pw:ratelimit:"

// Validate rejects a configuration whose limits cannot bind.
func (c RateLimitConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	switch c.Backend {
	case "", RateLimitBackendMemory, RateLimitBackendRedis:
	default:
		return fmt.Errorf("ratelimit.backend %q is not memory or redis", c.Backend)
	}
	if c.Window <= 0 {
		return errors.New("ratelimit.window must be positive")
	}
	if c.PerSubject < 0 {
		return errors.New("ratelimit.per_subject must not be negative")
	}
	if c.Process < 0 {
		return errors.New("ratelimit.process must not be negative")
	}
	// The one count with no off position: it is what an unauthenticated flood
	// meets, and nothing else does.
	if c.PerAddress <= 0 {
		return errors.New("ratelimit.per_address must be positive; it is the only bucket an unauthenticated caller meets")
	}
	if c.Process > 0 {
		// A caller allowed more than the total describes a limit that can
		// never bind, which is a configuration mistake rather than a policy.
		if c.PerAddress > c.Process {
			return fmt.Errorf("ratelimit.per_address %d exceeds ratelimit.process %d", c.PerAddress, c.Process)
		}
		if c.PerSubject > c.Process {
			return fmt.Errorf("ratelimit.per_subject %d exceeds ratelimit.process %d", c.PerSubject, c.Process)
		}
	}
	if c.Backend != RateLimitBackendRedis && strings.TrimSpace(c.Redis.DSN) != "" {
		return errors.New(`ratelimit.redis.dsn is set while ratelimit.backend is not "redis"`)
	}
	if c.Backend == RateLimitBackendRedis && strings.TrimSpace(c.Redis.DSN) == "" {
		return errors.New(`ratelimit.backend = "redis" requires ratelimit.redis.dsn`)
	}
	return nil
}

// RateLimitCounter is the whole storage interface: add one to the count under
// key for the window ending at expiry, and report the new total.
//
// A backend sets the expiry when it creates the key and never extends it,
// which is what makes the window fixed rather than sliding.
type RateLimitCounter interface {
	Increment(ctx context.Context, key string, window time.Duration) (uint64, error)
}

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
	// and the public asset mount — and it is not a deployment setting; see
	// rateLimiter.exempt for why those two cannot be counted.
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
	limiter, err := newRateLimiter(config, deps)
	if err != nil || limiter == nil {
		return passThrough(err)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter.exempt(r) {
				next.ServeHTTP(w, r)
				return
			}
			key, limit := limiter.identity(r)
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.admit(w, r, key, limit) {
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
	limiter, err := newRateLimiter(config, deps)
	if err != nil || limiter == nil {
		return passThrough(err)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter.exempt(r) {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.admit(w, r, "process", config.Process) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// newRateLimiter returns nil with no error when the configuration installs
// nothing, which both constructors turn into a pass-through frame.
func newRateLimiter(config RateLimitConfig, deps RateLimitDeps) (*rateLimiter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, nil
	}
	if deps.Counter == nil {
		return nil, errors.New("ratelimit: enabled with no counter store")
	}
	exempt, err := pathpattern.Compile(deps.Exempt)
	if err != nil {
		return nil, err
	}
	reject := deps.Reject
	if reject == nil {
		reject = writeRateLimitStatus
	}
	return &rateLimiter{
		config:      config,
		counter:     deps.Counter,
		reject:      reject,
		degraded:    deps.Degraded,
		exemptPaths: exempt,
	}, nil
}

func passThrough(err error) (Middleware, error) {
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler { return next }, nil
}

type rateLimiter struct {
	config      RateLimitConfig
	counter     RateLimitCounter
	reject      RateLimitRejection
	degraded    RateLimitDegraded
	exemptPaths []pathpattern.Pattern
}

// identity resolves the bucket key and the count that governs it.
//
// One bucket, two counts: the population differs, since an authenticated
// caller is accountable while an address bucket is both the abuse surface and
// what a corporate NAT shares among many real people.
func (l *rateLimiter) identity(r *http.Request) (string, int) {
	if authentication := pwruntime.RequestAuthentication(r.Context()); authentication.Authenticated &&
		authentication.Subject != "" {
		return "subject:" + authentication.Subject, l.config.PerSubject
	}
	return "address:" + pwruntime.ClientAddress(r.Context(), r), l.config.PerAddress
}

// admit counts the arrival and answers whether the request may continue. A
// store that cannot answer admits, per RateLimitDegraded.
func (l *rateLimiter) admit(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	window := l.config.Window
	count, err := l.counter.Increment(r.Context(), key, window)
	if err != nil {
		if l.degraded != nil {
			l.degraded(r, err)
		}
		return true
	}
	if count <= uint64(limit) {
		return true
	}
	reset := windowReset(time.Now(), window)
	l.reject(w, r, pwruntime.RateLimit{
		Limit:      uint64(limit),
		Remaining:  0,
		Reset:      reset,
		RetryAfter: time.Until(reset),
	})
	return false
}

// exempt reports whether this path is outside what the limiter counts.
//
// The carve-out is fixed rather than configurable: a readiness probe arrives
// from the proxy on the same address as every anonymous caller and would
// exhaust that bucket by itself, and one page view fetches many assets. Both
// are endpoints the framework owns and routes, so this is a fact about them
// rather than a policy a deployment tunes.
func (l *rateLimiter) exempt(r *http.Request) bool {
	if len(l.exemptPaths) == 0 {
		return false
	}
	path, ok := pathpattern.CanonicalPath(r)
	if !ok {
		return false
	}
	return pathpattern.MatchAny(l.exemptPaths, path)
}

// windowReset is the end of the fixed window the current instant falls in,
// which is what the counter's own expiry is set to.
func windowReset(now time.Time, window time.Duration) time.Time {
	if window <= 0 {
		return now
	}
	return now.Truncate(window).Add(window)
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
