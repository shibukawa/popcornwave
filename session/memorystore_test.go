package session

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestMemoryStoreCopiesPayloadAndEnforcesExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store := NewMemoryStore(func() time.Time { return now })
	key := keyHash("0123456789012345678901234567890123456789012")
	payload := []byte("value")
	if err := store.Put(context.Background(), key, RawRecord{Payload: payload, ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	payload[0] = 'X'
	record, err := store.Get(context.Background(), key)
	if err != nil || string(record.Payload) != "value" {
		t.Fatalf("Get = %q, %v", record.Payload, err)
	}
	record.Payload[0] = 'Y'
	record, _ = store.Get(context.Background(), key)
	if string(record.Payload) != "value" {
		t.Fatalf("caller mutated stored payload: %q", record.Payload)
	}
	now = now.Add(2 * time.Minute)
	if _, err := store.Get(context.Background(), key); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Get error = %v", err)
	}
	if _, err := store.Get(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired record was retained: %v", err)
	}
}

func TestServerSideAnonymousKeepsPrivateStateOutOfTheRecordCookie(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	store := NewMemoryStore(c.Now)
	manager := testManager(t, accountRegistry(t), store, Options{
		TTL:                 time.Hour,
		Cookie:              CookieOptions{HTTPOnly: true},
		ServerSideAnonymous: true,
		Now:                 c.Now,
	})

	first := run(manager, nil, func(_ http.ResponseWriter, r *http.Request) {
		handle, _ := Value[payload](r.Context())
		if err := handle.Set(payload{AccountID: "dev"}); err != nil {
			t.Fatal(err)
		}
	})
	if cookieOf(first, DefaultDataCookieName) != nil {
		t.Fatal("server-side anonymous mode emitted a sealed record cookie")
	}
	if cookieOf(first, DefaultCookieName) == nil {
		t.Fatal("server-side anonymous mode emitted no opaque token")
	}
	run(manager, carry(first), func(_ http.ResponseWriter, r *http.Request) {
		value, ok := Load[payload](r.Context())
		if !ok || value.AccountID != "dev" {
			t.Fatalf("memory session = %#v ok=%v", value, ok)
		}
	})
}

func TestMemoryStoreDefersExpiryLookupUntilSessionAccess(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	store := NewMemoryStore(c.Now)
	manager := testManager(t, accountRegistry(t), store, Options{
		TTL:                 time.Minute,
		Cookie:              CookieOptions{HTTPOnly: true},
		ServerSideAnonymous: true,
		Now:                 c.Now,
	})
	first := run(manager, nil, func(_ http.ResponseWriter, r *http.Request) {
		handle, _ := Value[payload](r.Context())
		if err := handle.Set(payload{AccountID: "dev"}); err != nil {
			t.Fatal(err)
		}
	})
	c.advance(2 * time.Minute)

	untouched := run(manager, carry(first), func(http.ResponseWriter, *http.Request) {})
	if cleared := sessionCookie(t, untouched, DefaultCookieName); cleared != nil {
		t.Fatalf("a route with no session access looked up an expired memory record: %#v", cleared)
	}

	observed := run(manager, carry(first), func(_ http.ResponseWriter, r *http.Request) {
		_, _ = Load[payload](r.Context())
	})
	if cleared := sessionCookie(t, observed, DefaultCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("first session access did not clear the expired memory record: %#v", cleared)
	}
}
