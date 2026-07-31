package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// CookieMode selects how much protection a cookie value carries. A stronger
// mode never changes the typed API, so a cookie can be promoted from Plain to
// Sealed without touching the handlers that read it.
type CookieMode int

const (
	// CookiePlain writes the payload without protection. The client can read
	// it and can replace it, so a handler must treat the decoded value as
	// request input rather than as application state.
	CookiePlain CookieMode = iota + 1
	// CookieSigned appends a message authentication code. The client can still
	// read the payload but cannot change it, and a changed value is rejected
	// instead of being decoded.
	CookieSigned
	// CookieSealed encrypts and authenticates the payload. The client can
	// neither read nor change it.
	CookieSealed
)

// String names the mode for diagnostics. It never renders a key or a value.
func (m CookieMode) String() string {
	switch m {
	case CookiePlain:
		return "plain"
	case CookieSigned:
		return "signed"
	case CookieSealed:
		return "sealed"
	default:
		return "unknown"
	}
}

var (
	// ErrCookieMissing reports that the request carries no such cookie.
	ErrCookieMissing = errors.New("session: cookie not present")
	// ErrCookieInvalid reports a value this keyring did not produce: a changed
	// payload, a foreign encoding, a value moved from another cookie name, or
	// one written under a retired key.
	ErrCookieInvalid = errors.New("session: cookie value is not authentic")
	// ErrCookieTooLarge reports an encoded value beyond the configured budget.
	// Browsers drop an oversized cookie silently, so it is refused at the
	// write instead.
	ErrCookieTooLarge = errors.New("session: cookie value exceeds the size limit")
)

const (
	// cookieSecretBytes is the minimum length of one keyring secret.
	// policy:cookie-value-protection requires 256 bits.
	cookieSecretBytes = 32
	// maxKeyringSecrets bounds how many retired keys a value is tried against.
	maxKeyringSecrets = 8
	// nonceBytes is the AES-GCM nonce length.
	nonceBytes = 12
	// expiryBytes is the fixed-width expiry prefix of a protected payload.
	expiryBytes = 8

	// DefaultMaxCookieBytes bounds the encoded value of a Jar or CookieStore
	// cookie. Browsers commonly accept about 4096 bytes per cookie including
	// its name and attributes, so the default leaves room for both.
	DefaultMaxCookieBytes = 3800
	hardMaxCookieBytes    = 4096
)

// Wire format of a protected value. The version and mode tag are read before
// any key is used, so a value written in another mode is rejected rather than
// decoded under the wrong rules.
const (
	cookieVersion = "1"
	tagPlain      = "p"
	tagSigned     = "s"
	tagSealed     = "e"
)

var cookieEncoding = base64.RawURLEncoding

// Keyring holds the secret that protects signed and sealed cookies. The first
// secret writes every new value; the remaining ones only read, which is what
// makes a key rotation invisible to a browser that still holds a value written
// under the previous secret.
//
// A secret is 32 or more random bytes and is never logged.
type Keyring struct {
	keys []cookieKey
}

// cookieKey is one secret expanded into its two purpose-separated subkeys. The
// same secret can therefore serve a signed and a sealed cookie without either
// use weakening the other.
type cookieKey struct {
	sign []byte
	seal cipher.AEAD
}

// NewKeyring returns a keyring over raw secrets. The first secret is the
// writing key and the rest are accepted for reading during a rotation.
func NewKeyring(secrets ...[]byte) (*Keyring, error) {
	if len(secrets) == 0 {
		return nil, fmt.Errorf("%w: empty keyring", ErrInvalidKey)
	}
	if len(secrets) > maxKeyringSecrets {
		return nil, fmt.Errorf("%w: too many secrets", ErrInvalidKey)
	}
	keys := make([]cookieKey, 0, len(secrets))
	for _, secret := range secrets {
		if len(secret) < cookieSecretBytes {
			return nil, fmt.Errorf("%w: secret shorter than %d bytes", ErrInvalidKey, cookieSecretBytes)
		}
		block, err := aes.NewCipher(derive(secret, "popcornwave/cookie/seal/v1"))
		if err != nil {
			return nil, fmt.Errorf("%w: cipher", ErrInvalidKey)
		}
		seal, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("%w: aead", ErrInvalidKey)
		}
		keys = append(keys, cookieKey{sign: derive(secret, "popcornwave/cookie/sign/v1"), seal: seal})
	}
	return &Keyring{keys: keys}, nil
}

