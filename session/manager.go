package session

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultCookieName is used when Options.Cookie.Name is empty.
	DefaultCookieName = "pw_session"
	maxTTL            = 365 * 24 * time.Hour
)

// CookieOptions is the browser cookie policy of a Manager. Secure defaults to
// true; only an explicit loopback development deployment should disable it.
type CookieOptions struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

// Options configures a Manager. TTL is required; every other field has a safe
// default.
type Options[T any] struct {
	// TTL is the absolute session lifetime.
	TTL time.Duration
	// IdleTimeout optionally expires a session that stops being used. It must
	// not exceed TTL.
	IdleTimeout time.Duration
	// RenewalInterval bounds how often an active request renews idle expiry.
	// It defaults to one tenth of IdleTimeout.
	RenewalInterval time.Duration
	Cookie          CookieOptions
	// Version invalidates records written before an incompatible change.
	Version int
	// Subject derives the request authentication subject from the payload. A
	// nil Subject leaves the subject empty.
	Subject func(T) string
	// Method labels the authentication method recorded on new sessions when
	// Create and Rotate receive an empty method.
	Method string
	Now    func() time.Time
	Random io.Reader
}

// Manager owns cookie tokens and coordinates a Store without exposing backend
// details to handlers. It is safe for concurrent use when its Store is.
type Manager[T any] struct {
	store    Store[T]
	options  Options[T]
	cookie   CookieOptions
	renewal  time.Duration
	now      func() time.Time
	random   io.Reader
	sameSite http.SameSite
}

// NewManager validates options and returns a Manager over store.
func NewManager[T any](store Store[T], options Options[T]) (*Manager[T], error) {
	if store == nil {
		return nil, fmt.Errorf("%w: nil store", ErrInvalidOptions)
	}
	if options.TTL <= 0 || options.TTL > maxTTL {
		return nil, fmt.Errorf("%w: ttl", ErrInvalidOptions)
	}
	if options.IdleTimeout < 0 || options.IdleTimeout > options.TTL {
		return nil, fmt.Errorf("%w: idle timeout", ErrInvalidOptions)
	}
	if options.RenewalInterval < 0 {
		return nil, fmt.Errorf("%w: renewal interval", ErrInvalidOptions)
	}
	if options.Version < 0 {
		return nil, fmt.Errorf("%w: version", ErrInvalidOptions)
	}
	cookie, sameSite, err := normalizeCookie(options.Cookie, DefaultCookieName)
	if err != nil {
		return nil, err
	}
	renewal := options.RenewalInterval
	if renewal == 0 && options.IdleTimeout > 0 {
		renewal = options.IdleTimeout / 10
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	return &Manager[T]{
		store:    store,
		options:  options,
		cookie:   cookie,
		renewal:  renewal,
		now:      now,
		random:   random,
		sameSite: sameSite,
	}, nil
}

// CookieName reports the configured browser cookie name.
func (m *Manager[T]) CookieName() string { return m.cookie.Name }

// Create issues a new token and stored record, and writes the session cookie.
// It does not revoke an existing session; use Rotate after authenticating a
// request that may already carry one.
func (m *Manager[T]) Create(w http.ResponseWriter, r *http.Request, data T) error {
	return m.create(w, r, data, m.options.Method)
}

// CreateWithMethod is Create with an explicit authentication method label.
func (m *Manager[T]) CreateWithMethod(w http.ResponseWriter, r *http.Request, data T, method string) error {
	return m.create(w, r, data, method)
}

// Rotate revokes the record referenced by the current request before issuing a
// replacement. Authentication-strength changes must rotate instead of mutating
// a session in place.
func (m *Manager[T]) Rotate(w http.ResponseWriter, r *http.Request, data T) error {
	return m.RotateWithMethod(w, r, data, m.options.Method)
}

// RotateWithMethod is Rotate with an explicit authentication method label.
func (m *Manager[T]) RotateWithMethod(w http.ResponseWriter, r *http.Request, data T, method string) error {
	if err := m.revokeRequestRecord(w, r); err != nil {
		return err
	}
	return m.create(w, r, data, method)
}

// Delete revokes the stored record and expires the browser cookie. Deleting a
// request without a session succeeds.
func (m *Manager[T]) Delete(w http.ResponseWriter, r *http.Request) error {
	err := m.revokeRequestRecord(w, r)
	m.clearCookie(w)
	return err
}

// bind hands the request and response to a Store that keeps its records in the
// browser rather than in a backend. A backend store implements no binder and
// receives the context unchanged.
func (m *Manager[T]) bind(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	binder, ok := m.store.(RequestBinder)
	if !ok {
		return ctx
	}
	return binder.BindRequest(ctx, w, r)
}

func (m *Manager[T]) create(w http.ResponseWriter, r *http.Request, data T, method string) error {
	if w == nil || r == nil {
		return fmt.Errorf("%w: nil request", ErrInvalidOptions)
	}
	token, err := newToken(m.random)
	if err != nil {
		return fmt.Errorf("%w: token", ErrUnavailable)
	}
	now := m.now()
	record := Record[T]{
		Data:            data,
		CreatedAt:       now,
		AuthenticatedAt: now,
		LastSeenAt:      now,
		ExpiresAt:       now.Add(m.options.TTL),
		Method:          method,
		Version:         m.options.Version,
	}
	if m.options.IdleTimeout > 0 {
		record.IdleExpiresAt = now.Add(m.options.IdleTimeout)
	}
	if err := m.store.Put(m.bind(r.Context(), w, r), keyHash(token), record); err != nil {
		return err
	}
	m.writeCookie(w, token, record.deadline())
	return nil
}

// revokeRequestRecord removes the store record referenced by the request
// cookie. A missing or malformed cookie is not an error.
func (m *Manager[T]) revokeRequestRecord(w http.ResponseWriter, r *http.Request) error {
	if r == nil {
		return nil
	}
	token, ok := m.requestToken(r)
	if !ok {
		return nil
	}
	return m.store.Delete(m.bind(r.Context(), w, r), keyHash(token))
}

func (m *Manager[T]) requestToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(m.cookie.Name)
	if err != nil || cookie == nil || !validToken(cookie.Value) {
		return "", false
	}
	return cookie.Value, true
}

