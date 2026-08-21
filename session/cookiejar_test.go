package session

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type preferences struct {
	Theme string `json:"theme"`
	Seats int    `json:"seats"`
}

func testJar(t *testing.T, options JarOptions) *Jar[preferences] {
	t.Helper()
	if options.Cookie.Name == "" {
		options.Cookie.Name = "pw_prefs"
	}
	jar, err := NewJar[preferences](nil, options)
	if err != nil {
		t.Fatalf("NewJar: %v", err)
	}
	return jar
}

// jarCookie returns the jar cookie of a recorded response.
func jarCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	return sessionCookie(t, recorder, name)
}

func TestJarRoundTripsThroughTheBrowserInEveryMode(t *testing.T) {
	keys := testKeyring(t, 1)
	for _, mode := range []CookieMode{CookiePlain, CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			jar := testJar(t, JarOptions{
				Mode:   mode,
				Keys:   keys,
				Cookie: CookieOptions{Name: "pw_prefs", Secure: true, HTTPOnly: true},
				MaxAge: time.Hour,
			})
			recorder := httptest.NewRecorder()
			if err := jar.Save(recorder, preferences{Theme: "dark", Seats: 4}); err != nil {
				t.Fatalf("Save: %v", err)
			}
			cookie := jarCookie(t, recorder, "pw_prefs")
			if cookie == nil {
				t.Fatal("no cookie written")
			}
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("cookie policy = %#v", cookie)
			}
			if cookie.MaxAge <= 0 {
				t.Fatalf("cookie max age = %d", cookie.MaxAge)
			}

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.AddCookie(cookie)
			loaded, err := jar.Load(request)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if loaded != (preferences{Theme: "dark", Seats: 4}) {
				t.Fatalf("loaded = %#v", loaded)
			}
		})
	}
}

func TestPlainJarAcceptsAClientWrittenValue(t *testing.T) {
	jar := testJar(t, JarOptions{Mode: CookiePlain, Cookie: CookieOptions{Name: "pw_prefs"}})
	// A plain cookie is the client's to edit, so a value the browser authored
	// is ordinary request input rather than an attack.
	authored := "1.p." + base64.RawURLEncoding.EncodeToString([]byte(`{"theme":"light","seats":2}`))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "pw_prefs", Value: authored})
	loaded, err := jar.Load(request)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded != (preferences{Theme: "light", Seats: 2}) {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestProtectedJarRefusesAClientWrittenValue(t *testing.T) {
	keys := testKeyring(t, 1)
	authored := "1.p." + base64.RawURLEncoding.EncodeToString([]byte(`{"theme":"light","seats":99}`))
	for _, mode := range []CookieMode{CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			jar := testJar(t, JarOptions{Mode: mode, Keys: keys, Cookie: CookieOptions{Name: "pw_prefs"}})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.AddCookie(&http.Cookie{Name: "pw_prefs", Value: authored})
			if _, err := jar.Load(request); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("Load error = %v", err)
			}
		})
	}
}

func TestSignedJarLeavesTheValueReadableToTheClient(t *testing.T) {
	jar := testJar(t, JarOptions{
		Mode:   CookieSigned,
		Keys:   testKeyring(t, 1),
		Cookie: CookieOptions{Name: "pw_prefs"},
	})
	recorder := httptest.NewRecorder()
	if err := jar.Save(recorder, preferences{Theme: "dark", Seats: 4}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Signed protects integrity, not confidentiality: a client that decodes
	// the body still sees the payload, which is why a secret never goes here.
	body := strings.Split(strings.TrimPrefix(jarCookie(t, recorder, "pw_prefs").Value, "1.s."), ".")[0]
	decoded, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || !strings.Contains(string(decoded), `"theme":"dark"`) {
		t.Fatalf("signed body = %q, %v", decoded, err)
	}
}

func TestJarMiddlewarePublishesAndUpdatesTheValue(t *testing.T) {
	jar := testJar(t, JarOptions{
		Mode:   CookieSealed,
		Keys:   testKeyring(t, 1),
		Cookie: CookieOptions{Name: "pw_prefs"},
		MaxAge: time.Hour,
	})

	write := httptest.NewRecorder()
	handler := jar.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		value, ok := jar.Value(r.Context())
		if !ok {
			t.Error("no jar handle on the request")
			return
		}
		if _, present := value.Get(); present {
			t.Error("first request already carried a value")
		}
		if err := value.Set(preferences{Theme: "dark", Seats: 4}); err != nil {
			t.Errorf("Set: %v", err)
		}
		// A write is visible to the rest of the request without a re-read.
		if current, present := jar.Read(r.Context()); !present || current.Theme != "dark" {
			t.Errorf("value after Set = %#v present=%v", current, present)
		}
	}))
	handler.ServeHTTP(write, httptest.NewRequest(http.MethodGet, "/", nil))

	cookie := jarCookie(t, write, "pw_prefs")
	if cookie == nil {
		t.Fatal("no cookie written")
	}
	var seen preferences
	var present bool
	read := jar.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, present = jar.Read(r.Context())
	}))
	next := httptest.NewRequest(http.MethodGet, "/", nil)
	next.AddCookie(cookie)
	read.ServeHTTP(httptest.NewRecorder(), next)
	if !present || seen != (preferences{Theme: "dark", Seats: 4}) {
		t.Fatalf("second request value = %#v present=%v", seen, present)
	}
}

