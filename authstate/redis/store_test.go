package redis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornweb/authstate"
)

type stringCodec struct{}

func (stringCodec) Encode(value string) ([]byte, error) {
	if value == "encode-error" {
		return nil, errors.New("secret encode error")
	}
	return append([]byte{1}, value...), nil
}

func (stringCodec) Decode(encoded []byte) (string, error) {
	if len(encoded) == 0 || encoded[0] != 1 || string(encoded[1:]) == "decode-error" {
		return "", errors.New("secret decode error")
	}
	return string(encoded[1:]), nil
}

type fakeClient struct {
	mu      sync.Mutex
	values  map[string]string
	expiry  map[string]time.Time
	now     func() time.Time
	err     error
	lastTTL time.Duration
}

func newFakeClient(now func() time.Time) *fakeClient {
	return &fakeClient{values: map[string]string{}, expiry: map[string]time.Time{}, now: now}
}

func (c *fakeClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *goredis.BoolCmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return goredis.NewBoolResult(false, c.err)
	}
	if expiry, ok := c.expiry[key]; ok && !expiry.After(c.now()) {
		delete(c.values, key)
		delete(c.expiry, key)
	}
	if _, ok := c.values[key]; ok {
		return goredis.NewBoolResult(false, nil)
	}
	c.lastTTL = expiration
	c.values[key] = string(value.([]byte))
	c.expiry[key] = c.now().Add(expiration)
	return goredis.NewBoolResult(true, nil)
}

func (c *fakeClient) GetDel(ctx context.Context, key string) *goredis.StringCmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return goredis.NewStringResult("", c.err)
	}
	value, ok := c.values[key]
	if !ok {
		return goredis.NewStringResult("", goredis.Nil)
	}
	delete(c.values, key)
	delete(c.expiry, key)
	return goredis.NewStringResult(value, nil)
}

func newTestStore(t *testing.T, now func() time.Time) (*Store[string], *fakeClient) {
	t.Helper()
	client := newFakeClient(now)
	store, err := newStore[string](client, stringCodec{}, Options{Prefix: "petitweb:", Namespace: "test", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return store, client
}

func TestStorePutTakeAndDuplicate(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, client := newTestStore(t, func() time.Time { return now })
	expiresAt := now.Add(time.Second + time.Nanosecond)
	if err := store.Put(context.Background(), "key", "value", expiresAt); err != nil {
		t.Fatal(err)
	}
	if client.lastTTL != time.Second+time.Millisecond {
		t.Fatalf("TTL = %s, want 1.001s", client.lastTTL)
	}
	if err := store.Put(context.Background(), "key", "other", expiresAt); !errors.Is(err, authstate.ErrAlreadyExists) {
		t.Fatalf("duplicate Put error = %v", err)
	}
	value, err := store.Take(context.Background(), "key")
	if err != nil || value != "value" {
		t.Fatalf("Take = (%q, %v)", value, err)
	}
	if _, err := store.Take(context.Background(), "key"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("second Take error = %v", err)
	}
}

func TestStoreConcurrentTakeReturnsValueOnce(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	store, _ := newTestStore(t, func() time.Time { return now })
	if err := store.Put(context.Background(), "key", "value", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Take(context.Background(), "key")
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
			t.Fatalf("Take error = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successful Take count = %d, want 1", success)
	}
}

func TestStoreExpiryCodecBoundsAndUnavailable(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	clock := now
	store, client := newTestStore(t, func() time.Time { return clock })
	if err := store.Put(context.Background(), "expired", "value", now); !errors.Is(err, authstate.ErrInvalidExpiry) {
		t.Fatalf("expired Put error = %v", err)
	}
	if err := store.Put(context.Background(), "expires", "value", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	clock = now.Add(time.Second)
	if _, err := store.Take(context.Background(), "expires"); !errors.Is(err, authstate.ErrExpired) {
		t.Fatalf("expired Take error = %v", err)
	}
	if err := store.Put(context.Background(), "codec", "encode-error", now.Add(time.Minute)); !errors.Is(err, authstate.ErrCodec) || stringsContains(err.Error(), "secret") {
		t.Fatalf("codec Put error = %v", err)
	}
	client.values[store.prefix+"malformed"] = "bad"
	if _, err := store.Take(context.Background(), "malformed"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("malformed Take error = %v", err)
	}
	client.err = errors.New("server at redis://user:password@example")
	if _, err := store.Take(context.Background(), "key"); !errors.Is(err, authstate.ErrUnavailable) || stringsContains(err.Error(), "password") {
		t.Fatalf("unavailable Take error = %v", err)
	}
	client.err = nil
	if err := store.Put(context.Background(), "reconnect", "value", clock.Add(time.Minute)); err != nil {
		t.Fatalf("Put after recovery error = %v", err)
	}
}

func TestStoreRejectsInvalidOptionsAndCancellation(t *testing.T) {
	client := newFakeClient(time.Now)
	for _, options := range []Options{{}, {Prefix: "bad prefix", Namespace: "test"}, {Prefix: "ok", Namespace: "bad:value"}} {
		if _, err := newStore[string](client, stringCodec{}, options); !errors.Is(err, authstate.ErrInvalidOptions) {
			t.Fatalf("NewStore(%+v) error = %v", options, err)
		}
	}
	store, _ := newTestStore(t, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, "key", "value", time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Put error = %v", err)
	}
}

func stringsContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
