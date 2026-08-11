package session

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultDataCookieName holds the sealed record of a CookieStore. It is a
// second cookie beside the token cookie of the Manager, because the token
// names the record and this one carries it.
const DefaultDataCookieName = "pw_session_data"

// recordFormat versions the encoded record. A record written by another
// version is rejected rather than reinterpreted.
const recordFormat = 1

// maxMethodBytes bounds the authentication method label inside a record.
const maxMethodBytes = 64

// CookieStoreOptions configures a CookieStore. Keys is required.
type CookieStoreOptions struct {
	// Keys seals every record. Its first secret writes; the rest keep records
	// written before a rotation readable.
	Keys *Keyring
	// Cookie is the policy of the record cookie. Name defaults to
	// DefaultDataCookieName; the remaining fields normally repeat the policy
	// of the session cookie so both expire under the same rules.
	Cookie CookieOptions
	// MaxBytes bounds the cookie name and encoded record together. It defaults
	// to DefaultMaxCookieBytes.
	MaxBytes int
	Now      func() time.Time
	Random   io.Reader
}

// CookieStore keeps session records in a sealed browser cookie instead of in a
// backend. It implements RawStore and RequestBinder, so a Manager built over
// Typed of it behaves like one built over sessionstore/sqlite: the same Options,
// the same Create, Rotate, and Delete, and the same Read[T] in handlers. Moving
// a deployment to a persistent backend later replaces the store and nothing
// else.
//
// The record is sealed under the hash of the session token, so a record cookie
// is only readable together with the token cookie it was issued with, and a
// record captured from one browser cannot be presented with another token.
//
// What a browser holds, a browser keeps. This store cannot revoke a record it
// has already written: Delete expires the client copy, and a client that kept
// one can replay it until its sealed expiry passes. A deployment that must be
// able to end a session immediately, or whose payload outgrows a cookie, uses
// a server-side store instead.
type CookieStore struct {
	value    cookieCodec
	cookie   CookieOptions
	sameSite http.SameSite
	maxBytes int
	now      func() time.Time
}

// NewCookieStore validates options and returns a store. Wrap it with Typed to
// give a Manager the payload type it stores.
func NewCookieStore(options CookieStoreOptions) (*CookieStore, error) {
	cookie, sameSite, err := normalizeCookie(options.Cookie, DefaultDataCookieName)
	if err != nil {
		return nil, err
	}
	maxBytes, err := cookieBudget(options.MaxBytes)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	// A session record is never client-readable, so this store offers no mode
	// choice: it always seals.
	value, err := newCookieCodec(CookieSealed, cookie.Name, options.Keys, now, options.Random)
	if err != nil {
		return nil, err
	}
	return &CookieStore{
		value:    value,
		cookie:   cookie,
		sameSite: sameSite,
		maxBytes: maxBytes,
		now:      now,
	}, nil
}

// CookieName reports the record cookie name.
func (s *CookieStore) CookieName() string { return s.cookie.Name }

// cookieCarrier is the request and response one store call works against.
// carrierKey identifies one store's carrier, so two stores in one request
// never read each other's cookie.
type carrierKey struct{ store any }

// BindRequest implements RequestBinder.
func (s *CookieStore) BindRequest(ctx context.Context, carrier Carrier) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, carrierKey{s}, carrier)
}

func (s *CookieStore) carrier(ctx context.Context) (Carrier, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: cookie store outside a request", ErrUnavailable)
	}
	carrier, ok := ctx.Value(carrierKey{s}).(Carrier)
	if !ok || carrier == nil {
		return nil, fmt.Errorf("%w: cookie store outside a request", ErrUnavailable)
	}
	return carrier, nil
}

// Put seals record under keyHash and writes the record cookie.
func (s *CookieStore) Put(ctx context.Context, keyHash string, record RawRecord) error {
	if !validKeyHash(keyHash) {
		return ErrInvalidKey
	}
	carrier, err := s.carrier(ctx)
	if err != nil {
		return err
	}
	if !writable(carrier) {
		return fmt.Errorf("%w: cookie store without a response", ErrUnavailable)
	}
	if len(record.Payload) == 0 {
		return fmt.Errorf("%w: empty payload", ErrCodec)
	}
	if len(record.Method) > maxMethodBytes {
		return fmt.Errorf("%w: method label", ErrCodec)
	}
	deadline := record.Deadline()
	encoded, err := s.value.encode(encodeRecord(record), deadline, keyHash)
	if err != nil {
		return err
	}
	if len(s.cookie.Name)+len(encoded) > s.maxBytes {
		// A browser drops an oversized cookie without saying so, which would
		// look like a session that silently never starts.
		return fmt.Errorf("%w: %d bytes; use a server-side store", ErrCookieTooLarge, len(encoded))
	}
	carrier.SetCookie(s.newCookie(encoded, deadline))
	return nil
}

