package authn

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

type failingRandom struct{}

func (failingRandom) Read([]byte) (int, error) { return 0, errors.New("random failure") }

func TestGenerateSecretAndDecode(t *testing.T) {
	secret, err := GenerateSecret(bytes.NewReader(make([]byte, 16)), 16)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeBase64URL(secret, 64, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 16 {
		t.Fatalf("decoded length = %d", len(decoded))
	}
	if !EqualSecret(secret, secret) || EqualSecret(secret, secret+"a") {
		t.Fatal("EqualSecret returned an invalid result")
	}
}

func TestGenerateSecretBoundsAndRandomFailure(t *testing.T) {
	for _, byteCount := range []int{0, MaxSecretBytes + 1} {
		if _, err := GenerateSecret(bytes.NewReader(nil), byteCount); !errors.Is(err, ErrInvalidSize) {
			t.Fatalf("byteCount %d error = %v", byteCount, err)
		}
	}
	if _, err := GenerateSecret(failingRandom{}, 16); err == nil || err.Error() != "random failure" {
		t.Fatalf("random failure = %v", err)
	}
}

func TestEqualSecretBoundsInput(t *testing.T) {
	if EqualSecret(strings.Repeat("a", MaxEncodedSecretBytes+1), "a") {
		t.Fatal("oversized secret must not compare equal")
	}
}

func TestDecodeBase64URLRejectsPaddingAndLimits(t *testing.T) {
	if _, err := DecodeBase64URL("YQ==", 16, 16); !errors.Is(err, ErrInvalidEncoding) {
		t.Fatalf("padding error = %v", err)
	}
	if _, err := DecodeBase64URL("YWFh", 3, 16); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("encoded limit error = %v", err)
	}
}

func TestPKCERFCVector(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got, err := PKCEChallengeS256(verifier)
	if err != nil {
		t.Fatal(err)
	}
	if got != challenge {
		t.Fatalf("challenge = %q", got)
	}
}

func TestPKCERejectsInvalidVerifier(t *testing.T) {
	for _, verifier := range []string{"", strings.Repeat("a", 42), strings.Repeat("a", 129), strings.Repeat("a", 42) + "/"} {
		if _, err := PKCEChallengeS256(verifier); !errors.Is(err, ErrInvalidVerifier) {
			t.Errorf("verifier length=%d error = %v", len(verifier), err)
		}
	}
}

func TestRequireUnexpired(t *testing.T) {
	now := time.Unix(100, 0)
	if err := RequireUnexpired(now, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := RequireUnexpired(now, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry error = %v", err)
	}
}

func TestValidateJSON(t *testing.T) {
	options := JSONOptions{MaxBytes: 128, MaxDepth: 4, MaxMembers: 8}
	if err := ValidateJSON([]byte(`{"a":[1,{"b":true}]}`), options); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSON([]byte(`{"a":1,"a":2}`), options); !errors.Is(err, ErrDuplicateJSON) {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := ValidateJSON([]byte(`{"outer":{"inner":1,"inner":2}}`), options); !errors.Is(err, ErrDuplicateJSON) {
		t.Fatalf("nested duplicate error = %v", err)
	}
	if err := ValidateJSON([]byte(`{} {}`), options); !errors.Is(err, ErrMalformedJSON) {
		t.Fatalf("trailing error = %v", err)
	}
	if err := ValidateJSON([]byte(`[[[[[]]]]]`), options); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("depth error = %v", err)
	}
}

func TestParseEndpointRejectsEmptyHostname(t *testing.T) {
	if _, err := ParseEndpoint("https://:443/token", false); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("empty hostname error = %v", err)
	}
}
