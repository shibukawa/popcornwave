//go:build !tinygo

package redis

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornwave/contrib/authstate"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore[string](client, stringCodec{}, Options{
		Prefix: "petitweb-test:", Namespace: time.Now().Format("150405.000000000"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "session", "value", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, "session", "duplicate", time.Now().Add(time.Minute)); !errors.Is(err, authstate.ErrAlreadyExists) {
		t.Fatalf("duplicate Put error = %v", err)
	}
	value, err := store.Take(ctx, "session")
	if err != nil || value != "value" {
		t.Fatalf("Take = (%q, %v)", value, err)
	}
	if _, err := store.Take(ctx, "session"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("second Take error = %v", err)
	}

	if err := store.Put(ctx, "expires", "value", time.Now().Add(20*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := store.Take(ctx, "expires"); !errors.Is(err, authstate.ErrNotFound) && !errors.Is(err, authstate.ErrExpired) {
		t.Fatalf("expired Take error = %v", err)
	}

	if err := store.Put(ctx, "concurrent", "once", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Take(ctx, "concurrent")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, authstate.ErrNotFound) {
			t.Fatalf("concurrent Take error = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successful concurrent Take count = %d, want 1", success)
	}

	if err := client.Set(ctx, store.prefix+"malformed", "bad", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Take(ctx, "malformed"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("malformed Take error = %v", err)
	}
	if err := store.Put(ctx, "codec", "decode-error", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Take(ctx, "codec"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("codec Take error = %v", err)
	}

	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if err := store.Put(canceled, "canceled", "value", time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Put error = %v", err)
	}

	// Force the current normal connection closed. The next command must create
	// a new connection rather than leaving the Store unusable.
	_ = client.Do(ctx, "CLIENT", "KILL", "TYPE", "normal", "SKIPME", "no").Err()
	if err := store.Put(ctx, "reconnect", "value", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("Put after connection kill error = %v", err)
	}
	if value, err := store.Take(ctx, "reconnect"); err != nil || value != "value" {
		t.Fatalf("Take after reconnect = (%q, %v)", value, err)
	}
}
