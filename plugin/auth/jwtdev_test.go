//go:build !pwdev

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The build lock is the only structural one: in a production build the relaxed
// verifier is not code that declines to run, it is code that is not present.
func TestProductionBuildCarriesNoRelaxation(t *testing.T) {
	if devRelaxationBuilt {
		t.Fatal("a build without the pwdev tag reports the relaxation as present")
	}
}

// A security setting that is silently dropped reads as configured security, so
// a binary that cannot honor the field refuses to start rather than ignoring it.
func TestProductionBuildRefusesToStartWithTheRelaxationConfigured(t *testing.T) {
	config := validJWTConfig().JWT
	config.Dev.TrustUnverifiedTokens = true

	err := checkDevRelaxation(config)
	if err == nil {
		t.Fatal("a production binary accepted auth.jwt.dev.trust_unverified_tokens")
	}
	if !strings.Contains(err.Error(), "pwdev") {
		t.Fatalf("the error does not name the build mode that would honor it: %v", err)
	}
}

func TestProductionBuildStartsWithoutTheSetting(t *testing.T) {
	if err := checkDevRelaxation(validJWTConfig().JWT); err != nil {
		t.Fatalf("an ordinary configuration was refused: %v", err)
	}
}

// Even with the field set, this build has no path that admits an unverified
// token. The other locks are irrelevant here because the first one is shut.
func TestProductionBuildAdmitsNothingUnverified(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.Dev.TrustUnverifiedTokens = true })

	request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Authorization", "Bearer "+issuer.mint(t, nil, issuer.standardClaims()))

	if _, ok := verifier.devAdmits(HTTPExchange(httptest.NewRecorder(), request)); ok {
		t.Fatal("a production build admitted a token without verifying it")
	}
}
