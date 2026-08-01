package oidc

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
	"github.com/shibukawa/popcornwave/authstate/memory"
	"github.com/shibukawa/popcornwave/contrib/jwt"
	"github.com/shibukawa/popcornwave/contrib/oauth"
)

type oidcTransport struct{ handler http.Handler }

func (t oidcTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	response := recorder.Result()
	response.Body = io.NopCloser(bytes.NewReader(recorder.Body.Bytes()))
	return response, nil
}

type blockingHTTPTransport struct{}

func (blockingHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

type serializedRefreshTransport struct {
	once      sync.Once
	entered   chan struct{}
	release   chan struct{}
	active    atomic.Int32
	maxActive atomic.Int32
}

func (t *serializedRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	active := t.active.Add(1)
	for {
		current := t.maxActive.Load()
		if active <= current || t.maxActive.CompareAndSwap(current, active) {
			break
		}
	}
	t.once.Do(func() { close(t.entered) })
	<-t.release
	t.active.Add(-1)
	body := `{"keys":[]}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

type mutatingTransactionStore struct {
	inner      authstate.Store[oauth.Transaction]
	stripNonce bool
}

func (s *mutatingTransactionStore) Put(ctx context.Context, key string, value oauth.Transaction, expiresAt time.Time) error {
	if s.stripNonce {
		value.Nonce = ""
	}
	return s.inner.Put(ctx, key, value, expiresAt)
}

func (s *mutatingTransactionStore) Take(ctx context.Context, key string) (oauth.Transaction, error) {
	return s.inner.Take(ctx, key)
}

func TestDiscoveryAuthorizationAndNonceBoundIDToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	issuer := "https://issuer.example"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var expectedNonce string
	tokenRequests := 0
	userinfoBody := `{"sub":"user","name":"Example"}`
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/authorize","token_endpoint":"https://issuer.example/token","jwks_uri":"https://issuer.example/keys","userinfo_endpoint":"https://issuer.example/userinfo"}`)
		case "/keys":
			_, _ = io.WriteString(w, jwksJSON(key))
		case "/token":
			tokenRequests++
			_, _ = io.WriteString(w, `{"access_token":"access","token_type":"Bearer","id_token":"`+signedIDToken(t, key, issuer, expectedNonce, now)+`"}`)
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, userinfoBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	provider, err := Discover(context.Background(), issuer, DiscoverOptions{HTTPClient: &http.Client{Transport: transport}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if provider.options.requestTimeout != defaultRequestTimeout {
		t.Fatalf("default discovery timeout = %s, want %s", provider.options.requestTimeout, defaultRequestTimeout)
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, Options{OAuth: oauth.Options{StateStore: store, HTTPClient: &http.Client{Transport: transport}}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	authURL, transactionKey, err := client.BeginAuthorization(context.Background(), BeginOptions{Scopes: []string{"profile"}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	expectedNonce = parsed.Query().Get("nonce")
	if parsed.Query().Get("scope") != "openid profile" || expectedNonce == "" {
		t.Fatalf("authorization query = %v", parsed.Query())
	}
	set, _, err := client.HandleCallback(context.Background(), transactionKey, Callback{State: parsed.Query().Get("state"), Code: "code"}, CallbackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var rawIDToken string
	if err := json.Unmarshal(set.Raw["id_token"], &rawIDToken); err != nil {
		t.Fatal(err)
	}
	if _, err := client.VerifyIDTokenWithNonce(context.Background(), rawIDToken, expectedNonce); err != nil {
		t.Fatalf("standalone nonce-bound verification = %v", err)
	}
	if _, err := client.VerifyIDTokenWithNonce(context.Background(), rawIDToken, expectedNonce+"wrong"); !errors.Is(err, ErrNonce) {
		t.Fatalf("standalone nonce mismatch = %v", err)
	}
	if set.AccessToken != "access" {
		t.Fatalf("token set = %#v", set)
	}
	info, err := client.UserInfo(context.Background(), set.AccessToken)
	if err != nil || string(info["sub"]) != `"user"` {
		t.Fatalf("userinfo = %#v err=%v", info, err)
	}
	for _, invalidToken := range []string{"access token", "access\ttoken", "access\r\ntoken", "access\nX-Injected: yes"} {
		if _, err := client.UserInfo(context.Background(), invalidToken); !errors.Is(err, ErrUserInfo) {
			t.Fatalf("invalid bearer token %q error = %v", invalidToken, err)
		}
	}
	if _, err := client.UserInfoWithSubject(context.Background(), set.AccessToken, "user"); err != nil {
		t.Fatalf("subject-bound userinfo = %v", err)
	}
	if _, err := client.UserInfoWithSubject(context.Background(), set.AccessToken, "other-user"); !errors.Is(err, ErrUserInfo) {
		t.Fatalf("subject mismatch userinfo = %v", err)
	}
	if _, err := client.UserInfoWithSubject(context.Background(), set.AccessToken, ""); !errors.Is(err, ErrUserInfo) {
		t.Fatalf("empty expected subject userinfo = %v", err)
	}
	userinfoBody = `{"sub":"user","sub":"attacker"}`
	if _, err := client.UserInfo(context.Background(), set.AccessToken); !errors.Is(err, ErrUserInfo) {
		t.Fatalf("duplicate UserInfo = %v", err)
	}
	userinfoBody = `{"name":"missing-sub"}`
	if _, err := client.UserInfo(context.Background(), set.AccessToken); !errors.Is(err, ErrUserInfo) {
		t.Fatalf("missing UserInfo sub = %v", err)
	}
	badInner, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	badClient, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, Options{
		OAuth: oauth.Options{StateStore: &mutatingTransactionStore{inner: badInner, stripNonce: true}, HTTPClient: &http.Client{Transport: transport}}, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	badURL, badKey, err := badClient.BeginAuthorization(context.Background(), BeginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	badParsed, _ := url.Parse(badURL)
	if _, _, err := badClient.HandleCallback(context.Background(), badKey, Callback{State: badParsed.Query().Get("state"), Code: "code"}, CallbackOptions{}); !errors.Is(err, oauth.ErrState) {
		t.Fatalf("stripped nonce = %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("malformed nonce reached token endpoint: requests=%d", tokenRequests)
	}
}

func TestOIDCClientRejectsUnboundedTokenOptions(t *testing.T) {
	provider := &Provider{authorizationEndpoint: "https://issuer.example/a", tokenEndpoint: "https://issuer.example/t", options: providerOptions{}}
	for _, options := range []Options{{MaxTokenBytes: maxMaxTokenBytes + 1}, {MaxSegmentBytes: maxMaxSegmentBytes + 1}} {
		if _, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, options); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("options %#v error = %v", options, err)
		}
	}
}

func TestOIDCPublicClientWithoutSecretIsUnsupported(t *testing.T) {
	provider := &Provider{
		authorizationEndpoint: "https://issuer.example/a",
		tokenEndpoint:         "https://issuer.example/t",
		options:               providerOptions{},
	}
	_, err := NewClient(provider, Config{ClientID: "public", RedirectURI: "https://app.example/callback"}, Options{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("public client error = %v", err)
	}
}

func TestDiscoveryEndpointValidatorCoversAllProviderEndpoints(t *testing.T) {
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/a","token_endpoint":"https://issuer.example/t","jwks_uri":"https://issuer.example/k","userinfo_endpoint":"https://issuer.example/u"}`)
			return
		}
		_, _ = io.WriteString(w, `{"keys":[]}`)
	})}
	var paths []string
	provider, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient: &http.Client{Transport: transport},
		EndpointValidator: func(endpoint *url.URL) error {
			paths = append(paths, endpoint.Path)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil || len(paths) != 5 {
		t.Fatalf("validated endpoint paths = %v, want issuer plus four endpoints", paths)
	}
	for _, path := range []string{"", "/a", "/t", "/k", "/u"} {
		found := false
		for _, got := range paths {
			if got == path {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("endpoint %q was not validated: %v", path, paths)
		}
	}
}

func TestDiscoveryEndpointValidatorErrorIsSanitized(t *testing.T) {
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/a","token_endpoint":"https://issuer.example/t","jwks_uri":"https://issuer.example/k"}`)
	})}
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient: &http.Client{Transport: transport},
		EndpointValidator: func(endpoint *url.URL) error {
			if endpoint.Path == "/k" {
				return errors.New("trust secret=must-not-escape")
			}
			return nil
		},
	})
	if !errors.Is(err, ErrDiscovery) || strings.Contains(err.Error(), "must-not-escape") {
		t.Fatalf("validator error = %v", err)
	}
}

func TestDiscoveryEndpointValidatorCannotRewriteEndpoints(t *testing.T) {
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/a","token_endpoint":"https://issuer.example/t","jwks_uri":"https://issuer.example/k","userinfo_endpoint":"https://issuer.example/u"}`)
			return
		}
		_, _ = io.WriteString(w, `{"keys":[]}`)
	})}
	provider, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient: &http.Client{Transport: transport},
		EndpointValidator: func(endpoint *url.URL) error {
			endpoint.Scheme = "http"
			endpoint.Host = "attacker.example"
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.authorizationEndpoint != "https://issuer.example/a" || provider.tokenEndpoint != "https://issuer.example/t" || provider.jwksURI != "https://issuer.example/k" || provider.userInfoEndpoint != "https://issuer.example/u" {
		t.Fatalf("validator rewrote provider endpoints: auth=%q token=%q jwks=%q userinfo=%q", provider.authorizationEndpoint, provider.tokenEndpoint, provider.jwksURI, provider.userInfoEndpoint)
	}
}

