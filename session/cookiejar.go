package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// JarOptions configures a Jar. Name is required; every other field has a safe
// default. Signed and Sealed additionally require Keys.
type JarOptions struct {
	// Mode selects the protection of the value. It defaults to CookieSealed,
	// so a jar declared without a mode does not silently hand application
	// state to the client.
	Mode CookieMode
	// Keys protects signed and sealed values. Its first secret writes; the
	// rest keep a rotation readable.
	Keys *Keyring
	// Cookie is the browser cookie policy. Name has no default.
	Cookie CookieOptions
	// MaxAge bounds how long the browser keeps the value and, in a protected
	// mode, how long this process accepts it. Zero writes a session cookie
	// that the browser drops when it closes.
	MaxAge time.Duration
	// MaxBytes bounds the cookie name and encoded value together. It defaults
	// to DefaultMaxCookieBytes.
	MaxBytes int
	Now      func() time.Time
	Random   io.Reader
}

// Jar reads and writes one typed browser cookie under one protection mode.
//
// The typed API is the same in all three modes, so a cookie that starts as
// CookiePlain during development can be promoted to CookieSigned or
// CookieSealed later without changing the handlers that use it. A value
// written under another name, another mode, or another keyring is rejected
// rather than decoded.
//
// A Jar is safe for concurrent use.
type Jar[T any] struct {
	codec    Codec[T]
	value    cookieCodec
	cookie   CookieOptions
	sameSite http.SameSite
	maxAge   time.Duration
	maxBytes int
	now      func() time.Time
}

// NewJar validates options and returns a Jar over codec. A nil codec uses
// JSONCodec[T].
func NewJar[T any](codec Codec[T], options JarOptions) (*Jar[T], error) {
	if codec == nil {
		codec = JSONCodec[T]{}
	}
	mode := options.Mode
	if mode == 0 {
		mode = CookieSealed
	}
	// A jar names its own cookie: there is no framework-wide default that
	// would be right for application state.
	cookie, sameSite, err := normalizeCookie(options.Cookie, "")
	if err != nil {
		return nil, err
	}
	if options.MaxAge < 0 || options.MaxAge > BrowserMax {
		return nil, fmt.Errorf("%w: cookie max age", ErrInvalidOptions)
	}
	maxBytes, err := cookieBudget(options.MaxBytes)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	value, err := newCookieCodec(mode, cookie.Name, options.Keys, now, options.Random)
	if err != nil {
		return nil, err
	}
	return &Jar[T]{
		codec:    codec,
		value:    value,
		cookie:   cookie,
		sameSite: sameSite,
		maxAge:   options.MaxAge,
		maxBytes: maxBytes,
		now:      now,
	}, nil
}

// Name reports the browser cookie name.
func (j *Jar[T]) Name() string { return j.cookie.Name }

// Mode reports the protection of the value.
func (j *Jar[T]) Mode() CookieMode { return j.value.mode }

// Load decodes the cookie of r.
//
// It reports ErrCookieMissing when the request carries no such cookie,
// ErrCookieInvalid for a value this jar did not write, ErrExpired for a
// protected value past its embedded expiry, and ErrCodec for a payload the
// codec cannot decode.
func (j *Jar[T]) Load(r *http.Request) (T, error) {
	if r == nil {
		var zero T
		return zero, ErrCookieMissing
	}
	cookie, err := r.Cookie(j.cookie.Name)
	if err != nil || cookie == nil {
		var zero T
		return zero, ErrCookieMissing
	}
	return j.loadValue(cookie.Value)
}

// loadValue decodes an already-extracted cookie value, for a caller that has
// parsed the Cookie header itself. An empty value reads as a missing cookie,
// exactly as Load treats it.
func (j *Jar[T]) loadValue(value string) (T, error) {
	var zero T
	if value == "" {
		return zero, ErrCookieMissing
	}
	payload, err := j.value.decode(value, "")
	if err != nil {
		return zero, err
	}
	return j.codec.Decode(payload)
}

// Save encodes value and writes the cookie. It must run before the response
// body is committed, like any other header write.
func (j *Jar[T]) Save(w http.ResponseWriter, value T) error {
	return j.SaveTo(HTTPCarrier(w, nil), value)
}

