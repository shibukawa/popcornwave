package auth

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/sessionconfig"
)

func hintSecret() string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
}

func enabledHint() HintConfig {
	return HintConfig{
		Enabled: true, Name: "pw_hint", Secret: hintSecret(),
		TTL: 720 * time.Hour, IdleTimeout: 336 * time.Hour,
	}
}

// The hint carries no authority. It is sealed so its contents never reach the
// client, which is what lets it hold a login identifier at all.
func TestTheHintRoundTripsSealed(t *testing.T) {
	jar, err := hintJar(enabledHint(), sessionconfig.SessionCookieConfig{})
	if err != nil {
		t.Fatal(err)
	}
	rt := &runtime{hint: jar, config: Config{Assurance: AssuranceConfig{Hint: enabledHint()}}}

	recorder := httptest.NewRecorder()
	rt.rememberSignIn(HTTPExchange(recorder, httptest.NewRequest("GET", "/", nil)), SessionData{DisplayName: "Ada", Email: "ada@example.com", Issuer: "https://issuer.example"})
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	if strings.Contains(cookies[0].Value, "ada") || strings.Contains(cookies[0].Value, "issuer.example") {
		t.Fatal("the sealed cookie leaked its contents to the client")
	}

	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(cookies[0])
	got, ok := rt.readSignInHint(HTTPExchange(httptest.NewRecorder(), request))
	if !ok || got.DisplayName != "Ada" || got.Issuer != "https://issuer.example" {
		t.Fatalf("hint = %+v, %v", got, ok)
	}
}

// The issuer is the part no protocol can supply: a provider knows nothing about
// the other providers a deployment offers, so a multi-issuer login screen can
// only skip its picker from local memory.
func TestTheHintRemembersTheIssuer(t *testing.T) {
	jar, _ := hintJar(enabledHint(), sessionconfig.SessionCookieConfig{})
	rt := &runtime{hint: jar, config: Config{Assurance: AssuranceConfig{Hint: enabledHint()}}}
	recorder := httptest.NewRecorder()
	rt.rememberSignIn(HTTPExchange(recorder, httptest.NewRequest("GET", "/", nil)), SessionData{Issuer: "https://accounts.google.com"})
	request := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	got, ok := rt.readSignInHint(HTTPExchange(httptest.NewRecorder(), request))
	if !ok || got.Issuer != "https://accounts.google.com" {
		t.Fatalf("issuer = %q, %v", got.Issuer, ok)
	}
}

// Exceeding either bound drops the browser to anonymous, which is the state a
// deployment that never enabled the hint is always in.
func TestAnIdleHintIsDiscardedRatherThanShown(t *testing.T) {
	config := enabledHint()
	config.IdleTimeout = time.Hour
	jar, _ := hintJar(config, sessionconfig.SessionCookieConfig{})
	rt := &runtime{hint: jar, config: Config{Assurance: AssuranceConfig{Hint: config}}}

	recorder := httptest.NewRecorder()
	rt.rememberSignIn(HTTPExchange(recorder, httptest.NewRequest("GET", "/", nil)), SessionData{DisplayName: "Ada"})
	request := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	// Rewind the last login past the idle bound without touching the absolute
	// one the cookie carries, so only the inactivity rule can reject it.
	rt.config.Assurance.Hint.IdleTimeout = time.Nanosecond
	clearing := httptest.NewRecorder()
	if _, ok := rt.readSignInHint(HTTPExchange(clearing, request)); ok {
		t.Fatal("an idle hint was shown")
	}
	if len(clearing.Result().Cookies()) == 0 {
		t.Fatal("the idle hint was not cleared")
	}
}

// Turning the hint off, or giving it no lifetime at all, is a valid answer and
// produces no jar.
func TestNoHintIsKeptWhenItIsOffOrHasNoLifetime(t *testing.T) {
	if jar, err := hintJar(HintConfig{}, sessionconfig.SessionCookieConfig{}); err != nil || jar != nil {
		t.Fatalf("disabled hint = %v, %v", jar, err)
	}
	config := enabledHint()
	config.TTL = 0
	if jar, err := hintJar(config, sessionconfig.SessionCookieConfig{}); err != nil || jar != nil {
		t.Fatalf("zero-lifetime hint = %v, %v", jar, err)
	}
}

// An unusable secret is refused rather than downgraded: a hint that cannot be
// sealed would have to be written in the clear or silently dropped.
func TestAnUnsealableHintIsRefused(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.Assurance.Hint = enabledHint()
	config.Assurance.Hint.Secret = "too-short"
	err := config.validateShape()
	if err == nil || !strings.Contains(err.Error(), "hint.secret") {
		t.Fatalf("bad secret = %v", err)
	}
	if strings.Contains(err.Error(), "too-short") {
		t.Fatal("the error repeated the secret it rejected")
	}
}

// Remembering the last user is precisely what a shared-device deployment exists
// to prevent, so the two settings cannot both be set.
func TestSharedDeviceForbidsTheHint(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.SharedDevice = true
	config.OIDC.LogoutScope = LogoutScopeGlobal
	config.Assurance.Hint = enabledHint()
	if err := config.validateShape(); err == nil || !strings.Contains(err.Error(), "shared_device") {
		t.Fatalf("shared device with a hint = %v", err)
	}
}

// The mask is a fixed run rather than one mark per character, so it discloses
// the first character and the domain and nothing else, not even the length.
func TestMaskIdentifierKeepsTheDomainAndHidesTheRest(t *testing.T) {
	cases := map[string]string{
		"ada@example.com":              "a•••••@example.com",
		"a-very-long-name@example.com": "a•••••@example.com",
		"ada":                          "a•••••",
		"a":                            "a•••••",
		"":                             "",
	}
	for input, want := range cases {
		if got := MaskIdentifier(input); got != want {
			t.Fatalf("MaskIdentifier(%q) = %q, want %q", input, got, want)
		}
	}
}
