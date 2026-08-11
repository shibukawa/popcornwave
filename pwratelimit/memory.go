package pwratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryCounter counts inside this process.
//
// It is the default because a limiter that needs a dependency before it starts
// is one nobody switches on. It is also only correct on one replica: N of them
// each enforce the configured limit, so the effective limit is N times what the
// deployment declared. That is a property of the choice rather than a defect,
// and pw doctor reports it.
type MemoryCounter struct {
	mu sync.Mutex
	// windows holds one counter per key for the window it names. An entry
	// whose window has passed is replaced on the next arrival rather than
	// swept, so an idle key costs one map entry until something touches it.
	windows map[string]*memoryWindow
	// sweptAt bounds how often the whole map is walked for expired keys, so a
	// deployment cycling through many addresses does not grow without bound.
	sweptAt time.Time
	// now is the clock, overridable in tests.
	now func() time.Time
}

type memoryWindow struct {
	count   uint64
	expires time.Time
}

// NewMemoryCounter returns an empty in-process counter.
func NewMemoryCounter() *MemoryCounter {
	return &MemoryCounter{windows: make(map[string]*memoryWindow), now: time.Now}
}

// memorySweepInterval is how often an increment also walks the map. It is a
// multiple of any realistic window, so the walk is rare relative to arrivals.
const memorySweepInterval = 10 * time.Minute

// Increment adds one to the count for key and returns the new total.
//
// The expiry is set when the window opens and never extended, which is what
// makes this a fixed window: every key in one window resets together, at the
// instant X-RateLimit-Reset reported.
func (c *MemoryCounter) Increment(_ context.Context, key string, window time.Duration) (uint64, error) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.windows == nil {
		c.windows = make(map[string]*memoryWindow)
	}
	c.sweepLocked(now)
	current, ok := c.windows[key]
	if !ok || !now.Before(current.expires) {
		current = &memoryWindow{expires: WindowReset(now, window)}
		c.windows[key] = current
	}
	current.count++
	return current.count, nil
}

// sweepLocked drops keys whose window has passed, at most once per interval.
func (c *MemoryCounter) sweepLocked(now time.Time) {
	if !c.sweptAt.IsZero() && now.Sub(c.sweptAt) < memorySweepInterval {
		return
	}
	c.sweptAt = now
	for key, window := range c.windows {
		if !now.Before(window.expires) {
			delete(c.windows, key)
		}
	}
}
