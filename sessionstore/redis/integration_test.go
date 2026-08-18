//go:build !tinygo

package redis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornweb/session"
)

// TestLiveRedisOrValkey exercises the store against a real server. The unit
// tests decide semantics; this one decides that the commands and the TTL
// behave the same way outside the fake. scripts/test-session-redis.sh runs it
// against both servers.
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
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(client, Options{
		KeyPrefix: "pw-test:" + time.Now().Format("150405.000000000") + ":",
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	record := testRecord(now)
	if err := store.Put(ctx, testKey, record); err != nil {
		t.Fatalf("Put: %v", err)
	}
	loaded, err := store.Get(ctx, testKey)
	if err != nil || string(loaded.Payload) != string(record.Payload) || loaded.Method != record.Method {
		t.Fatalf("Get = (%#v, %v)", loaded, err)
	}
	// The server holds a TTL, not just a stored timestamp.
	ttl, err := client.TTL(ctx, store.KeyPrefix()+testKey).Result()
	if err != nil || ttl <= 0 || ttl > 30*time.Minute {
		t.Fatalf("server ttl = (%v, %v)", ttl, err)
	}

	renewed := now.Add(45 * time.Minute)
	if err := store.Touch(ctx, testKey, now.Add(time.Minute), renewed); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	loaded, err = store.Get(ctx, testKey)
	if err != nil || !loaded.IdleExpiresAt.Equal(renewed.Truncate(time.Millisecond)) {
		t.Fatalf("renewed = (%#v, %v)", loaded, err)
	}

	if err := store.Delete(ctx, testKey); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, testKey); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Get after Delete error = %v", err)
	}
	// A renewal never recreates a key the server no longer holds.
	if err := store.Touch(ctx, testKey, now, now.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Touch after Delete error = %v", err)
	}

	// The server collects an expired record on its own, with no sweep.
	short := testRecord(time.Now())
	short.ExpiresAt = time.Now().Add(30 * time.Millisecond)
	short.IdleExpiresAt = time.Time{}
	if err := store.Put(ctx, testKey, short); err != nil {
		t.Fatalf("Put: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := store.Get(ctx, testKey); !errors.Is(err, session.ErrNotFound) && !errors.Is(err, session.ErrExpired) {
		t.Fatalf("expired Get error = %v", err)
	}
}
