package pwratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/shibukawa/popcornweb/internal/pathpattern"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// Limiter decides. Which bucket a request falls in, whether its path is
// counted at all, and whether this arrival may pass — none of which is a
// question about the transport.
//
// Each transport's frame supplies the two facts it alone can read, the path and
// the caller's address, and writes the refusal its own way. Everything between
// is here, so the two enforce one policy rather than two that agree today.
type Limiter struct {
	config      Config
	counter     Counter
	exemptPaths []pathpattern.Pattern
}

// NewLimiter builds the decision half, or returns nil when the configuration
// installs nothing — which a caller turns into a pass-through frame.
//
// exempt are the paths this limiter never counts. The framework fills them with
// the endpoints it owns and routes itself, and it is not a deployment setting;
// see Exempt for why those cannot be counted.
func NewLimiter(config Config, counter Counter, exempt []string) (*Limiter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, nil
	}
	if counter == nil {
		return nil, errors.New("ratelimit: enabled with no counter store")
	}
	compiled, err := pathpattern.Compile(exempt)
	if err != nil {
		return nil, err
	}
	return &Limiter{config: config, counter: counter, exemptPaths: compiled}, nil
}

// Process is the unkeyed total arrival ceiling, or zero where a deployment set
// none.
func (l *Limiter) Process() int { return l.config.Process }

// ProcessKey is the bucket the total ceiling counts in. It is a constant rather
// than derived from anything, because the ceiling is what an unkeyed count
// means.
const ProcessKey = "process"

// Exempt reports whether this path is outside what the limiter counts.
//
// The carve-out is fixed rather than configurable: a readiness probe arrives
// from the proxy on the same address as every anonymous caller and would
// exhaust that bucket by itself, and one page view fetches many assets. Both
// are endpoints the framework owns and routes, so this is a fact about them
// rather than a policy a deployment tunes.
//
// path is the request's canonical path. A caller that cannot produce one — an
// ambiguous encoding the transport refuses to normalize — passes the empty
// string, which is counted rather than exempted: an unreadable path must not be
// a way out of the limiter.
func (l *Limiter) Exempt(path string) bool {
	if len(l.exemptPaths) == 0 || path == "" {
		return false
	}
	return pathpattern.MatchAny(l.exemptPaths, path)
}

// HasExemptPaths reports whether Exempt can ever answer true, so a transport
// can skip canonicalising a path the limiter would not consult. Most limiter
// configurations carve nothing out, and this runs on every request.
func (l *Limiter) HasExemptPaths() bool { return len(l.exemptPaths) > 0 }

// Identity resolves the bucket key and the count that governs it.
//
// One bucket, two counts: the population differs, since an authenticated caller
// is accountable while an address bucket is both the abuse surface and what a
// corporate NAT shares among many real people.
//
// A limit of zero or less means this caller is not counted, which is what
// PerSubject = 0 asks for.
func (l *Limiter) Identity(ctx context.Context, address string) (string, int) {
	if authentication := pwruntime.RequestAuthentication(ctx); authentication.Authenticated &&
		authentication.Subject != "" {
		return "subject:" + authentication.Subject, l.config.PerSubject
	}
	return "address:" + address, l.config.PerAddress
}

// Admit counts the arrival and reports what to do with it.
//
// A nil refusal admits. A non-nil degraded error means the store could not
// answer and the request was admitted without being counted — deliberately, per
// the note below — which a caller reports rather than acts on.
//
// The request is admitted on a store failure on purpose: the edge in front
// still has its own limits, and refusing here would convert a store incident
// into an outage of every limited route at once, including login. Silently not
// limiting is the state worth knowing about, which is why it is returned rather
// than swallowed.
func (l *Limiter) Admit(ctx context.Context, key string, limit int, now time.Time) (*pwruntime.RateLimit, error) {
	count, err := l.counter.Increment(ctx, key, l.config.Window)
	if err != nil {
		return nil, err
	}
	if count <= uint64(limit) {
		return nil, nil
	}
	reset := WindowReset(now, l.config.Window)
	return &pwruntime.RateLimit{
		Limit:      uint64(limit),
		Remaining:  0,
		Reset:      reset,
		RetryAfter: time.Until(reset),
	}, nil
}
