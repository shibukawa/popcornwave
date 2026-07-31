package oauth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
)

func TestTransactionCodecRoundTrip(t *testing.T) {
	want := Transaction{
		State: "state-value", Verifier: "verifier-value", Nonce: "nonce-value",
		RedirectURI: "https://app.example/callback", ExpiresAt: time.UnixMilli(1_800_000_000_123),
	}
	encoded, err := (TransactionCodec{}).Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := (TransactionCodec{}).Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("decoded transaction = %#v, want %#v", got, want)
	}
}

func TestTransactionCodecRejectsMalformedAndOversized(t *testing.T) {
	tests := [][]byte{
		nil,
		[]byte(`{"v":2,"state":"s","verifier":"v","redirect_uri":"https://app.example/cb","expires_at_ms":1}`),
		[]byte(`{"v":1,"state":"s","verifier":"v","redirect_uri":"https://app.example/cb","expires_at_ms":1,"extra":true}`),
		[]byte(`{"v":1}{}`),
		[]byte(`{"v":1} {"v":1}`),
		[]byte(strings.Repeat("x", maxTransactionCodecBytes+1)),
	}
	for _, encoded := range tests {
		if _, err := (TransactionCodec{}).Decode(encoded); !errors.Is(err, authstate.ErrCodec) {
			t.Fatalf("Decode(%q) error = %v, want ErrCodec", encoded[:min(len(encoded), 64)], err)
		}
	}
}

func TestTransactionCodecRejectsInvalidValueWithoutLeakingIt(t *testing.T) {
	secret := "secret-verifier"
	_, err := (TransactionCodec{}).Encode(Transaction{Verifier: secret})
	if !errors.Is(err, authstate.ErrCodec) || strings.Contains(err.Error(), secret) {
		t.Fatalf("Encode error = %q", err)
	}
}
