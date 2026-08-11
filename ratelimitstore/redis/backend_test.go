//go:build !tinygo

package redis

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornwave/pwratelimit"
)

func TestNewCounterValidatesTheKeySpace(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	defer client.Close()

	counter, err := NewCounter(client, Options{})
	if err != nil {
		t.Fatalf("an empty prefix was refused: %v", err)
	}
	if counter.KeyPrefix() != pwratelimit.DefaultKeyPrefix {
		t.Errorf("KeyPrefix = %q, want the framework default", counter.KeyPrefix())
	}
	if _, err := NewCounter(nil, Options{}); err == nil {
		t.Error("a nil client was accepted")
	}
	if _, err := NewCounter(client, Options{KeyPrefix: "has space:"}); err == nil {
		t.Error("a prefix carrying whitespace was accepted")
	}
	if _, err := NewCounter(client, Options{KeyPrefix: strings.Repeat("x", maxPrefixBytes+1)}); err == nil {
		t.Error("an oversized prefix was accepted")
	}
}

func TestOpenRefusesAnUnusableDSN(t *testing.T) {
	for name, config := range map[string]pwratelimit.Config{
		"no dsn":      {Redis: pwratelimit.RedisConfig{}},
		"not a url":   {Redis: pwratelimit.RedisConfig{DSN: "localhost:6379"}},
		"wrong sche+": {Redis: pwratelimit.RedisConfig{DSN: "postgres://localhost:5432/db"}},
	} {
		if _, _, err := open(context.Background(), config); err == nil {
			t.Errorf("%s: accepted", name)
		} else if strings.Contains(err.Error(), config.Redis.DSN) && config.Redis.DSN != "" {
			// A DSN can carry a password, so it never reaches an error string.
			t.Errorf("%s: the error repeated the DSN: %v", name, err)
		}
	}
}

// TestLiveRedisOrValkey decides that INCR and the expiry behave as the fixed
// window needs outside a fake. scripts/test-ratelimit-redis.sh runs it against
// both servers.
func TestLiveRedisOrValkey(t *testing.T) {
	address := os.Getenv("PETITWEB_REDIS_ADDR")
	if address == "" {
		t.Skip("PETITWEB_REDIS_ADDR is not set")
	}
	client := goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs: []string{address}, Protocol: 2, ContextTimeoutEnabled: true,
		DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
	})
	defer client.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	counter, err := NewCounter(client, Options{KeyPrefix: "pwtest:ratelimit:"})
	if err != nil {
		t.Fatal(err)
	}
	key := "address:203.0.113.9"
	for want := uint64(1); want <= 3; want++ {
		count, err := counter.Increment(ctx, key, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("count = %d, want %d", count, want)
		}
	}
	// Two callers are two keys, which is what makes the bucket per client.
	other, err := counter.Increment(ctx, "address:198.51.100.4", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if other != 1 {
		t.Fatalf("a second key started at %d", other)
	}
	// The expiry is set when the key is created and never extended, or a busy
	// caller's window would never end.
	bucket := time.Now().Truncate(time.Minute).UnixMilli()
	full := counter.KeyPrefix() + key + ":" + strconv.FormatInt(bucket, 10)
	ttl, err := client.TTL(ctx, full).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > time.Minute+2*time.Second {
		t.Fatalf("TTL = %s, want a bounded window", ttl)
	}
	before := ttl
	if _, err := counter.Increment(ctx, key, time.Minute); err != nil {
		t.Fatal(err)
	}
	after, err := client.TTL(ctx, full).Result()
	if err != nil {
		t.Fatal(err)
	}
	if after > before {
		t.Fatalf("TTL grew from %s to %s; the window is not fixed", before, after)
	}
	_ = client.Del(ctx, full,
		counter.KeyPrefix()+"address:198.51.100.4:"+strconv.FormatInt(bucket, 10)).Err()
}
