package authfastjwte2e

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/auth/authfast"
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwfast"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

const audience = "https://api.example"

type deployment struct {
	base   string
	issuer *testIssuer
}

var (
	once    sync.Once
	shared  *deployment
	buildEr error
)

func start(t *testing.T) *deployment {
	t.Helper()
	once.Do(func() { shared, buildEr = build() })
	if buildEr != nil {
		t.Fatalf("deployment: %v", buildEr)
	}
	return shared
}

func build() (*deployment, error) {
	directory, err := os.MkdirTemp("", "authfastjwte2e")
	if err != nil {
		return nil, err
	}
	issuer, err := newTestIssuer()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	configPath, err := writeConfig(directory, issuer.server.URL)
	if err != nil {
		return nil, err
	}
	pwconfig.SetLoadOptions(configbind.LoadOptions{
		Vendor:             "popcornwave-authfastjwt-e2e",
		Tool:               "authfastjwt-e2e",
		ExplicitConfigPath: configPath,
		Args:               []string{},
		Environ:            []string{"APP_ENV=dev"},
	})

	// Startup, without the net/http runtime. This binary links none of it: the
	// settings come from pwconfig, the bearer runtime from plugin/auth, and the
	// chain from pwfast — which is the claim the whole layer move was for, run
	// rather than asserted. TestTheBinaryLinksNoNetHTTPRuntime checks the graph.
	if err := pwconfig.Parse(); err != nil {
		return nil, fmt.Errorf("configuration: %w", err)
	}
	ctx := pwruntime.WithResources(context.Background(),
		pwruntime.Resources{Configs: pwconfig.Snapshot()})
	options, err := authfast.Setup(ctx)
	if err != nil {
		return nil, fmt.Errorf("authentication: %w", err)
	}
	handler, err := pwfast.Middlewares(application(), options.Apply(pwfast.RuntimeOptions{}))
	if err != nil {
		return nil, fmt.Errorf("fasthttp chain: %w", err)
	}
	go func() { _ = fasthttp.Serve(listener, handler) }()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, err
	}
	return &deployment{base: "http://127.0.0.1:" + port, issuer: issuer}, nil
}

func application() fasthttp.RequestHandler {
	mux := pwfast.NewServeMux()
	// An open route, so a test can prove the frame lets an anonymous request
	// through rather than refusing everything.
	mux.HandleFunc("GET /open", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("open:" + subject(r))
	})
	mux.HandleFunc("GET /api/thing", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("thing:" + subject(r))
	})
	return mux.Handler
}

// subject reports what the bearer frame recorded, which is what a guard and an
// application both read.
func subject(r *fasthttp.RequestCtx) string {
	authentication := pwfast.RequestAuthentication(r)
	if !authentication.Authenticated {
		return "anonymous"
	}
	identity, ok := auth.Bearer(r)
	if !ok {
		// The frame recorded an authentication with no bearer identity behind
		// it, which would mean the principal did not survive the request value.
		return "unreadable"
	}
	return authentication.Method + ":" + identity.Identity.Key
}

func call(t *testing.T, path string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	deployment := start(t)
	request, err := http.NewRequest(http.MethodGet, deployment.base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range headers {
		request.Header.Add(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := make([]byte, 4096)
	read, _ := response.Body.Read(body)
	return response, string(body[:read])
}

func bearer(t *testing.T, adjust func(map[string]any)) string {
	t.Helper()
	claims := start(t).issuer.claims()
	if adjust != nil {
		adjust(claims)
	}
	token, err := start(t).issuer.mint(nil, claims)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + token
}

// testIssuer is a loopback authorization server: it publishes metadata and a
// key set, and mints the tokens these tests present.
type testIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	keyID  string
}

func newTestIssuer() (*testIssuer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	issuer := &testIssuer{key: key, keyID: "e2e-key"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuer.server.URL,
			"jwks_uri": issuer.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kid": issuer.keyID, "kty": "RSA", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}},
		})
	})
	issuer.server = httptest.NewServer(mux)
	return issuer, nil
}

// mint assembles a compact RS256 token. contrib/jwt publishes no RSA signer, so
// the compact form is built here.
func (i *testIssuer) mint(header map[string]any, claims map[string]any) (string, error) {
	if header == nil {
		header = map[string]any{}
	}
	if _, ok := header["alg"]; !ok {
		header["alg"] = "RS256"
	}
	if _, ok := header["kid"]; !ok {
		header["kid"] = i.keyID
	}
	// typ is a header field rather than a claim, and this deployment requires
	// it: an access token and an ID token are different credentials, and one
	// presented as the other is what the check exists to refuse.
	if _, ok := header["typ"]; !ok {
		header["typ"] = "at+jwt"
	}
	encode := func(value any) (string, error) {
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(encoded), nil
	}
	head, err := encode(header)
	if err != nil {
		return "", err
	}
	payload, err := encode(claims)
	if err != nil {
		return "", err
	}
	signingInput := head + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// claims is a token that verifies. Each test changes one member.
func (i *testIssuer) claims() map[string]any {
	now := time.Now().Unix()
	return map[string]any{
		"iss": i.server.URL,
		"sub": "caller-1",
		"aud": []string{audience},
		"exp": now + 300,
		"iat": now,
		"jti": "token-1",
	}
}

func writeConfig(directory, issuer string) (string, error) {
	content := fmt.Sprintf(`
[server]
public.enabled = false

[security.csrf]
# jwt_only creates no session, so there is no secret a CSRF check could compare
# against; the framework refuses the pair at startup rather than at the request.
enabled = false

[session]
enabled = false

[auth]
enabled = true
mode = "jwt_only"
protection.include = ["/api/*"]
protection.unauthenticated = "unauthorized"

[auth.jwt]
issuer = "%s"
audience = ["%s"]
algorithms = ["RS256"]
max_token_lifetime = "1h"
allow_loopback_http = true
admission = "authenticated"
auto_provision = true
revocation.mode = "off"
`, issuer, audience)
	path := filepath.Join(directory, "config.toml")
	return path, os.WriteFile(path, []byte(content), 0o600)
}
