package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testIssuer is a loopback authorization server: it publishes metadata and a
// key set, and mints tokens the tests then take apart.
type testIssuer struct {
	server  *httptest.Server
	key     *rsa.PrivateKey
	keyID   string
	fetches int
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := &testIssuer{key: key, keyID: "test-key"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   issuer.server.URL,
			"jwks_uri": issuer.server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		issuer.fetches++
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
	t.Cleanup(issuer.server.Close)
	return issuer
}

// mint builds a compact RS256 token from the given header and claim overrides.
// contrib/jwt publishes no RSA signer, so the compact form is assembled here.
func (i *testIssuer) mint(t *testing.T, header map[string]any, claims map[string]any) string {
	t.Helper()
	if header == nil {
		header = map[string]any{}
	}
	if _, ok := header["alg"]; !ok {
		header["alg"] = "RS256"
	}
	if _, ok := header["kid"]; !ok {
		header["kid"] = i.keyID
	}
	encode := func(value any) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(encoded)
	}
	signingInput := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// standardClaims is a token that verifies. Each test changes one member.
func (i *testIssuer) standardClaims() map[string]any {
	now := time.Now().Unix()
	return map[string]any{
		"iss": i.server.URL,
		"sub": "caller-1",
		"aud": []string{"https://api.example"},
		"exp": now + 300,
		"iat": now,
		"jti": "token-1",
	}
}

func testVerifier(t *testing.T, issuer *testIssuer, adjust func(*JWTConfig)) *bearerVerifier {
	t.Helper()
	config := validJWTConfig().JWT
	config.Issuer = issuer.server.URL
	config.AllowLoopbackHTTP = true
	if adjust != nil {
		adjust(&config)
	}
	verifier, err := newBearerVerifier(config)
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func TestBearerVerificationAcceptsAWellFormedToken(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())

	identity, err := verifier.verify(context.Background(), token)
	if err != nil {
		t.Fatalf("well-formed token rejected: %v", err)
	}
	if identity.Key != "caller-1" || identity.TokenID != "token-1" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.IssuedAt.IsZero() || identity.ExpiresAt.IsZero() {
		t.Fatalf("token times not published: %#v", identity)
	}
}

// Each of these is required by RFC 9068. A check that is applied only when the
// claim happens to be present is one some deployment finds switched off after
// an incident.
func TestBearerVerificationRequiresEveryMandatoryClaim(t *testing.T) {
	for _, claim := range []string{"iss", "sub", "aud", "exp", "iat", "jti"} {
		t.Run(claim, func(t *testing.T) {
			issuer := newTestIssuer(t)
			verifier := testVerifier(t, issuer, nil)
			claims := issuer.standardClaims()
			delete(claims, claim)
			token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)

			if _, err := verifier.verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("token missing %s was accepted: err = %v", claim, err)
			}
		})
	}
}

// An ID Token is signed by the same issuer with the same key. The token type is
// the field that says which one the issuer meant to mint.
func TestBearerVerificationRefusesAnIDTokenReplay(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	idToken := issuer.mint(t, map[string]any{"typ": "JWT"}, issuer.standardClaims())

	if _, err := verifier.verify(context.Background(), idToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("an ID Token was accepted as an access token: err = %v", err)
	}
}

// RFC 9068 permits both spellings, and media types are case-insensitive.
func TestBearerVerificationAcceptsBothTokenTypeSpellings(t *testing.T) {
	for _, spelling := range []string{"at+jwt", "AT+JWT", "application/at+jwt"} {
		t.Run(spelling, func(t *testing.T) {
			issuer := newTestIssuer(t)
			verifier := testVerifier(t, issuer, nil)
			token := issuer.mint(t, map[string]any{"typ": spelling}, issuer.standardClaims())

			if _, err := verifier.verify(context.Background(), token); err != nil {
				t.Fatalf("typ %q rejected: %v", spelling, err)
			}
		})
	}
}

func TestBearerVerificationRefusesAForeignAudience(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	claims := issuer.standardClaims()
	claims["aud"] = []string{"https://other.example"}
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)

	if _, err := verifier.verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a token minted for another service was accepted: err = %v", err)
	}
}

func TestBearerVerificationRefusesAForeignIssuer(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	claims := issuer.standardClaims()
	claims["iss"] = "https://attacker.example"
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)

	if _, err := verifier.verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a token claiming another issuer was accepted: err = %v", err)
	}
}

func TestBearerVerificationRefusesAnExpiredToken(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	claims := issuer.standardClaims()
	now := time.Now().Unix()
	claims["iat"] = now - 600
	claims["exp"] = now - 300
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)

	if _, err := verifier.verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("an expired token was accepted: err = %v", err)
	}
}

