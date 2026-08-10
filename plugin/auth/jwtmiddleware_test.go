package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/pw"
	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

// bearerRuntime assembles the runtime setupBearer would build, without the
// framework initialization a full application boot needs.
func bearerRuntime(t *testing.T, issuer *testIssuer, adjust func(*Config)) *runtime {
	t.Helper()
	config := validJWTConfig()
	config.JWT.Issuer = issuer.server.URL
	config.JWT.AllowLoopbackHTTP = true
	if adjust != nil {
		adjust(&config)
	}
	if err := config.validate(); err != nil {
		t.Fatalf("test configuration is invalid: %v", err)
	}
	verifier, err := newBearerVerifier(config.JWT)
	if err != nil {
		t.Fatal(err)
	}
	include, err := pathpattern.Compile(config.Protection.Include)
	if err != nil {
		t.Fatal(err)
	}
	instance := &runtime{config: config, include: include, bearer: verifier}
	if config.JWT.Revocation.enabled() {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "auth.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, err := db.Exec(revocationSchemaSQL("sqlite")); err != nil {
			t.Fatal(err)
		}
		instance.revocations = newRevocationStore(db, "sqlite", config.JWT)
	}
	return instance
}

// serve runs a request through the bearer middleware and the guard, which is
// the order an application sees them in.
func serve(rt *runtime, request *http.Request) *httptest.ResponseRecorder {
	reached := false
	handler := httpFrame(rt.serveBearer)(httpFrame(rt.guard)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			reached = true
			if identity, ok := Bearer(r.Context()); ok {
				w.Header().Set("X-Test-Account", identity.AccountID)
			}
			w.WriteHeader(http.StatusOK)
		})))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if reached {
		recorder.Header().Set("X-Test-Reached", "yes")
	}
	return recorder
}

func bearerRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func TestBearerRequestReachesTheHandler(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())

	response := serve(rt, bearerRequest(token))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if response.Header().Get("X-Test-Account") == "" {
		t.Fatal("the handler saw no bearer identity")
	}
}

// A protected path with no credential is a 401 that names the scheme, not a
// redirect to a login page an API client cannot render.
func TestProtectedPathWithoutACredentialAnswers401(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)

	response := serve(rt, bearerRequest(""))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if response.Header().Get("X-Test-Reached") != "" {
		t.Fatal("an unauthenticated request reached the handler")
	}
	if challenge := response.Header().Get("WWW-Authenticate"); !strings.HasPrefix(challenge, "Bearer ") {
		t.Fatalf("WWW-Authenticate = %q, want a Bearer challenge", challenge)
	}
}

// An unprotected path is served anonymously: no credential is not a failure.
func TestUnprotectedPathServesAnonymously(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := serve(rt, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Test-Reached") == "" {
		t.Fatalf("an unprotected path was refused: status = %d", response.Code)
	}
}

// Every rejection is the same 401. The difference between "wrong audience" and
// "expired" is an oracle for probing what this deployment trusts.
func TestEveryRejectionLooksTheSame(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)
	now := time.Now().Unix()

	claimSets := map[string]map[string]any{
		"foreign audience": {"iss": issuer.server.URL, "sub": "c", "aud": []string{"https://other.example"}, "exp": now + 300, "iat": now, "jti": "t"},
		"foreign issuer":   {"iss": "https://attacker.example", "sub": "c", "aud": []string{"https://api.example"}, "exp": now + 300, "iat": now, "jti": "t"},
		"expired":          {"iss": issuer.server.URL, "sub": "c", "aud": []string{"https://api.example"}, "exp": now - 300, "iat": now - 600, "jti": "t"},
		"no subject":       {"iss": issuer.server.URL, "aud": []string{"https://api.example"}, "exp": now + 300, "iat": now, "jti": "t"},
	}

	var bodies []string
	for name, claims := range claimSets {
		response := serve(rt, bearerRequest(issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, response.Code)
		}
		if response.Header().Get("X-Test-Reached") != "" {
			t.Fatalf("%s reached the handler", name)
		}
		bodies = append(bodies, response.Body.String())
	}
	for _, body := range bodies[1:] {
		if body != bodies[0] {
			t.Fatal("rejection bodies differ, which tells a caller which check refused it")
		}
	}
}

// The revoked token is refused at the middleware, after verification proved the
// issuer minted it.
func TestRevokedTokenIsRefusedByTheMiddleware(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())

	if response := serve(rt, bearerRequest(token)); response.Code != http.StatusOK {
		t.Fatalf("the token was refused before it was revoked: status = %d", response.Code)
	}
	if err := rt.RevokeToken(t.Context(), issuer.server.URL, "token-1", "leaked"); err != nil {
		t.Fatal(err)
	}
	response := serve(rt, bearerRequest(token))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("a revoked token was still served: status = %d", response.Code)
	}
}

// Admission runs on a verified identity, so an organization rule refuses a
// caller the issuer vouched for.
func TestClaimAdmissionRefusesAVerifiedOutsider(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, func(c *Config) {
		c.JWT.Admission = AdmissionClaim
		c.JWT.Claim = ClaimConfig{Path: "/groups", Values: []string{"engineering"}, Match: MatchAny}
	})

	outsider := issuer.standardClaims()
	outsider["groups"] = []string{"marketing"}
	if response := serve(rt, bearerRequest(issuer.mint(t, map[string]any{"typ": "at+jwt"}, outsider))); response.Code != http.StatusUnauthorized {
		t.Fatalf("a verified caller outside the configured group was admitted: status = %d", response.Code)
	}

	member := issuer.standardClaims()
	member["groups"] = []string{"engineering", "marketing"}
	if response := serve(rt, bearerRequest(issuer.mint(t, map[string]any{"typ": "at+jwt"}, member))); response.Code != http.StatusOK {
		t.Fatalf("a group member was refused: status = %d", response.Code)
	}
}

// A bearer request establishes no session, so nothing is set on the client that
// would let the next request skip verification.
func TestBearerRequestSetsNoCookie(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())

	response := serve(rt, bearerRequest(token))
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("a bearer request set %d cookies", len(cookies))
	}
}

// The published identity carries no token body, so a handler cannot replay the
// caller's credential against another service.
func TestPublishedIdentityCarriesNoTokenBody(t *testing.T) {
	issuer := newTestIssuer(t)
	rt := bearerRuntime(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())

	var published BearerIdentity
	handler := httpFrame(rt.serveBearer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		published, _ = Bearer(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), bearerRequest(token))

	if published.AccountID == "" {
		t.Fatal("no identity was published")
	}
	if _, found := published.Identity.Claims.Raw("__raw_token"); found {
		t.Fatal("the raw token reached the handler")
	}
	if pw.RequestAuthentication(t.Context()).Authenticated {
		t.Fatal("an unrelated context reported an authenticated request")
	}
}

// Revoking through the package-level entry point without a running runtime must
// report that rather than silently doing nothing.
func TestRevokeWithoutARuntimeReportsIt(t *testing.T) {
	replaceRuntime(nil)
	err := RevokeToken(t.Context(), "https://issuer.example", "token-1", "")
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("err = %v, want an uninitialized report", err)
	}
	if errors.Is(err, ErrRevokedToken) {
		t.Fatal("a missing runtime was reported as a revocation")
	}
}
