//go:build pwdev

package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
)

func TestDevBuildCarriesTheRelaxation(t *testing.T) {
	if !devRelaxationBuilt {
		t.Fatal("a pwdev build does not report the relaxation as present")
	}
}

// unsignedToken is what a developer with a text editor produces: a decodable
// claim set behind a header nobody signed.
func unsignedToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	encode := func(value any) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(encoded)
	}
	return encode(map[string]any{"alg": "none"}) + "." + encode(claims) + "."
}

func devRequest(t *testing.T, token, remoteAddr string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	request.RemoteAddr = remoteAddr
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

// The whole point: no authorization server is running, and the token verifies
// against nothing.
func TestDevRelaxationAdmitsAnUnsignedTokenFromLoopback(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })
	token := unsignedToken(t, map[string]any{"sub": "developer"})

	identity, ok := verifier.devAdmits(devRequest(t, token, "127.0.0.1:54321"))
	if !ok {
		t.Fatal("an unsigned token was refused under the development relaxation")
	}
	if identity.Key != "developer" {
		t.Fatalf("identity = %#v", identity)
	}
	// A hand-written token carries no iat, and the subject form of revocation
	// compares against one, so it is stamped rather than left zero.
	if identity.IssuedAt.IsZero() {
		t.Fatal("a relaxed identity carries no issue time, so subject revocation could not compare")
	}
}

// The network lock has no opt-out. A device that needs a token has devidp,
// which signs.
func TestDevRelaxationRefusesANonLoopbackRequest(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })
	token := unsignedToken(t, map[string]any{"sub": "developer"})

	if _, ok := verifier.devAdmits(devRequest(t, token, "192.168.1.20:54321")); ok {
		t.Fatal("the relaxation was reachable from the network")
	}
}

// A forwarded header is a claim made by whoever sent the request. Believing it
// here would hand the relaxation to the network.
func TestDevRelaxationIgnoresForwardedHeaders(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })
	token := unsignedToken(t, map[string]any{"sub": "developer"})

	request := devRequest(t, token, "192.168.1.20:54321")
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	if _, ok := verifier.devAdmits(request); ok {
		t.Fatal("a forwarded header opened the loopback lock")
	}
}

// The configuration lock: APP_ENV=dev alone must not turn verification off,
// because the environment token is data rather than a feature switch.
func TestDevRelaxationRequiresTheConfigurationField(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	token := unsignedToken(t, map[string]any{"sub": "developer"})

	if _, ok := verifier.devAdmits(devRequest(t, token, "127.0.0.1:54321")); ok {
		t.Fatal("the relaxation ran without being configured")
	}
}

// The environment lock, matching policy:devidp-safety.
func TestDevRelaxationRefusesToStartOutsideDevelopment(t *testing.T) {
	config := validJWTConfig().JWT
	config.Dev.TrustUnverifiedTokens = true

	for _, environment := range []string{pw.EnvStaging, pw.EnvProduction, "production"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv(pw.EnvVar, environment)
			if err := checkDevRelaxation(config); err == nil {
				t.Fatalf("the relaxation started under %s=%s", pw.EnvVar, environment)
			}
		})
	}

	t.Run("dev", func(t *testing.T) {
		t.Setenv(pw.EnvVar, pw.EnvDevelopment)
		if err := checkDevRelaxation(config); err != nil {
			t.Fatalf("the relaxation was refused under development: %v", err)
		}
	})
}

// Relaxation removes checks; it does not add tolerance for malformed input.
func TestDevRelaxationStillRefusesGarbage(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })

	for name, token := range map[string]string{
		"not a jwt":   "hello",
		"bad base64":  "!!!.!!!.",
		"no identity": unsignedToken(t, map[string]any{"nothing": "here"}),
		"oversized":   "a." + strings.Repeat("x", 9000) + ".",
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := verifier.devAdmits(devRequest(t, token, "127.0.0.1:1")); ok {
				t.Fatalf("%s was admitted", name)
			}
		})
	}
}

// An expired hand-written token is admitted, because time is one of the checks
// the single switch turns off.
func TestDevRelaxationIgnoresExpiry(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })
	token := unsignedToken(t, map[string]any{
		"sub": "developer",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	if _, ok := verifier.devAdmits(devRequest(t, token, "127.0.0.1:1")); !ok {
		t.Fatal("an expired token was refused, but time is one of the relaxed checks")
	}
}

// The response says so, which is where someone notices that a deployment they
// believed was verifying is not.
func TestDevRelaxationMarksTheResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	markDevResponse(recorder)
	if recorder.Header().Get(DevUnverifiedHeader) == "" {
		t.Fatalf("a relaxed response carried no %s header", DevUnverifiedHeader)
	}
}

func TestMain(m *testing.M) {
	// The environment lock reads APP_ENV, and an inherited staging value would
	// otherwise make every relaxed case here fail for the wrong reason.
	_ = os.Setenv(pw.EnvVar, pw.EnvDevelopment)
	os.Exit(m.Run())
}