func TestOIDCRejectsNonBearerTokenType(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var expectedNonce string
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/a","token_endpoint":"https://issuer.example/t","jwks_uri":"https://issuer.example/k"}`)
		case "/k":
			_, _ = io.WriteString(w, jwksJSON(key))
		case "/t":
			_, _ = io.WriteString(w, `{"access_token":"access","token_type":"MAC","id_token":"`+signedIDToken(t, key, "https://issuer.example", expectedNonce, now)+`"}`)
		}
	})}
	provider, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{HTTPClient: &http.Client{Transport: transport}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, Options{OAuth: oauth.Options{StateStore: store, HTTPClient: &http.Client{Transport: transport}}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	urlValue, keyValue, err := client.BeginAuthorization(context.Background(), BeginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(urlValue)
	if err != nil {
		t.Fatal(err)
	}
	expectedNonce = parsed.Query().Get("nonce")
	if _, _, err := client.HandleCallback(context.Background(), keyValue, Callback{State: parsed.Query().Get("state"), Code: "code"}, CallbackOptions{}); !errors.Is(err, ErrIDToken) {
		t.Fatalf("non-Bearer token type error = %v", err)
	}
}

func TestNonceMismatchAndDiscoveryIssuerValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = io.WriteString(w, `{"issuer":"https://other.example","authorization_endpoint":"https://issuer.example/a","token_endpoint":"https://issuer.example/t","jwks_uri":"https://issuer.example/k"}`)
			return
		}
		_, _ = io.WriteString(w, jwksJSON(key))
	})}
	if _, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{HTTPClient: &http.Client{Transport: transport}}); !errors.Is(err, ErrDiscovery) {
		t.Fatalf("issuer error = %v", err)
	}
}

