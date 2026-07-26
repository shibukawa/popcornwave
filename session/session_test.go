package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type payload struct {
	AccountID string `json:"account_id"`
}

// mapStore is a minimal in-process Store used to exercise Manager semantics.
type mapStore struct {
	sync.Mutex
	records map[string]Record[payload]
	failing bool
	puts    int
	touches int
}

func newMapStore() *mapStore {
	return &mapStore{records: make(map[string]Record[payload])}
}

func (s *mapStore) Put(_ context.Context, keyHash string, record Record[payload]) error {
	s.Lock()
	defer s.Unlock()
	if s.failing {
		return ErrUnavailable
	}
	s.puts++
	s.records[keyHash] = record
	return nil
}

func (s *mapStore) Get(_ context.Context, keyHash string) (Record[payload], error) {
	s.Lock()
	defer s.Unlock()
	if s.failing {
		return Record[payload]{}, ErrUnavailable
	}
	record, ok := s.records[keyHash]
	if !ok {
		return Record[payload]{}, ErrNotFound
	}
	return record, nil
}

func (s *mapStore) Touch(_ context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	s.Lock()
	defer s.Unlock()
	record, ok := s.records[keyHash]
	if !ok {
		return ErrNotFound
	}
	s.touches++
	record.LastSeenAt = lastSeenAt
	record.IdleExpiresAt = idleExpiresAt
	s.records[keyHash] = record
	return nil
}

func (s *mapStore) Delete(_ context.Context, keyHash string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.records, keyHash)
	return nil
}

func (s *mapStore) len() int {
	s.Lock()
	defer s.Unlock()
	return len(s.records)
}

type clock struct {
	sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.Lock()
	defer c.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.Lock()
	defer c.Unlock()
	c.now = c.now.Add(d)
}

func testManager(t *testing.T, store Store[payload], options Options[payload]) *Manager[payload] {
	t.Helper()
	manager, err := NewManager(store, options)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
}

func defaultOptions(now func() time.Time) Options[payload] {
	return Options[payload]{
		TTL:             time.Hour,
		IdleTimeout:     30 * time.Minute,
		RenewalInterval: 5 * time.Minute,
		Cookie:          CookieOptions{Secure: true, HTTPOnly: true},
		Method:          "test",
		Subject:         func(p payload) string { return p.AccountID },
		Now:             now,
	}
}

// sessionCookie returns the manager cookie written to the recorded response.
func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestManagerCreatesOpaqueCookieAndResolvesSession(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := manager.Create(recorder, request, payload{AccountID: "account-1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cookie := sessionCookie(t, recorder, DefaultCookieName)
	if cookie == nil {
		t.Fatal("no session cookie")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie policy = %#v", cookie)
	}
	if !validToken(cookie.Value) {
		t.Fatalf("cookie value %q is not a canonical token", cookie.Value)
	}
	// The browser value must never be the stored key.
	if _, stored := store.records[cookie.Value]; stored {
		t.Fatal("raw token used as store key")
	}
	if _, stored := store.records[keyHash(cookie.Value)]; !stored {
		t.Fatal("record not stored under the token hash")
	}

	var seen View[payload]
	var found bool
	handler := manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, found = Read[payload](r.Context())
	}))
	next := httptest.NewRequest(http.MethodGet, "/", nil)
	next.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), next)
	if !found || seen.Data.AccountID != "account-1" || seen.Method != "test" {
		t.Fatalf("resolved session = %#v found=%v", seen, found)
	}
}

func TestMiddlewareReportsAuthenticationAndClearsStaleCookie(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))

	create := httptest.NewRecorder()
	if err := manager.Create(create, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(t, create, DefaultCookieName)

	// An expired record is not a failure: the request continues anonymously
	// and the browser cookie is cleared.
	c.advance(2 * time.Hour)
	var authenticated bool
	handler := manager.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, authenticated = Read[payload](r.Context())
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if authenticated {
		t.Fatal("expired session was accepted")
	}
	cleared := sessionCookie(t, recorder, DefaultCookieName)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("stale cookie was not cleared: %#v", cleared)
	}
	if store.len() != 0 {
		t.Fatalf("expired record was not removed: %d", store.len())
	}
}