// Get returns the record the request carries under keyHash. A missing or
// unauthentic cookie is ErrNotFound: it is stale browser state, not a backend
// failure, and the request continues unauthenticated.
func (s *CookieStore) Get(ctx context.Context, keyHash string) (RawRecord, error) {
	var zero RawRecord
	if !validKeyHash(keyHash) {
		return zero, ErrInvalidKey
	}
	carrier, err := s.carrier(ctx)
	if err != nil {
		return zero, err
	}
	if !readable(carrier) {
		return zero, ErrNotFound
	}
	cookie := lookupCookie(carrier.Cookies(), s.cookie.Name)
	if cookie == nil || cookie.Value == "" {
		return zero, ErrNotFound
	}
	blob, err := s.value.decode(cookie.Value, keyHash)
	switch {
	case errors.Is(err, ErrExpired):
		return zero, ErrExpired
	case err != nil:
		return zero, ErrNotFound
	}
	record, err := decodeRecord(blob)
	if err != nil {
		return zero, err
	}
	// The sealed expiry is authoritative over the cookie attributes, which the
	// client controls. A zero deadline is no deadline, which is what the codec
	// already means by a zero expiry stamp: a session whose lifetime source
	// declared no bound is held by the browser alone.
	if deadline := record.Deadline(); !deadline.IsZero() && !deadline.After(s.now()) {
		return zero, ErrExpired
	}
	return record, nil
}

// Touch renews the record cookie. Like a backend store it never revives a
// missing or expired record and never renews past the absolute expiry.
func (s *CookieStore) Touch(ctx context.Context, keyHash string, lastSeenAt, idleExpiresAt time.Time) error {
	record, err := s.Get(ctx, keyHash)
	if err != nil {
		return err
	}
	if !record.ExpiresAt.IsZero() && idleExpiresAt.After(record.ExpiresAt) {
		return ErrNotFound
	}
	record.LastSeenAt = lastSeenAt
	record.IdleExpiresAt = idleExpiresAt
	return s.Put(ctx, keyHash, record)
}

// Delete expires the record cookie. It is idempotent, and it only reaches the
// browser that made this request: a copy taken earlier stays valid until its
// sealed expiry.
func (s *CookieStore) Delete(ctx context.Context, keyHash string) error {
	if !validKeyHash(keyHash) {
		return ErrInvalidKey
	}
	carrier, err := s.carrier(ctx)
	if err != nil {
		return err
	}
	if !writable(carrier) {
		return nil
	}
	cookie := s.newCookie("", time.Time{})
	cookie.MaxAge = -1
	carrier.SetCookie(cookie)
	return nil
}

func (s *CookieStore) newCookie(value string, expiresAt time.Time) *http.Cookie {
	cookie := &http.Cookie{
		Name:     s.cookie.Name,
		Value:    value,
		Path:     s.cookie.Path,
		Domain:   s.cookie.Domain,
		Secure:   s.cookie.Secure,
		HttpOnly: s.cookie.HTTPOnly,
		SameSite: s.sameSite,
	}
	if !expiresAt.IsZero() {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(expiresAt.Sub(s.now()).Seconds())
	}
	return cookie
}

// encodeRecord lays out the record timestamps ahead of the encoded payload.
// The layout is fixed width, so renewal rewrites the header without touching
// the payload encoding.
func encodeRecord(record RawRecord) []byte {
	blob := make([]byte, 0, 1+5*8+4+1+len(record.Method)+len(record.Payload))
	blob = append(blob, recordFormat)
	for _, stamp := range []time.Time{
		record.CreatedAt, record.AuthenticatedAt, record.LastSeenAt,
		record.ExpiresAt, record.IdleExpiresAt,
	} {
		blob = binary.BigEndian.AppendUint64(blob, milliOf(stamp))
	}
	blob = binary.BigEndian.AppendUint32(blob, uint32(record.Version))
	blob = append(blob, byte(len(record.Method)))
	blob = append(blob, record.Method...)
	return append(blob, record.Payload...)
}

func decodeRecord(blob []byte) (RawRecord, error) {
	var zero RawRecord
	const header = 1 + 5*8 + 4 + 1
	if len(blob) < header || blob[0] != recordFormat {
		return zero, fmt.Errorf("%w: record layout", ErrCodec)
	}
	stamps := make([]time.Time, 0, 5)
	for index := range 5 {
		offset := 1 + index*8
		stamps = append(stamps, timeOf(binary.BigEndian.Uint64(blob[offset:offset+8])))
	}
	version := int(binary.BigEndian.Uint32(blob[41:45]))
	methodLength := int(blob[45])
	if len(blob) < header+methodLength {
		return zero, fmt.Errorf("%w: record layout", ErrCodec)
	}
	payload := blob[header+methodLength:]
	if len(payload) == 0 {
		return zero, fmt.Errorf("%w: empty payload", ErrCodec)
	}
	return RawRecord{
		Payload:         payload,
		CreatedAt:       stamps[0],
		AuthenticatedAt: stamps[1],
		LastSeenAt:      stamps[2],
		ExpiresAt:       stamps[3],
		IdleExpiresAt:   stamps[4],
		Method:          string(blob[header : header+methodLength]),
		Version:         version,
	}, nil
}

func milliOf(stamp time.Time) uint64 {
	if stamp.IsZero() || stamp.UnixMilli() < 0 {
		return 0
	}
	return uint64(stamp.UnixMilli())
}

func timeOf(milli uint64) time.Time {
	if milli == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(milli))
}

// validKeyHash reports whether value has the syntax of a store key.
func validKeyHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

var (
	_ RawStore      = (*CookieStore)(nil)
	_ RequestBinder = (*CookieStore)(nil)
)