func TestDiscoveryRejectsJWKSBounds(t *testing.T) {
	ctx := context.Background()
	for _, options := range []DiscoverOptions{
		{JWKSMaxBytes: -1},
		{JWKSMaxBytes: maxJWKSBytes + 1},
		{JWKSMaxKeys: -1},
		{JWKSMaxKeys: maxJWKSKeys + 1},
	} {
		if _, err := Discover(ctx, "https://issuer.example", options); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("options %+v error = %v", options, err)
		}
	}
}

// TestDiscoveryAcceptsPublishedCapabilityArrays covers a document shaped like
// a real provider's: few top-level members, but long capability arrays whose
// elements also count toward the JSON member bound.
func TestDiscoveryAcceptsPublishedCapabilityArrays(t *testing.T) {
	issuer := "https://issuer.example"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	claims := make([]string, 0, 24)
	for index := range 24 {
		claims = append(claims, `"claim-`+strconv.Itoa(index)+`"`)
	}
	document := `{"issuer":"` + issuer + `","authorization_endpoint":"` + issuer + `/authorize",` +
		`"token_endpoint":"` + issuer + `/token","jwks_uri":"` + issuer + `/keys",` +
		`"userinfo_endpoint":"` + issuer + `/userinfo",` +
		`"claims_supported":[` + strings.Join(claims, ",") + `],` +
		`"scopes_supported":["openid","profile","email","offline_access"],` +
		`"response_types_supported":["code","id_token","code id_token"],` +
		`"grant_types_supported":["authorization_code","refresh_token","client_credentials"],` +
		`"token_endpoint_auth_methods_supported":["client_secret_basic","client_secret_post","none"],` +
		`"id_token_signing_alg_values_supported":["RS256"],` +
		`"subject_types_supported":["public"]}`
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, document)
		case "/keys":
			_, _ = io.WriteString(w, jwksJSON(key))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	if _, err := Discover(context.Background(), issuer, DiscoverOptions{
		HTTPClient: &http.Client{Transport: transport},
	}); err != nil {
		t.Fatalf("discovery of a capability-rich document = %v", err)
	}
}