// resolve loads and renews the record referenced by the request cookie.
func (m *Manager[T]) resolve(w http.ResponseWriter, r *http.Request) (Record[T], bool, error) {
	token, ok := m.requestToken(r)
	if !ok {
		return Record[T]{}, false, nil
	}
	hash := keyHash(token)
	ctx := m.bind(r.Context(), w, r)
	record, err := m.store.Get(ctx, hash)
	if err != nil {
		return Record[T]{}, false, err
	}
	if record.Version != m.options.Version {
		_ = m.store.Delete(ctx, hash)
		return Record[T]{}, false, ErrExpired
	}
	now := m.now()
	if !record.deadline().After(now) {
		_ = m.store.Delete(ctx, hash)
		return Record[T]{}, false, ErrExpired
	}
	if renewed, ok := m.renew(ctx, hash, record, now); ok {
		record = renewed
		m.writeCookie(w, token, record.deadline())
	}
	return record, true, nil
}

// renew touches the record only after RenewalInterval and never extends it
// beyond the absolute expiry.
func (m *Manager[T]) renew(ctx context.Context, hash string, record Record[T], now time.Time) (Record[T], bool) {
	if m.options.IdleTimeout <= 0 || m.renewal <= 0 {
		return record, false
	}
	if now.Sub(record.LastSeenAt) < m.renewal {
		return record, false
	}
	idleExpiresAt := now.Add(m.options.IdleTimeout)
	if idleExpiresAt.After(record.ExpiresAt) {
		idleExpiresAt = record.ExpiresAt
	}
	if err := m.store.Touch(ctx, hash, now, idleExpiresAt); err != nil {
		return record, false
	}
	record.LastSeenAt = now
	record.IdleExpiresAt = idleExpiresAt
	return record, true
}

func (m *Manager[T]) writeCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookie.Name,
		Value:    token,
		Path:     m.cookie.Path,
		Domain:   m.cookie.Domain,
		Expires:  expiresAt,
		MaxAge:   int(expiresAt.Sub(m.now()).Seconds()),
		Secure:   m.cookie.Secure,
		HttpOnly: m.cookie.HTTPOnly,
		SameSite: m.sameSite,
	})
}

func (m *Manager[T]) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookie.Name,
		Value:    "",
		Path:     m.cookie.Path,
		Domain:   m.cookie.Domain,
		MaxAge:   -1,
		Secure:   m.cookie.Secure,
		HttpOnly: m.cookie.HTTPOnly,
		SameSite: m.sameSite,
	})
}

// normalizeCookie applies the cookie policy defaults shared by the session
// manager and Jar, and rejects a policy the browser would not honor safely.
// An empty defaultName makes the name required.
func normalizeCookie(cookie CookieOptions, defaultName string) (CookieOptions, http.SameSite, error) {
	if cookie.Name == "" {
		cookie.Name = defaultName
	}
	if !validCookieName(cookie.Name) {
		return CookieOptions{}, 0, fmt.Errorf("%w: cookie name", ErrInvalidOptions)
	}
	if cookie.Path == "" {
		cookie.Path = "/"
	}
	if !strings.HasPrefix(cookie.Path, "/") {
		return CookieOptions{}, 0, fmt.Errorf("%w: cookie path", ErrInvalidOptions)
	}
	sameSite := cookie.SameSite
	if sameSite == 0 {
		sameSite = http.SameSiteLaxMode
	}
	if sameSite == http.SameSiteNoneMode && !cookie.Secure {
		return CookieOptions{}, 0, fmt.Errorf("%w: insecure same-site none cookie", ErrInvalidOptions)
	}
	return cookie, sameSite, nil
}

func validCookieName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range len(value) {
		c := value[index]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