func TestMiddlewareFailsClosedWhenBackendIsUnavailable(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))
	create := httptest.NewRecorder()
	if err := manager.Create(create, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(t, create, DefaultCookieName)

	store.failing = true
	reached := false
	handler := manager.Middleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if reached {
		t.Fatal("handler ran while the session backend was unavailable")
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestRotateRevokesPreviousRecord(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))

	first := httptest.NewRecorder()
	if err := manager.Create(first, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	firstCookie := sessionCookie(t, first, DefaultCookieName)

	second := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(firstCookie)
	if err := manager.Rotate(second, request, payload{AccountID: "b"}); err != nil {
		t.Fatal(err)
	}
	secondCookie := sessionCookie(t, second, DefaultCookieName)
	if secondCookie.Value == firstCookie.Value {
		t.Fatal("rotation reused the previous token")
	}
	if store.len() != 1 {
		t.Fatalf("stored records = %d, want only the rotated one", store.len())
	}
	if _, err := store.Get(context.Background(), keyHash(firstCookie.Value)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("previous record error = %v", err)
	}
}

func TestDeleteRevokesRecordAndExpiresCookie(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))
	create := httptest.NewRecorder()
	if err := manager.Create(create, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(t, create, DefaultCookieName)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	request.AddCookie(cookie)
	if err := manager.Delete(recorder, request); err != nil {
		t.Fatal(err)
	}
	if store.len() != 0 {
		t.Fatalf("records after delete = %d", store.len())
	}
	cleared := sessionCookie(t, recorder, DefaultCookieName)
	if cleared == nil || cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatalf("cookie after delete = %#v", cleared)
	}
}

func TestRenewalIsBoundedByIntervalAndAbsoluteExpiry(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))
	create := httptest.NewRecorder()
	if err := manager.Create(create, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(t, create, DefaultCookieName)
	handler := manager.Middleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	visit := func() {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.AddCookie(cookie)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	c.advance(time.Minute)
	visit()
	if store.touches != 0 {
		t.Fatalf("touches before the renewal interval = %d", store.touches)
	}
	c.advance(10 * time.Minute)
	visit()
	if store.touches != 1 {
		t.Fatalf("touches after the renewal interval = %d", store.touches)
	}

	// Idle renewal must never move the record past its absolute expiry. At
	// this point the renewed idle window would reach beyond it.
	c.advance(25 * time.Minute)
	visit()
	record, err := store.Get(context.Background(), keyHash(cookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	if record.IdleExpiresAt.After(record.ExpiresAt) {
		t.Fatalf("idle expiry %s passed absolute expiry %s", record.IdleExpiresAt, record.ExpiresAt)
	}
}

func TestVersionMismatchInvalidatesRecord(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	options := defaultOptions(c.Now)
	manager := testManager(t, store, options)
	create := httptest.NewRecorder()
	if err := manager.Create(create, httptest.NewRequest(http.MethodGet, "/", nil), payload{AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(t, create, DefaultCookieName)

	options.Version = 2
	upgraded := testManager(t, store, options)
	authenticated := false
	handler := upgraded.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, authenticated = Read[payload](r.Context())
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if authenticated {
		t.Fatal("record of an older version was accepted")
	}
	if store.len() != 0 {
		t.Fatal("record of an older version was kept")
	}
}

func TestManagerRejectsUnsafeOptions(t *testing.T) {
	store := newMapStore()
	cases := map[string]Options[payload]{
		"missing ttl":     {},
		"idle beyond ttl": {TTL: time.Hour, IdleTimeout: 2 * time.Hour},
		"insecure same-site none": {
			TTL:    time.Hour,
			Cookie: CookieOptions{SameSite: http.SameSiteNoneMode},
		},
		"invalid cookie name": {TTL: time.Hour, Cookie: CookieOptions{Name: "bad name"}},
		"relative cookie path": {
			TTL:    time.Hour,
			Cookie: CookieOptions{Path: "relative"},
		},
	}
	for name, options := range cases {
		if _, err := NewManager(store, options); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("%s: error = %v", name, err)
		}
	}
}

func TestMalformedCookieNeverReachesTheStore(t *testing.T) {
	store := newMapStore()
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	manager := testManager(t, store, defaultOptions(c.Now))
	store.failing = true

	reached := false
	handler := manager.Middleware(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: DefaultCookieName, Value: "not-a-token"})
	handler.ServeHTTP(recorder, request)
	if !reached || recorder.Code != http.StatusOK {
		t.Fatalf("malformed cookie was sent to the backend: status=%d reached=%v", recorder.Code, reached)
	}
}

func TestJSONCodecRejectsTrailingBytes(t *testing.T) {
	codec := JSONCodec[payload]{}
	encoded, err := codec.Encode(payload{AccountID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Decode(encoded); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := codec.Decode(append(encoded, '{')); !errors.Is(err, ErrCodec) {
		t.Fatalf("trailing bytes error = %v", err)
	}
}