func TestDiscoveryRequestTimeoutCancelsHTTP(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient:     &http.Client{Transport: blockingHTTPTransport{}},
		RequestTimeout: time.Millisecond,
	})
	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("discovery timeout error = %v", err)
	}
}

func TestParseCacheMaxAge(t *testing.T) {
	tests := []struct {
		header string
		want   int64
		ok     bool
	}{
		{header: "public, max-age=60", want: 60, ok: true},
		{header: "MAX-AGE=0", want: 0, ok: true},
		{header: `max-age="120"`, want: 120, ok: true},
		{header: "no-cache", want: 0, ok: true},
		{header: "no-store", want: 0, ok: true},
		{header: "no-cache, no-store", want: 0, ok: true},
		{header: "max-age=-1", ok: false},
		{header: "max-age=not-a-number", ok: false},
	}
	for _, test := range tests {
		got, ok := parseCacheMaxAge(test.header)
		if got != test.want || ok != test.ok {
			t.Errorf("parseCacheMaxAge(%q) = %d, %v; want %d, %v", test.header, got, ok, test.want, test.ok)
		}
	}
}

func TestJWKSNoStoreExpiresStaleRetention(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, jwksJSON(key))
	})}
	provider := &Provider{jwksURI: "https://issuer.example/keys", options: providerOptions{
		httpClient: &http.Client{Transport: transport}, clock: func() time.Time { return now },
		maxResponseBytes: defaultMaxResponseBytes, cacheTTL: time.Minute, staleTTL: time.Hour,
	}}
	if err := provider.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !provider.staleExpiresAt.Equal(now) {
		t.Fatalf("stale expiry = %s, want %s", provider.staleExpiresAt, now)
	}
	header := jwt.Header{Algorithm: "RS256", KeyID: "key-1"}
	if _, err := provider.resolveKey(context.Background(), header); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.resolveKey(context.Background(), header); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("no-store JWKS requests = %d, want 3", requests)
	}
}