// ParseKeyring is NewKeyring over base64 secrets, which is the form a
// configuration file or environment variable carries. Standard and URL
// alphabets are both accepted, with or without padding.
//
// Generate one with `openssl rand -base64 32`.
func ParseKeyring(secrets ...string) (*Keyring, error) {
	raw := make([][]byte, 0, len(secrets))
	for _, secret := range secrets {
		decoded, err := decodeSecret(secret)
		if err != nil {
			return nil, err
		}
		raw = append(raw, decoded)
	}
	return NewKeyring(raw...)
}

func decodeSecret(secret string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("%w: empty secret", ErrInvalidKey)
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(secret); err == nil {
			return decoded, nil
		}
	}
	// The secret itself never reaches the error, only its rejected shape.
	return nil, fmt.Errorf("%w: secret is not base64", ErrInvalidKey)
}

// derive expands one secret into a purpose-separated subkey. The secret is
// already uniform random of full length, so one PRF application per label is
// the whole derivation.
func derive(secret []byte, label string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}

// cookieCodec encodes and decodes one cookie value under one mode. The cookie
// name and an optional binding string are authenticated but never written into
// the value, so a value cannot be replayed under another name or against
// another owner.
type cookieCodec struct {
	mode   CookieMode
	name   string
	keys   *Keyring
	now    func() time.Time
	random io.Reader
}

func newCookieCodec(mode CookieMode, name string, keys *Keyring, now func() time.Time, random io.Reader) (cookieCodec, error) {
	switch mode {
	case CookiePlain:
	case CookieSigned, CookieSealed:
		if keys == nil || len(keys.keys) == 0 {
			return cookieCodec{}, fmt.Errorf("%w: %s cookie needs a keyring", ErrInvalidKey, mode)
		}
	default:
		return cookieCodec{}, fmt.Errorf("%w: cookie mode", ErrInvalidOptions)
	}
	if now == nil {
		now = time.Now
	}
	if random == nil {
		random = rand.Reader
	}
	return cookieCodec{mode: mode, name: name, keys: keys, now: now, random: random}, nil
}

// binding is the authenticated context of a value: format version, mode,
// cookie name, and the caller's binding string.
func (c cookieCodec) binding(tag, aad string) string {
	return cookieVersion + "." + tag + "." + c.name + "\x00" + aad
}

// encode returns the cookie value carrying payload.
//
// expiresAt is embedded in a protected value so that this process stops
// accepting it at the same moment the browser was asked to forget it. A plain
// value carries no expiry, because the client could rewrite it anyway.
func (c cookieCodec) encode(payload []byte, expiresAt time.Time, aad string) (string, error) {
	switch c.mode {
	case CookiePlain:
		return cookieVersion + "." + tagPlain + "." + cookieEncoding.EncodeToString(payload), nil
	case CookieSigned:
		body := cookieEncoding.EncodeToString(withExpiry(payload, expiresAt))
		mac := c.keys.sign(c.binding(tagSigned, aad), body)
		return cookieVersion + "." + tagSigned + "." + body + "." + cookieEncoding.EncodeToString(mac), nil
	case CookieSealed:
		sealed, err := c.keys.seal(c.binding(tagSealed, aad), withExpiry(payload, expiresAt), c.random)
		if err != nil {
			return "", err
		}
		return cookieVersion + "." + tagSealed + "." + cookieEncoding.EncodeToString(sealed), nil
	default:
		return "", fmt.Errorf("%w: cookie mode", ErrInvalidOptions)
	}
}