func TestJarMiddlewareClearsAValueItDidNotWrite(t *testing.T) {
	jar := testJar(t, JarOptions{
		Mode:   CookieSealed,
		Keys:   testKeyring(t, 1),
		Cookie: CookieOptions{Name: "pw_prefs"},
	})
	var present bool
	handler := jar.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, present = jar.Read(r.Context())
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "pw_prefs", Value: "1.e.not-a-value"})
	handler.ServeHTTP(recorder, request)

	if present {
		t.Fatal("a foreign value was published to the handler")
	}
	cleared := jarCookie(t, recorder, "pw_prefs")
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("foreign value was not cleared: %#v", cleared)
	}
}

func TestJarClearEndsTheValue(t *testing.T) {
	jar := testJar(t, JarOptions{Mode: CookiePlain, Cookie: CookieOptions{Name: "pw_prefs"}})
	recorder := httptest.NewRecorder()
	jar.Clear(recorder)
	cookie := jarCookie(t, recorder, "pw_prefs")
	if cookie == nil || cookie.MaxAge >= 0 || cookie.Value != "" {
		t.Fatalf("cleared cookie = %#v", cookie)
	}
}

func TestJarRefusesAValueTheBrowserWouldDrop(t *testing.T) {
	jar := testJar(t, JarOptions{
		Mode:   CookieSealed,
		Keys:   testKeyring(t, 1),
		Cookie: CookieOptions{Name: "pw_prefs"},
	})
	// Browsers discard an oversized cookie silently, so the write fails here
	// instead of looking like a value that never comes back.
	err := jar.Save(httptest.NewRecorder(), preferences{Theme: strings.Repeat("x", DefaultMaxCookieBytes)})
	if !errors.Is(err, ErrCookieTooLarge) {
		t.Fatalf("oversized Save error = %v", err)
	}
}

func TestJarExpiryIsEnforcedByTheServer(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	jar := testJar(t, JarOptions{
		Mode:   CookieSealed,
		Keys:   testKeyring(t, 1),
		Cookie: CookieOptions{Name: "pw_prefs"},
		MaxAge: time.Hour,
		Now:    c.Now,
	})
	recorder := httptest.NewRecorder()
	if err := jar.Save(recorder, preferences{Theme: "dark"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cookie := jarCookie(t, recorder, "pw_prefs")

	c.advance(2 * time.Hour)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	if _, err := jar.Load(request); !errors.Is(err, ErrExpired) {
		t.Fatalf("Load after expiry error = %v", err)
	}
}

func TestJarRejectsAnUnsafePolicy(t *testing.T) {
	cases := map[string]JarOptions{
		"missing name":  {Mode: CookiePlain},
		"invalid name":  {Mode: CookiePlain, Cookie: CookieOptions{Name: "pw prefs"}},
		"relative path": {Mode: CookiePlain, Cookie: CookieOptions{Name: "pw_prefs", Path: "prefs"}},
		"insecure same-site none": {
			Mode:   CookiePlain,
			Cookie: CookieOptions{Name: "pw_prefs", SameSite: http.SameSiteNoneMode, AllowInsecure: true},
		},
		"oversized budget": {Mode: CookiePlain, Cookie: CookieOptions{Name: "pw_prefs"}, MaxBytes: 1 << 20},
		"negative max age": {Mode: CookiePlain, Cookie: CookieOptions{Name: "pw_prefs"}, MaxAge: -time.Second},
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewJar[preferences](nil, options); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("NewJar error = %v", err)
			}
		})
	}
}

func TestJarsDoNotReadEachOther(t *testing.T) {
	keys := testKeyring(t, 1)
	first := testJar(t, JarOptions{Mode: CookieSealed, Keys: keys, Cookie: CookieOptions{Name: "pw_first"}})
	second := testJar(t, JarOptions{Mode: CookieSealed, Keys: keys, Cookie: CookieOptions{Name: "pw_second"}})

	var firstSeen, secondSeen bool
	handler := first.Middleware()(second.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, firstSeen = first.Read(r.Context())
		_, secondSeen = second.Read(r.Context())
	})))
	recorder := httptest.NewRecorder()
	if err := first.Save(recorder, preferences{Theme: "dark"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(jarCookie(t, recorder, "pw_first"))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if !firstSeen || secondSeen {
		t.Fatalf("first=%v second=%v", firstSeen, secondSeen)
	}
}