func TestJWKSCacheHonorsServerMaxAgeWithinConfiguredBound(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=1")
		_, _ = io.WriteString(w, jwksJSON(key))
	})}
	provider := &Provider{jwksURI: "https://issuer.example/keys", options: providerOptions{
		httpClient: &http.Client{Transport: transport}, clock: func() time.Time { return now },
		maxResponseBytes: defaultMaxResponseBytes, cacheTTL: 5 * time.Minute, staleTTL: 15 * time.Minute,
	}}
	if err := provider.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := now.Add(time.Second); !provider.cacheExpiresAt.Equal(want) {
		t.Fatalf("cache expiry = %s, want %s", provider.cacheExpiresAt, want)
	}
}

func TestUserInfoRequestTimeoutCancelsHTTP(t *testing.T) {
	provider := &Provider{userInfoEndpoint: "https://issuer.example/userinfo", options: providerOptions{
		httpClient:       &http.Client{Transport: blockingHTTPTransport{}},
		maxResponseBytes: defaultMaxResponseBytes, requestTimeout: time.Millisecond,
	}}
	client := &Client{provider: provider}
	if _, err := client.UserInfo(context.Background(), "access-token"); !errors.Is(err, ErrHTTP) {
		t.Fatalf("UserInfo timeout error = %v", err)
	}
}