// decode returns the payload of value, or ErrCookieInvalid for anything this
// keyring did not write under this name and binding, and ErrExpired for a
// value past its embedded expiry.
func (c cookieCodec) decode(value, aad string) ([]byte, error) {
	tag, rest, ok := splitCookieValue(value)
	if !ok || tag != c.tag() {
		return nil, ErrCookieInvalid
	}
	switch c.mode {
	case CookiePlain:
		payload, err := cookieEncoding.DecodeString(rest)
		if err != nil {
			return nil, ErrCookieInvalid
		}
		return payload, nil
	case CookieSigned:
		body, encodedMAC, found := strings.Cut(rest, ".")
		if !found {
			return nil, ErrCookieInvalid
		}
		mac, err := cookieEncoding.DecodeString(encodedMAC)
		if err != nil {
			return nil, ErrCookieInvalid
		}
		if !c.keys.verify(c.binding(tagSigned, aad), body, mac) {
			return nil, ErrCookieInvalid
		}
		stamped, err := cookieEncoding.DecodeString(body)
		if err != nil {
			return nil, ErrCookieInvalid
		}
		return c.unstamp(stamped)
	case CookieSealed:
		sealed, err := cookieEncoding.DecodeString(rest)
		if err != nil {
			return nil, ErrCookieInvalid
		}
		stamped, ok := c.keys.open(c.binding(tagSealed, aad), sealed)
		if !ok {
			return nil, ErrCookieInvalid
		}
		return c.unstamp(stamped)
	default:
		return nil, fmt.Errorf("%w: cookie mode", ErrInvalidOptions)
	}
}

func (c cookieCodec) tag() string {
	switch c.mode {
	case CookieSigned:
		return tagSigned
	case CookieSealed:
		return tagSealed
	default:
		return tagPlain
	}
}

// unstamp splits the authenticated expiry from the payload.
func (c cookieCodec) unstamp(stamped []byte) ([]byte, error) {
	if len(stamped) < expiryBytes {
		return nil, ErrCookieInvalid
	}
	milli := binary.BigEndian.Uint64(stamped[:expiryBytes])
	if milli != 0 && !time.UnixMilli(int64(milli)).After(c.now()) {
		return nil, ErrExpired
	}
	return stamped[expiryBytes:], nil
}

func withExpiry(payload []byte, expiresAt time.Time) []byte {
	stamped := make([]byte, expiryBytes+len(payload))
	if !expiresAt.IsZero() && expiresAt.UnixMilli() > 0 {
		binary.BigEndian.PutUint64(stamped[:expiryBytes], uint64(expiresAt.UnixMilli()))
	}
	copy(stamped[expiryBytes:], payload)
	return stamped
}

// splitCookieValue separates the mode tag from the rest of a versioned value.
func splitCookieValue(value string) (tag, rest string, ok bool) {
	version, rest, found := strings.Cut(value, ".")
	if !found || version != cookieVersion {
		return "", "", false
	}
	tag, rest, found = strings.Cut(rest, ".")
	if !found || tag == "" {
		return "", "", false
	}
	return tag, rest, true
}

// sign returns the MAC of body under the writing key.
func (k *Keyring) sign(binding, body string) []byte {
	return macOf(k.keys[0].sign, binding, body)
}

// verify accepts a MAC produced by any key in the ring, which is what lets a
// rotation keep reading values written under the previous secret.
func (k *Keyring) verify(binding, body string, mac []byte) bool {
	for _, key := range k.keys {
		if hmac.Equal(macOf(key.sign, binding, body), mac) {
			return true
		}
	}
	return false
}

func macOf(key []byte, binding, body string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(binding))
	mac.Write([]byte{0})
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

// seal encrypts plaintext under the writing key, with binding as associated
// data so the result decrypts only under the same name and binding.
func (k *Keyring) seal(binding string, plaintext []byte, random io.Reader) ([]byte, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("%w: nonce", ErrUnavailable)
	}
	sealed := k.keys[0].seal.Seal(nil, nonce, plaintext, []byte(binding))
	return append(nonce, sealed...), nil
}

// open decrypts under any key in the ring.
func (k *Keyring) open(binding string, sealed []byte) ([]byte, bool) {
	if len(sealed) < nonceBytes {
		return nil, false
	}
	nonce, ciphertext := sealed[:nonceBytes], sealed[nonceBytes:]
	for _, key := range k.keys {
		if plaintext, err := key.seal.Open(nil, nonce, ciphertext, []byte(binding)); err == nil {
			return plaintext, true
		}
	}
	return nil, false
}
