package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/authstate/memory"
	"github.com/shibukawa/popcornweb/contrib/oauth"
)

// authAgeFixture serves discovery, JWKS, and a token endpoint whose ID Token
// claims the test controls, so a provider that ignores max_age can be
// reproduced exactly.
type authAgeFixture struct {
	client *Client
	now    time.Time
	// extra is merged into the ID Token claims of the next token response.
	extra map[string]any
	// nonceSink lets run publish the nonce the authorization request generated
	// so the token endpoint can echo it back.
	nonceSink *string
}

func newAuthAgeFixture(t *testing.T) *authAgeFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	fixture := &authAgeFixture{now: now}
	nonce := ""
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/authorize","token_endpoint":"https://issuer.example/token","jwks_uri":"https://issuer.example/keys"}`)
		case "/keys":
			_, _ = io.WriteString(w, jwksJSON(key))
		case "/token":
			_, _ = io.WriteString(w, `{"access_token":"access","token_type":"Bearer","id_token":"`+idTokenWith(t, key, "https://issuer.example", nonce, now, fixture.extra)+`"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	httpClient := &http.Client{Transport: transport}
	provider, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{HTTPClient: httpClient, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(provider,
		Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"},
		Options{OAuth: oauth.Options{StateStore: store, HTTPClient: httpClient}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	fixture.client = client
	fixture.nonceSink = &nonce
	return fixture
}

// run completes one authorization round trip and returns the authorization
// query alongside the verification result.
func (f *authAgeFixture) run(t *testing.T, begin BeginOptions, callback CallbackOptions) (url.Values, IDToken, error) {
	t.Helper()
	authURL, key, err := f.client.BeginAuthorization(context.Background(), begin)
	if err != nil {
		return nil, IDToken{}, err
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	*f.nonceSink = query.Get("nonce")
	_, token, err := f.client.HandleCallback(context.Background(), key, Callback{State: query.Get("state"), Code: "code"}, callback)
	return query, token, err
}

func TestBeginAuthorizationSendsMaxAgeAndPrompt(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	fixture.extra = map[string]any{"auth_time": fixture.now.Unix()}
	maxAge := 5 * time.Minute
	query, _, err := fixture.run(t, BeginOptions{MaxAge: &maxAge, Prompt: []string{"select_account", "login"}}, CallbackOptions{RequireAuthTime: true})
	if err != nil {
		t.Fatal(err)
	}
	if query.Get("max_age") != "300" {
		t.Fatalf("max_age = %q, want 300", query.Get("max_age"))
	}
	if query.Get("prompt") != "select_account login" {
		t.Fatalf("prompt = %q", query.Get("prompt"))
	}
}

// A zero max_age is meaningful and must reach the provider, so it cannot be
// treated as an unset optional value.
func TestBeginAuthorizationSendsAZeroMaxAge(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	fixture.extra = map[string]any{"auth_time": fixture.now.Unix()}
	zero := time.Duration(0)
	query, _, err := fixture.run(t, BeginOptions{MaxAge: &zero}, CallbackOptions{RequireAuthTime: true})
	if err != nil {
		t.Fatal(err)
	}
	if query.Get("max_age") != "0" {
		t.Fatalf("max_age = %q, want 0", query.Get("max_age"))
	}
}

func TestBeginAuthorizationSendsNeitherByDefault(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	query, _, err := fixture.run(t, BeginOptions{}, CallbackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := query["max_age"]; ok {
		t.Fatal("max_age was sent without being requested")
	}
	if _, ok := query["prompt"]; ok {
		t.Fatal("prompt was sent without being requested")
	}
}

func TestBeginAuthorizationRejectsBadPromptAndMaxAge(t *testing.T) {
	negative := -time.Second
	cases := map[string]BeginOptions{
		"unknown prompt":         {Prompt: []string{"reauthenticate"}},
		"duplicate prompt":       {Prompt: []string{"login", "login"}},
		"none combined":          {Prompt: []string{"none", "login"}},
		"negative max_age":       {MaxAge: &negative},
		"max_age through Params": {Params: map[string]string{"max_age": "300"}},
		"prompt through Params":  {Params: map[string]string{"prompt": "login"}},
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newAuthAgeFixture(t)
			if _, _, err := fixture.client.BeginAuthorization(context.Background(), options); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("err = %v, want ErrInvalidOptions", err)
			}
		})
	}
}

// This is the trap the feature exists to close: a provider that ignores
// max_age and answers from its own single sign-on session returns a valid
// token with no auth_time. Accepting it would report a re-authentication that
// never happened.
func TestCallbackRejectsAMissingAuthTimeWhenRequired(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	maxAge := 5 * time.Minute
	if _, _, err := fixture.run(t, BeginOptions{MaxAge: &maxAge}, CallbackOptions{RequireAuthTime: true}); !errors.Is(err, ErrAuthTime) {
		t.Fatalf("err = %v, want ErrAuthTime", err)
	}
}

func TestCallbackAcceptsAMissingAuthTimeWhenNotRequired(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	_, token, err := fixture.run(t, BeginOptions{}, CallbackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if token.AuthTime != nil {
		t.Fatalf("AuthTime = %v, want nil", token.AuthTime)
	}
}

func TestAuthTimeIsExposedAsAVerifiedValue(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	proved := fixture.now.Add(-90 * time.Second)
	fixture.extra = map[string]any{"auth_time": proved.Unix(), "acr": "urn:example:loa2"}
	_, token, err := fixture.run(t, BeginOptions{}, CallbackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if token.AuthTime == nil || !token.AuthTime.Equal(proved) {
		t.Fatalf("AuthTime = %v, want %v", token.AuthTime, proved)
	}
	if token.ACR != "urn:example:loa2" {
		t.Fatalf("ACR = %q", token.ACR)
	}
}

// The arrival time is not the proof time: a provider may satisfy the request
// from a session established long before, and freshness must be measured from
// what it reported rather than from when the answer landed.
func TestAuthTimeIsIndependentOfIssuedAt(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	proved := fixture.now.Add(-8 * time.Hour)
	fixture.extra = map[string]any{"auth_time": proved.Unix()}
	_, token, err := fixture.run(t, BeginOptions{}, CallbackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if token.AuthTime == nil || !token.AuthTime.Equal(proved) {
		t.Fatalf("AuthTime = %v, want the reported %v rather than the arrival time", token.AuthTime, proved)
	}
}

func TestMalformedAuthTimeIsRejectedRatherThanDropped(t *testing.T) {
	cases := map[string]any{
		"quoted number": "1700000000",
		"fractional":    1.5,
		"negative":      -1,
		"boolean":       true,
		"future":        float64(time.Unix(1_700_000_000, 0).Add(time.Hour).Unix()),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newAuthAgeFixture(t)
			fixture.extra = map[string]any{"auth_time": value}
			if _, _, err := fixture.run(t, BeginOptions{}, CallbackOptions{}); !errors.Is(err, ErrAuthTime) {
				t.Fatalf("err = %v, want ErrAuthTime", err)
			}
		})
	}
}

func TestNonStringACRIsRejected(t *testing.T) {
	fixture := newAuthAgeFixture(t)
	fixture.extra = map[string]any{"acr": 2}
	if _, _, err := fixture.run(t, BeginOptions{}, CallbackOptions{}); !errors.Is(err, ErrIDToken) {
		t.Fatalf("err = %v, want ErrIDToken", err)
	}
}

func idTokenWith(t *testing.T, key *rsa.PrivateKey, issuer, nonce string, now time.Time, extra map[string]any) string {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "key-1"})
	body := map[string]any{"iss": issuer, "sub": "user", "aud": "client", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": nonce}
	for name, value := range extra {
		body[name] = value
	}
	claims, _ := json.Marshal(body)
	input := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + encode(signature)
}
