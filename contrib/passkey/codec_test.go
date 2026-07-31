package passkey

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
)

func TestCeremonyStateCodecRoundTrip(t *testing.T) {
	want := CeremonyState{
		kind: authenticationCeremony, challenge: "challenge-value",
		expiresAt: time.UnixMilli(1_800_000_000_123), userHandle: []byte("user"),
		allowedCredentialIDs:    [][]byte{[]byte("credential-one"), []byte("credential-two")},
		requireUserVerification: true,
	}
	encoded, err := (CeremonyStateCodec{}).Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := (CeremonyStateCodec{}).Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != want.kind || got.challenge != want.challenge || !got.expiresAt.Equal(want.expiresAt) ||
		string(got.userHandle) != string(want.userHandle) || got.requireUserVerification != want.requireUserVerification ||
		len(got.allowedCredentialIDs) != len(want.allowedCredentialIDs) {
		t.Fatalf("decoded ceremony state = %#v, want %#v", got, want)
	}
	for i := range want.allowedCredentialIDs {
		if string(got.allowedCredentialIDs[i]) != string(want.allowedCredentialIDs[i]) {
			t.Fatalf("credential %d = %q, want %q", i, got.allowedCredentialIDs[i], want.allowedCredentialIDs[i])
		}
	}
}

func TestCeremonyStateCodecRejectsMalformedAndInvalidState(t *testing.T) {
	tests := [][]byte{
		nil,
		[]byte(`{"v":2,"kind":1,"challenge":"c","expires_at_ms":1}`),
		[]byte(`{"v":1,"kind":1,"challenge":"c","expires_at_ms":1,"extra":true}`),
		[]byte(`{"v":1,"kind":9,"challenge":"c","expires_at_ms":1}`),
		[]byte(`{"v":1} {"v":1}`),
		[]byte(strings.Repeat("x", maxCeremonyStateCodecBytes+1)),
	}
	for _, encoded := range tests {
		if _, err := (CeremonyStateCodec{}).Decode(encoded); !errors.Is(err, authstate.ErrCodec) {
			t.Fatalf("Decode error = %v, want ErrCodec", err)
		}
	}
	if _, err := (CeremonyStateCodec{}).Encode(CeremonyState{}); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("Encode error = %v, want ErrCodec", err)
	}
}