func TestSecondIndependentProviderFixture(t *testing.T) {
	now := time.Unix(1_700_000_100, 0)
	issuer := "https://second-issuer.example"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var nonce string
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, `{"issuer":"https://second-issuer.example","authorization_endpoint":"https://second-issuer.example/a","token_endpoint":"https://second-issuer.example/t","jwks_uri":"https://second-issuer.example/k"}`)
		case "/k":
			_, _ = io.WriteString(w, jwksJSON(key))
		case "/t":
			_, _ = io.WriteString(w, `{"access_token":"second-access","token_type":"Bearer","id_token":"`+signedIDToken(t, key, issuer, nonce, now)+`"}`)
		}
	})}
	provider, err := Discover(context.Background(), issuer, DiscoverOptions{HTTPClient: &http.Client{Transport: transport}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, Options{OAuth: oauth.Options{StateStore: store, HTTPClient: &http.Client{Transport: transport}}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	authURL, keyValue, err := client.BeginAuthorization(context.Background(), BeginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	nonce = parsed.Query().Get("nonce")
	if _, _, err := client.HandleCallback(context.Background(), keyValue, Callback{State: parsed.Query().Get("state"), Code: "code"}, CallbackOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownKeyRefreshesOnce(t *testing.T) {
	now := time.Unix(1_700_000_200, 0)
	issuer := "https://rotating-issuer.example"
	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyRequests := 0
	var nonce string
	transport := oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = io.WriteString(w, `{"issuer":"https://rotating-issuer.example","authorization_endpoint":"https://rotating-issuer.example/a","token_endpoint":"https://rotating-issuer.example/t","jwks_uri":"https://rotating-issuer.example/k"}`)
		case "/k":
			keyRequests++
			if keyRequests == 1 {
				_, _ = io.WriteString(w, jwksJSONNamed(oldKey, "old"))
			} else {
				_, _ = io.WriteString(w, jwksJSONNamed(newKey, "new"))
			}
		case "/t":
			_, _ = io.WriteString(w, `{"access_token":"rotated","token_type":"Bearer","id_token":"`+signedIDTokenNamed(t, newKey, "new", issuer, nonce, now)+`"}`)
		}
	})}
	provider, err := Discover(context.Background(), issuer, DiscoverOptions{HTTPClient: &http.Client{Transport: transport}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(provider, Config{ClientID: "client", ClientSecret: "secret", RedirectURI: "https://app.example/callback"}, Options{OAuth: oauth.Options{StateStore: store, HTTPClient: &http.Client{Transport: transport}}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	authURL, keyValue, err := client.BeginAuthorization(context.Background(), BeginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	nonce = parsed.Query().Get("nonce")
	if _, _, err := client.HandleCallback(context.Background(), keyValue, Callback{State: parsed.Query().Get("state"), Code: "code"}, CallbackOptions{}); err != nil {
		t.Fatal(err)
	}
	if keyRequests != 2 {
		t.Fatalf("JWKS requests = %d", keyRequests)
	}
}

func TestProviderRefreshRejectsNilInputs(t *testing.T) {
	var provider *Provider
	if err := provider.refresh(context.Background()); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil provider refresh = %v", err)
	}
	provider = &Provider{}
	if err := provider.refresh(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil context refresh = %v", err)
	}
}

func TestProviderRefreshSerializesConcurrentRequests(t *testing.T) {
	transport := &serializedRefreshTransport{entered: make(chan struct{}), release: make(chan struct{})}
	provider := &Provider{jwksURI: "https://issuer.example/keys", options: providerOptions{
		httpClient: &http.Client{Transport: transport}, clock: time.Now,
		requestTimeout: time.Second, maxResponseBytes: 64 << 10,
		jwksMaxBytes: 16 << 20, jwksMaxKeys: 4096,
		cacheTTL: time.Minute, staleTTL: time.Minute,
	}}
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			if err := provider.refresh(context.Background()); err != nil {
				t.Errorf("refresh error = %v", err)
			}
		}()
	}
	select {
	case <-transport.entered:
	case <-time.After(time.Second):
		t.Fatal("refresh did not reach transport")
	}
	close(transport.release)
	wait.Wait()
	if got := transport.maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent refresh requests = %d, want 1", got)
	}
}

func TestAuthorizedPartyRejectsMalformedPresentClaim(t *testing.T) {
	claims := jwt.Claims{Audience: []string{"client"}, Raw: map[string]json.RawMessage{
		"azp": json.RawMessage(`123`),
	}}
	if err := validateAuthorizedParty(claims, "client"); !errors.Is(err, ErrIDToken) {
		t.Fatalf("malformed azp error = %v", err)
	}
	claims.Raw["azp"] = json.RawMessage(`"other"`)
	if err := validateAuthorizedParty(claims, "client"); !errors.Is(err, ErrIDToken) {
		t.Fatalf("mismatched azp error = %v", err)
	}
	claims = jwt.Claims{Audience: []string{"client", "other"}, Raw: map[string]json.RawMessage{}}
	if err := validateAuthorizedParty(claims, "client"); !errors.Is(err, ErrIDToken) {
		t.Fatalf("missing azp for multiple audiences error = %v", err)
	}
	claims.Raw["azp"] = json.RawMessage(`"client"`)
	if err := validateAuthorizedParty(claims, "client"); err != nil {
		t.Fatalf("valid azp for multiple audiences error = %v", err)
	}
}

func jwksJSON(key *rsa.PrivateKey) string {
	return jwksJSONNamed(key, "key-1")
}

func jwksJSONNamed(key *rsa.PrivateKey, keyID string) string {
	encode := base64.RawURLEncoding.EncodeToString
	return `{"keys":[{"kty":"RSA","kid":"` + keyID + `","use":"sig","alg":"RS256","n":"` + encode(key.N.Bytes()) + `","e":"` + encode(big.NewInt(int64(key.PublicKey.E)).Bytes()) + `"}]}`
}

func signedIDToken(t *testing.T, key *rsa.PrivateKey, issuer, nonce string, now time.Time) string {
	return signedIDTokenNamed(t, key, "key-1", issuer, nonce, now)
}

func signedIDTokenNamed(t *testing.T, key *rsa.PrivateKey, keyID, issuer, nonce string, now time.Time) string {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	claims, _ := json.Marshal(map[string]any{"iss": issuer, "sub": "user", "aud": "client", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": nonce})
	input := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + encode(signature)
}