// SaveTo is Save over a carrier, for a transport whose response is not an
// http.ResponseWriter. Save keeps the net/http shape because it is the one an
// application already calls.
func (j *Jar[T]) SaveTo(carrier Carrier, value T) error {
	if !writable(carrier) {
		return fmt.Errorf("%w: nil response writer", ErrInvalidOptions)
	}
	payload, err := j.codec.Encode(value)
	if err != nil {
		return err
	}
	var expiresAt time.Time
	if j.maxAge > 0 {
		expiresAt = j.now().Add(j.maxAge)
	}
	encoded, err := j.value.encode(payload, expiresAt, "")
	if err != nil {
		return err
	}
	if len(j.cookie.Name)+len(encoded) > j.maxBytes {
		return fmt.Errorf("%w: %d bytes", ErrCookieTooLarge, len(encoded))
	}
	carrier.SetCookie(j.newCookie(encoded, expiresAt))
	return nil
}

// Clear expires the cookie in the browser.
func (j *Jar[T]) Clear(w http.ResponseWriter) { j.ClearFrom(HTTPCarrier(w, nil)) }

// ClearFrom is Clear over a carrier.
func (j *Jar[T]) ClearFrom(carrier Carrier) {
	if !writable(carrier) {
		return
	}
	cookie := j.newCookie("", time.Time{})
	cookie.MaxAge = -1
	carrier.SetCookie(cookie)
}

func (j *Jar[T]) newCookie(value string, expiresAt time.Time) *http.Cookie {
	cookie := &http.Cookie{
		Name:     j.cookie.Name,
		Value:    value,
		Path:     j.cookie.Path,
		Domain:   j.cookie.Domain,
		Secure:   j.cookie.Secure,
		HttpOnly: j.cookie.HTTPOnly,
		SameSite: j.sameSite,
	}
	if !expiresAt.IsZero() {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(expiresAt.Sub(j.now()).Seconds())
	}
	return cookie
}

// jarKey identifies one jar's value in a request context. The jar pointer is
// the identity, so two jars over the same type never read each other's value.
type jarKey struct{ jar any }

// JarValue is the request-scoped handle to one jar cookie. The middleware
// decodes the cookie once and installs it, so repeated reads in one request
// cost nothing and a write is visible to later handlers immediately.
type JarValue[T any] struct {
	jar     *Jar[T]
	carrier Carrier
	data    T
	present bool
}

// Get returns the current value and whether the request carried a usable one.
func (v *JarValue[T]) Get() (T, bool) {
	if v == nil {
		var zero T
		return zero, false
	}
	return v.data, v.present
}

// Set writes value to the browser and makes it the value later handlers read.
func (v *JarValue[T]) Set(value T) error {
	if v == nil {
		return fmt.Errorf("%w: no cookie jar on request", ErrInvalidOptions)
	}
	if err := v.jar.SaveTo(v.carrier, value); err != nil {
		return err
	}
	v.data, v.present = value, true
	return nil
}

// Clear expires the cookie and makes the request look like it carried none.
func (v *JarValue[T]) Clear() {
	if v == nil {
		return
	}
	var zero T
	v.jar.ClearFrom(v.carrier)
	v.data, v.present = zero, false
}

// Middleware decodes the jar cookie once per request and publishes the handle
// returned by Value.
//
// A missing cookie continues with an absent value. A value this jar did not
// write, or one past its expiry, is cleared from the browser and continues as
// absent: stale client state is not an error the application has to handle.
func (j *Jar[T]) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			carrier := HTTPCarrier(w, r)
			handle := &JarValue[T]{jar: j, carrier: carrier}
			switch data, err := j.Load(r); {
			case err == nil:
				handle.data, handle.present = data, true
			case !errors.Is(err, ErrCookieMissing):
				j.ClearFrom(carrier)
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), jarKey{j}, handle)))
		})
	}
}

// Value returns the request-scoped handle installed by Middleware. It reports
// false when the jar's middleware did not run.
func (j *Jar[T]) Value(ctx context.Context) (*JarValue[T], bool) {
	if ctx == nil {
		return nil, false
	}
	handle, ok := ctx.Value(jarKey{j}).(*JarValue[T])
	return handle, ok
}

// Read returns the decoded value of the current request. It reports false when
// the request carried no usable value or when the jar's middleware did not
// run.
func (j *Jar[T]) Read(ctx context.Context) (T, bool) {
	handle, ok := j.Value(ctx)
	if !ok {
		var zero T
		return zero, false
	}
	return handle.Get()
}

// cookieBudget validates a configured size budget and applies the default.
func cookieBudget(maxBytes int) (int, error) {
	if maxBytes < 0 || maxBytes > hardMaxCookieBytes {
		return 0, fmt.Errorf("%w: cookie size budget", ErrInvalidOptions)
	}
	if maxBytes == 0 {
		return DefaultMaxCookieBytes, nil
	}
	return maxBytes, nil
}
