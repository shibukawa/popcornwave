package pwratelimit

import (
	"context"
	"sync"
	"time"
)

// memoryShardCount fans the key space out over independent locks. Every
// rate-limited request takes exactly one of them, so admissions only contend
// when they hash to the same shard — one process-wide mutex serialized all of
// them, and its periodic sweep stalled every admission while it walked the
// whole map. A power of two keeps the modulo a mask.
const memoryShardCount = 16

// MemoryCounter counts inside this process.
//
// It is the default because a limiter that needs a dependency before it starts
// is one nobody switches on. It is also only correct on one replica: N of them
// each enforce the configured limit, so the effective limit is N times what the
// deployment declared. That is a property of the choice rather than a defect,
// and pw doctor reports it.
type MemoryCounter struct {
	shards [memoryShardCount]memoryShard
	// now is the clock, overridable in tests.
	now func() time.Time
}

// memoryShard is one lock's worth of the key space. Each shard sweeps itself,
// which both bounds a sweep to one shard's keys and staggers the walks instead
// of stopping the world every interval.
type memoryShard struct {
	mu sync.Mutex
	// windows holds one counter per key for the window it names. An entry
	// whose window has passed is replaced on the next arrival rather than
	// swept, so an idle key costs one map entry until something touches it.
	windows map[string]*memoryWindow
	// sweptAt bounds how often this shard is walked for expired keys, so a
	// deployment cycling through many addresses does not grow without bound.
	sweptAt time.Time
}

type memoryWindow struct {
	count   uint64
	expires time.Time
}

// NewMemoryCounter returns an empty in-process counter.
func NewMemoryCounter() *MemoryCounter {
	return &MemoryCounter{now: time.Now}
}

// memorySweepInterval is how often an increment also walks its shard. It is a
// multiple of any realistic window, so the walk is rare relative to arrivals.
const memorySweepInterval = 10 * time.Minute

// Increment adds one to the count for key and returns the new total.
//
// The expiry is set when the window opens and never extended, which is what
// makes this a fixed window: every key in one window resets together, at the
// instant X-RateLimit-Reset reported.
func (c *MemoryCounter) Increment(_ context.Context, key string, window time.Duration) (uint64, error) {
	now := c.now()
	shard := &c.shards[memoryShardOf(key)]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if shard.windows == nil {
		shard.windows = make(map[string]*memoryWindow)
	}
	shard.sweepLocked(now)
	current, ok := shard.windows[key]
	if !ok || !now.Before(current.expires) {
		current = &memoryWindow{expires: WindowReset(now, window)}
		shard.windows[key] = current
	}
	current.count++
	return current.count, nil
}

// memoryShardOf places a key on its shard, by FNV-1a over the key bytes.
func memoryShardOf(key string) uint32 {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	hash := uint32(offset)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime
	}
	return hash % memoryShardCount
}

// sweepLocked drops keys whose window has passed, at most once per interval.
func (s *memoryShard) sweepLocked(now time.Time) {
	if !s.sweptAt.IsZero() && now.Sub(s.sweptAt) < memorySweepInterval {
		return
	}
	s.sweptAt = now
	for key, window := range s.windows {
		if !now.Before(window.expires) {
			delete(s.windows, key)
		}
	}
}