// A token outliving the declared maximum would also outlive a subject-form
// revocation entry, which is retained for exactly that long.
func TestBearerVerificationRefusesATokenLongerThanTheDeclaredLifetime(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.MaxTokenLifetime = 10 * time.Minute })
	claims := issuer.standardClaims()
	now := time.Now().Unix()
	claims["iat"] = now
	claims["exp"] = now + int64((2 * time.Hour).Seconds())
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)

	if _, err := verifier.verify(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a token outliving the declared maximum was accepted: err = %v", err)
	}
}

func TestBearerVerificationRefusesAForgedSignature(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	token := issuer.mint(t, map[string]any{"typ": "at+jwt"}, issuer.standardClaims())
	// Flip the last signature byte, which keeps the shape and breaks the proof.
	forged := token[:len(token)-1] + string(flipBase64Char(token[len(token)-1]))

	if _, err := verifier.verify(context.Background(), forged); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a forged signature was accepted: err = %v", err)
	}
}

func flipBase64Char(c byte) byte {
	if c == 'A' {
		return 'B'
	}
	return 'A'
}

// alg none is refused by the verifier itself, and this mode never gains a
// branch that would accept it.
func TestBearerVerificationRefusesAlgorithmNone(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)
	claims, err := json.Marshal(issuer.standardClaims())
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"at+jwt"}`)) +
		"." + base64.RawURLEncoding.EncodeToString(claims) + "."

	if _, err := verifier.verify(context.Background(), unsigned); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("an unsigned token was accepted: err = %v", err)
	}
}

// scope is a space-delimited string, so a whole-value comparison would admit
// "not-admin" for a required "admin". Splitting it is why it has its own field.
func TestBearerVerificationSplitsTheScopeClaim(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.RequiredScopes = []string{"admin"} })

	claims := issuer.standardClaims()
	claims["scope"] = "read not-admin write"
	denied := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)
	if _, err := verifier.verify(context.Background(), denied); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf(`a token scoped "not-admin" satisfied a required "admin": err = %v`, err)
	}

	claims["scope"] = "read admin write"
	granted := issuer.mint(t, map[string]any{"typ": "at+jwt"}, claims)
	if _, err := verifier.verify(context.Background(), granted); err != nil {
		t.Fatalf("a correctly scoped token was rejected: %v", err)
	}
}

// A forged stream of kid values must not be amplifiable into traffic against
// the issuer.
func TestUnknownKeyIDRefreshIsRateLimited(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, func(c *JWTConfig) { c.JWKSRefreshCooldown = time.Hour })
	before := issuer.fetches

	for i := range 5 {
		token := issuer.mint(t, map[string]any{"typ": "at+jwt", "kid": fmt.Sprintf("forged-%d", i)},
			issuer.standardClaims())
		if _, err := verifier.verify(context.Background(), token); err == nil {
			t.Fatal("a token signed under an unknown kid was accepted")
		}
	}
	// The first request warms the cache; the cooldown covers every retry after.
	if fetched := issuer.fetches - before; fetched > 1 {
		t.Fatalf("5 forged kid values caused %d key set fetches", fetched)
	}
}

// A metadata document naming another issuer must not be trusted, or a
// deployment pointed at the wrong host would accept whatever that host called
// itself.
func TestDiscoveryRefusesAMetadataIssuerMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   "https://someone.else.example",
			"jwks_uri": "https://someone.else.example/jwks",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	config := validJWTConfig().JWT
	config.Issuer = server.URL
	config.AllowLoopbackHTTP = true
	verifier, err := newBearerVerifier(config)
	if err != nil {
		t.Fatal(err)
	}
	// Any token will do: resolution fails before the signature is checked.
	if _, err := verifier.verify(context.Background(), "a.b.c"); err == nil {
		t.Fatal("a metadata document naming another issuer was trusted")
	}
}

func TestBearerCredentialExtraction(t *testing.T) {
	issuer := newTestIssuer(t)
	verifier := testVerifier(t, issuer, nil)

	for name, testCase := range map[string]struct {
		headers []string
		want    error
	}{
		"absent":         {nil, ErrNoCredential},
		"other scheme":   {[]string{"Basic dXNlcjpwYXNz"}, ErrNoCredential},
		"empty token":    {[]string{"Bearer "}, ErrInvalidToken},
		"two headers":    {[]string{"Bearer a.b.c", "Bearer d.e.f"}, ErrInvalidToken},
		"oversized":      {[]string{"Bearer " + strings.Repeat("x", 9000)}, ErrInvalidToken},
		"lowercase ok":   {[]string{"bearer a.b.c"}, nil},
		"capitalized ok": {[]string{"Bearer a.b.c"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
			for _, value := range testCase.headers {
				request.Header.Add("Authorization", value)
			}
			_, err := verifier.bearerCredential(request)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("err = %v, want %v", err, testCase.want)
			}
		})
	}
}
