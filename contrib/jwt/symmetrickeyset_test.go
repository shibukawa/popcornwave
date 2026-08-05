package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"testing"
)

func rsaKeySetDocument(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	modulus := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
	exponent := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())
	return []byte(`{"keys":[{"kty":"RSA","kid":"one","use":"sig","alg":"RS256","n":"` + modulus + `","e":"` + exponent + `"}]}`)
}

// A JWKS is a document published so anyone can verify a token, and an HMAC key
// is a shared secret. An oct entry in a fetched key set is therefore the
// verification secret handed to every reader: fetch the document, read k, and
// mint a token carrying any subject.
func TestAFetchedKeySetDropsSymmetricKeys(t *testing.T) {
	secret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	document := []byte(`{"keys":[{"kty":"oct","kid":"one","use":"sig","alg":"HS256","k":"` + secret + `"}]}`)

	set, err := ParseJWKS(document, JWKSOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.ResolveKey(Header{Algorithm: "HS256", KeyID: "one"}); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("ResolveKey err = %v, want ErrKeyNotFound for a symmetric key in a published set", err)
	}
}

// A set built in-process — from a file, a vault, a fixture — is not published,
// and an HMAC entry there is ordinary.
func TestALocalKeySetMayCarrySymmetricKeys(t *testing.T) {
	secret := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	document := []byte(`{"keys":[{"kty":"oct","kid":"one","use":"sig","alg":"HS256","k":"` + secret + `"}]}`)

	set, err := ParseJWKS(document, JWKSOptions{AllowSymmetric: true})
	if err != nil {
		t.Fatal(err)
	}
	key, err := set.ResolveKey(Header{Algorithm: "HS256", KeyID: "one"})
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key.Algorithm != "HS256" || len(key.HMAC) != 32 {
		t.Errorf("key = %+v, want a 32-byte HS256 secret", key)
	}
}

// The asymmetric case is unaffected either way, which is what makes the default
// safe to change.
func TestAsymmetricKeysAreUnaffected(t *testing.T) {
	for _, allow := range []bool{false, true} {
		set, err := ParseJWKS(rsaKeySetDocument(t), JWKSOptions{AllowSymmetric: allow})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := set.ResolveKey(Header{Algorithm: "RS256", KeyID: "one"}); err != nil {
			t.Errorf("AllowSymmetric=%t: ResolveKey: %v", allow, err)
		}
	}
}
