// Package redis counts rate limit arrivals in a shared Redis or Valkey server.
//
// A deployment running more than one replica needs this: the in-process
// counter is correct on one of them and enforces N times the configured limit
// on N of them.
//
//	import _ "github.com/shibukawa/popcornwave/ratelimitstore/redis"
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornwave/pw"
)

// Importing this package registers the redis counter store. Registration opens
// no connection; the client is dialed when ratelimit.backend selects it.
func init() {
	pw.RegisterRateLimitStore(pw.RateLimitBackendRedis, open)
}

// defaultConnectTimeout bounds the startup ping and the per-command deadlines
// when ratelimit.redis.connect_timeout is unset.
const defaultConnectTimeout = 5 * time.Second

// maxPrefixBytes bounds the key space name, so a mistyped value cannot build
// keys the server refuses one arrival at a time.
const maxPrefixBytes = 128

// open dials the configured server and refuses to start against one it cannot
// reach.
//
// Starting anyway would leave a limiter that fails open on every request, which
// is the degraded state made permanent and invisible.
func open(ctx context.Context, config pw.RateLimitConfig) (pw.RateLimitCounter, func(context.Context) error, error) {
	dsn := strings.TrimSpace(config.Redis.DSN)
	if dsn == "" {
		return nil, nil, errors.New(`ratelimit.backend = "redis" requires ratelimit.redis.dsn`)
	}
	options, err := goredis.ParseURL(dsn)
	if err != nil {
		// Client text would repeat the URL, which can carry a password.
		return nil, nil, errors.New("ratelimit.redis.dsn is not a redis:// or rediss:// URL")
	}
	timeout := config.Redis.ConnectTimeout
	if timeout <= 0 {
		timeout = defaultConnectTimeout
	}
	options.Protocol = 2
	options.ContextTimeoutEnabled = true
	options.DialTimeout = timeout
	options.ReadTimeout = timeout
	options.WriteTimeout = timeout
	client := goredis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("ratelimit.redis: server did not answer within %s", timeout)
	}
	counter, err := NewCounter(client, Options{KeyPrefix: config.Redis.KeyPrefix})
	if err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("ratelimit.redis: %w", err)
	}
	return counter, func(context.Context) error { return client.Close() }, nil
}

// Options configure a Counter.
type Options struct {
	// KeyPrefix isolates these keys from every other user of the server. It
	// defaults to pw.DefaultRateLimitKeyPrefix.
	KeyPrefix string
}

// Counter is the shared fixed-window counter.
type Counter struct {
	client goredis.UniversalClient
	prefix string
}

// NewCounter validates the key space and returns the counter.
func NewCounter(client goredis.UniversalClient, options Options) (*Counter, error) {
	if client == nil {
		return nil, errors.New("nil redis client")
	}
	prefix := options.KeyPrefix
	if prefix == "" {
		prefix = pw.DefaultRateLimitKeyPrefix
	}
	if len(prefix) > maxPrefixBytes || strings.ContainsAny(prefix, " \r\n\t") {
		return nil, fmt.Errorf("ratelimit.redis.key_prefix %q is not a usable key space", prefix)
	}
	return &Counter{client: client, prefix: prefix}, nil
}

// KeyPrefix reports the owned key space.
func (c *Counter) KeyPrefix() string { return c.prefix }

// Increment adds one to the count for key and returns the new total.
//
// The window is encoded in the key rather than kept as server state, so every
// caller in one window agrees on which counter they are incrementing without a
// read-modify-write. The expiry is set only when the key is created, which is
// what keeps the window fixed: extending it on every arrival would turn a busy
// caller's window into one that never ends.
//
// INCR and EXPIRE are pipelined, so this costs one round trip.
func (c *Counter) Increment(ctx context.Context, key string, window time.Duration) (uint64, error) {
	if window <= 0 {
		return 0, errors.New("ratelimit.redis: window must be positive")
	}
	// The window's own start instant names the bucket, so a key rolls over on
	// its own and a stale one expires without anyone sweeping it.
	bucket := time.Now().Truncate(window).UnixMilli()
	full := fmt.Sprintf("%s%s:%d", c.prefix, key, bucket)
	pipeline := c.client.Pipeline()
	incremented := pipeline.Incr(ctx, full)
	// A margin over the window so a counter outlives the requests reading it
	// through a slow round trip, without outliving the next window's key.
	pipeline.ExpireNX(ctx, full, window+time.Second)
	if _, err := pipeline.Exec(ctx); err != nil {
		return 0, err
	}
	count, err := incremented.Result()
	if err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, fmt.Errorf("ratelimit.redis: counter returned %d", count)
	}
	return uint64(count), nil
}
