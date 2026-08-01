package session

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func testSecret(fill byte) []byte { return bytes.Repeat([]byte{fill}, cookieSecretBytes) }

func testKeyring(t *testing.T, fills ...byte) *Keyring {
	t.Helper()
	secrets := make([][]byte, 0, len(fills))
	for _, fill := range fills {
		secrets = append(secrets, testSecret(fill))
	}
	keys, err := NewKeyring(secrets...)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return keys
}

func testCodec(t *testing.T, mode CookieMode, name string, keys *Keyring, now func() time.Time) cookieCodec {
	t.Helper()
	codec, err := newCookieCodec(mode, name, keys, now, nil)
	if err != nil {
		t.Fatalf("newCookieCodec: %v", err)
	}
	return codec
}

func TestCookieValueRoundTripsInEveryMode(t *testing.T) {
	keys := testKeyring(t, 1)
	for _, mode := range []CookieMode{CookiePlain, CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			codec := testCodec(t, mode, "pref", keys, nil)
			value, err := codec.encode([]byte(`{"theme":"dark"}`), time.Time{}, "")
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			// A cookie value never needs quoting or escaping by net/http.
			if strings.ContainsAny(value, ` ,;"\`) {
				t.Fatalf("value %q is not cookie safe", value)
			}
			payload, err := codec.decode(value, "")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if string(payload) != `{"theme":"dark"}` {
				t.Fatalf("payload = %q", payload)
			}
		})
	}
}

func TestPlainValueIsReadableAndSealedValueIsNot(t *testing.T) {
	keys := testKeyring(t, 1)
	plain, err := testCodec(t, CookiePlain, "pref", nil, nil).encode([]byte("theme=dark"), time.Time{}, "")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// A client reads a plain value with one base64 decode, which is the point
	// of the mode.
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(plain, "1.p."))
	if err != nil || string(decoded) != "theme=dark" {
		t.Fatalf("plain value %q is not client readable: %v", plain, err)
	}

	sealed, err := testCodec(t, CookieSealed, "pref", keys, nil).encode([]byte("theme=dark"), time.Time{}, "")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(sealed, "theme") {
		t.Fatalf("sealed value leaks its payload: %q", sealed)
	}
	if raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(sealed, "1.e.")); err != nil ||
		bytes.Contains(raw, []byte("theme")) {
		t.Fatalf("sealed bytes leak their payload")
	}
}

func TestProtectedValueRejectsTamperingAndReuse(t *testing.T) {
	keys := testKeyring(t, 1)
	for _, mode := range []CookieMode{CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			codec := testCodec(t, mode, "pref", keys, nil)
			value, err := codec.encode([]byte(`{"admin":false}`), time.Time{}, "account-1")
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			// A changed byte in the body is rejected, not decoded. The first
			// body character follows the four character "1.s." prefix.
			flipped := []byte(value)
			if flipped[4] == 'A' {
				flipped[4] = 'B'
			} else {
				flipped[4] = 'A'
			}
			if _, err := codec.decode(string(flipped), "account-1"); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("tampered value error = %v", err)
			}
			// A value moved to another cookie name is rejected.
			other := testCodec(t, mode, "other", keys, nil)
			if _, err := other.decode(value, "account-1"); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("renamed value error = %v", err)
			}
			// A value presented under another binding is rejected, which is
			// what stops one browser's record from being replayed with
			// another owner's token.
			if _, err := codec.decode(value, "account-2"); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("rebound value error = %v", err)
			}
			// A value written under another keyring is rejected.
			foreign := testCodec(t, mode, "pref", testKeyring(t, 9), nil)
			if _, err := foreign.decode(value, "account-1"); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("foreign key error = %v", err)
			}
		})
	}
}

func TestModeIsNotNegotiableByTheClient(t *testing.T) {
	keys := testKeyring(t, 1)
	// A client that downgrades a sealed cookie to a plain one it can author
	// must not be read as plain by a sealed jar, and the reverse must not
	// bypass authentication either.
	plain, err := testCodec(t, CookiePlain, "pref", keys, nil).encode([]byte("admin=true"), time.Time{}, "")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := testCodec(t, CookieSealed, "pref", keys, nil).decode(plain, ""); !errors.Is(err, ErrCookieInvalid) {
		t.Fatalf("downgraded value error = %v", err)
	}
	signed, err := testCodec(t, CookieSigned, "pref", keys, nil).encode([]byte("admin=false"), time.Time{}, "")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := testCodec(t, CookieSealed, "pref", keys, nil).decode(signed, ""); !errors.Is(err, ErrCookieInvalid) {
		t.Fatalf("cross-mode value error = %v", err)
	}
}

func TestProtectedValueExpiresOnItsOwnStamp(t *testing.T) {
	c := &clock{now: time.Unix(1_700_000_000, 0)}
	keys := testKeyring(t, 1)
	for _, mode := range []CookieMode{CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			codec := testCodec(t, mode, "pref", keys, c.Now)
			value, err := codec.encode([]byte("x"), c.Now().Add(time.Hour), "")
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if _, err := codec.decode(value, ""); err != nil {
				t.Fatalf("decode before expiry: %v", err)
			}
			// The browser was asked to forget the value; a client that kept it
			// anyway is refused by the stamp inside the authenticated payload.
			c.advance(2 * time.Hour)
			if _, err := codec.decode(value, ""); !errors.Is(err, ErrExpired) {
				t.Fatalf("decode after expiry error = %v", err)
			}
			c.advance(-2 * time.Hour)
		})
	}
}

func TestKeyRotationKeepsPreviousValuesReadable(t *testing.T) {
	for _, mode := range []CookieMode{CookieSigned, CookieSealed} {
		t.Run(mode.String(), func(t *testing.T) {
			old := testCodec(t, mode, "pref", testKeyring(t, 1), nil)
			value, err := old.encode([]byte("kept"), time.Time{}, "")
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			// The new secret writes and the retired one still reads.
			rotated := testCodec(t, mode, "pref", testKeyring(t, 2, 1), nil)
			payload, err := rotated.decode(value, "")
			if err != nil || string(payload) != "kept" {
				t.Fatalf("rotated decode = %q, %v", payload, err)
			}
			fresh, err := rotated.encode([]byte("new"), time.Time{}, "")
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if _, err := old.decode(fresh, ""); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("retired keyring accepted a value written after rotation: %v", err)
			}
			// Dropping the retired secret ends the grace period.
			narrowed := testCodec(t, mode, "pref", testKeyring(t, 2), nil)
			if _, err := narrowed.decode(value, ""); !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("dropped secret still accepted: %v", err)
			}
		})
	}
}

func TestKeyringRejectsWeakAndMalformedSecrets(t *testing.T) {
	if _, err := NewKeyring(); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("empty keyring error = %v", err)
	}
	if _, err := NewKeyring(bytes.Repeat([]byte{1}, cookieSecretBytes-1)); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("short secret error = %v", err)
	}
	_, err := ParseKeyring("not base64!")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("malformed secret error = %v", err)
	}
	// A rejected secret is still a secret: only its shape may be reported.
	if strings.Contains(err.Error(), "not base64!") {
		t.Fatalf("error repeats the secret: %v", err)
	}
	// The base64 form of a valid secret is accepted in every alphabet.
	encoded := base64.StdEncoding.EncodeToString(testSecret(3))
	if _, err := ParseKeyring(encoded, base64.RawURLEncoding.EncodeToString(testSecret(4))); err != nil {
		t.Fatalf("ParseKeyring: %v", err)
	}
}

func TestSignedAndSealedModesRequireAKeyring(t *testing.T) {
	for _, mode := range []CookieMode{CookieSigned, CookieSealed} {
		if _, err := newCookieCodec(mode, "pref", nil, nil, nil); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("%s without a keyring: %v", mode, err)
		}
	}
	if _, err := newCookieCodec(CookiePlain, "pref", nil, nil, nil); err != nil {
		t.Fatalf("plain without a keyring: %v", err)
	}
}
